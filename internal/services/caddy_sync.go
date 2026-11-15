package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/models"
)

// agentCertDir is the directory where the agent stores custom certificate files.
const agentCertDir = "/var/lib/posternproxy-agent/certs"

// CaddySyncService rebuilds and pushes the full Caddy configuration from the DB.
// It is called once on startup and can be triggered manually after bulk changes.
type CaddySyncService struct {
	mu      sync.Mutex
	locks   map[int64]*sync.Mutex
	pending map[int64]configAttempt
	cancel  context.CancelFunc
	done    chan struct{}
	db      *sql.DB
	caddy   *caddy.Client
	hub     *agent.Hub
	version string
}

func NewCaddySyncService(db *sql.DB, caddyClient *caddy.Client, hub *agent.Hub, version string) *CaddySyncService {
	s := &CaddySyncService{db: db, caddy: caddyClient, hub: hub, version: version, locks: make(map[int64]*sync.Mutex), pending: make(map[int64]configAttempt)}
	if hub != nil {
		hub.SetCommandResultHandler(s.commandResult)
	}
	return s
}

// FullSync reads all enabled entities for the local server (server_id=1), builds
// the complete Caddy JSON config, and pushes it via POST /load.
func (s *CaddySyncService) FullSync(ctx context.Context) error {
	return s.SyncServer(ctx, 1)
}

// SyncServer pushes the full Caddy config for the given server.
// For the local server (id=1) it calls the local Caddy admin API directly.
// For remote servers it sends a KindConfigFullSync message via the agent hub;
// if the agent is offline the message is queued and delivered on reconnect.
func (s *CaddySyncService) SyncServer(ctx context.Context, serverID int64) error {
	if serverID == 0 {
		serverID = 1
	}
	lock := s.serverLock(serverID)
	lock.Lock()
	defer lock.Unlock()
	state, err := s.State(ctx, serverID)
	if err != nil {
		return err
	}
	if s.attemptActive(serverID) {
		return nil
	}
	revision := state.DesiredRevision
	if serverID != 1 && (s.hub == nil || !s.hub.IsOnline(serverID)) {
		return s.mark(ctx, serverID, revision, "queued", "")
	}
	if err := s.mark(ctx, serverID, revision, "applying", ""); err != nil {
		return err
	}
	if err := s.applyServer(ctx, serverID, revision); err != nil {
		if markErr := s.mark(ctx, serverID, revision, "failed", "Configuration application failed; retry or check server connectivity."); markErr != nil {
			return fmt.Errorf("record application failure: %w", markErr)
		}
		return &ConfigApplyError{ServerID: serverID, cause: err}
	}
	return nil
}

func (s *CaddySyncService) applyServer(ctx context.Context, serverID, revision int64) error {

	input, err := s.buildInput(ctx, serverID)
	if err != nil {
		return fmt.Errorf("build caddy input for server %d: %w", serverID, err)
	}

	if serverID == 1 {
		cfg, err := caddy.BuildFullConfig(input)
		if err != nil {
			return fmt.Errorf("build caddy config: %w", err)
		}
		if err := s.caddy.LoadContext(ctx, cfg); err != nil {
			return fmt.Errorf("push caddy config: %w", err)
		}
		return s.applied(ctx, serverID, revision)
	}

	// Remote server: use agent-side cert paths and bundle cert bytes.
	input.AgentCertDir = agentCertDir

	cfg, err := caddy.BuildFullConfig(input)
	if err != nil {
		return fmt.Errorf("build caddy config: %w", err)
	}

	configJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal caddy config: %w", err)
	}

	// Read custom cert files from controller disk and bundle them so the agent
	// can write them before applying the config.
	var certFiles []agent.CertFile
	for _, cert := range input.Certificates {
		if cert.Provider != "custom" || cert.CertPath == nil || cert.KeyPath == nil {
			continue
		}
		certPEM, err := os.ReadFile(*cert.CertPath)
		if err != nil {
			return fmt.Errorf("read required certificate: %w", err)
		}
		keyPEM, err := os.ReadFile(*cert.KeyPath)
		if err != nil {
			return fmt.Errorf("read required key: %w", err)
		}
		certFiles = append(certFiles, agent.CertFile{
			CertID:  cert.ID,
			CertPEM: certPEM,
			KeyPEM:  keyPEM,
		})
	}

	syncPayload := agent.ConfigFullSyncPayload{
		Config: json.RawMessage(configJSON),
		Certs:  certFiles,
	}
	msg, err := agent.NewMessage(agent.KindConfigFullSync, syncPayload)
	if err != nil {
		return fmt.Errorf("build agent message: %w", err)
	}
	msg.ID, err = newCommandID()
	if err != nil {
		return err
	}
	msg.Revision = revision
	s.mu.Lock()
	s.pending[serverID] = configAttempt{ID: msg.ID, Revision: revision, Deadline: time.Now().Add(30 * time.Second)}
	s.mu.Unlock()
	if s.hub == nil || !s.hub.Send(serverID, msg) {
		s.clearAttempt(serverID, msg.ID)
		return s.mark(ctx, serverID, revision, "queued", "")
	}
	return nil
}

// Reachable reports whether the local Caddy admin API is responding.
func (s *CaddySyncService) Reachable() bool {
	return s.caddy.Ping() == nil
}

func (s *CaddySyncService) buildInput(ctx context.Context, serverID int64) (*caddy.BuildInput, error) {
	proxyHosts, err := s.loadProxyHosts(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("load proxy hosts: %w", err)
	}

	redirectionHosts, err := s.loadRedirectionHosts(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("load redirection hosts: %w", err)
	}

	streamHosts, err := s.loadStreamHosts(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("load stream hosts: %w", err)
	}

	deadHosts, err := s.loadDeadHosts(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("load dead hosts: %w", err)
	}

	certificates, err := s.loadCertificates(ctx)
	if err != nil {
		return nil, fmt.Errorf("load certificates: %w", err)
	}

	accessLists, err := s.loadAccessLists(ctx)
	if err != nil {
		return nil, fmt.Errorf("load access lists: %w", err)
	}

	http2Enabled := 1
	if err := s.db.QueryRowContext(ctx, `SELECT http2_enabled FROM servers WHERE id = ?`, serverID).Scan(&http2Enabled); err != nil {
		return nil, fmt.Errorf("load server settings: %w", err)
	}

	return &caddy.BuildInput{
		ProxyHosts:        proxyHosts,
		RedirectionHosts:  redirectionHosts,
		StreamHosts:       streamHosts,
		DeadHosts:         deadHosts,
		Certificates:      certificates,
		AccessLists:       accessLists,
		HTTP2Disabled:     http2Enabled == 0,
		ControllerVersion: s.version,
	}, nil
}

func (s *CaddySyncService) loadProxyHosts(ctx context.Context, serverID int64) ([]models.ProxyHost, error) {
	// Uses the canonical column list from proxy_host.go (proxyHostColumns).
	rows, err := s.db.QueryContext(ctx, `SELECT`+proxyHostColumns+`FROM proxy_hosts WHERE enabled = 1 AND server_id = ?`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProxyHosts(rows)
}

func (s *CaddySyncService) loadRedirectionHosts(ctx context.Context, serverID int64) ([]models.RedirectionHost, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, server_id, domain_names, forward_scheme, forward_domain_name, forward_path,
		       preserve_path, status_code, certificate_id, ssl_forced, access_list_id, enabled, caddy_id,
		       created_at, updated_at
		FROM redirection_hosts WHERE enabled = 1 AND server_id = ?`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []models.RedirectionHost
	for rows.Next() {
		var h models.RedirectionHost
		var preservePath, sslForced int
		var domainNames string
		if err := rows.Scan(
			&h.ID, &h.ServerID, &domainNames, &h.ForwardScheme, &h.ForwardDomainName, &h.ForwardPath,
			&preservePath, &h.StatusCode, &h.CertificateID, &sslForced, &h.AccessListID, &h.Enabled, &h.CaddyID,
			&h.CreatedAt, &h.UpdatedAt,
		); err != nil {
			return nil, err
		}
		h.DomainNames = json.RawMessage(domainNames)
		h.PreservePath = preservePath == 1
		h.SSLForced = sslForced == 1
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (s *CaddySyncService) loadStreamHosts(ctx context.Context, serverID int64) ([]models.StreamHost, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, server_id, incoming_port, forward_host, forward_port, protocol,
		       tcp_sni_match, enabled, caddy_id, created_at, updated_at
		FROM stream_hosts WHERE enabled = 1 AND server_id = ?`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []models.StreamHost
	for rows.Next() {
		var h models.StreamHost
		var enabled int
		if err := rows.Scan(
			&h.ID, &h.ServerID, &h.IncomingPort, &h.ForwardHost, &h.ForwardPort, &h.Protocol,
			&h.TCPSNIMatch, &enabled, &h.CaddyID, &h.CreatedAt, &h.UpdatedAt,
		); err != nil {
			return nil, err
		}
		h.Enabled = enabled == 1
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (s *CaddySyncService) loadDeadHosts(ctx context.Context, serverID int64) ([]models.DeadHost, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, server_id, domain_names, status_code, custom_body, enabled, caddy_id, created_at, updated_at
		FROM dead_hosts WHERE enabled = 1 AND server_id = ?`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []models.DeadHost
	for rows.Next() {
		var h models.DeadHost
		var domainNames string
		var enabled int
		if err := rows.Scan(
			&h.ID, &h.ServerID, &domainNames, &h.StatusCode, &h.CustomBody, &enabled, &h.CaddyID,
			&h.CreatedAt, &h.UpdatedAt,
		); err != nil {
			return nil, err
		}
		h.DomainNames = json.RawMessage(domainNames)
		h.Enabled = enabled == 1
		hosts = append(hosts, h)
	}
	return hosts, rows.Err()
}

func (s *CaddySyncService) loadCertificates(ctx context.Context) ([]*models.Certificate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, provider, domain_names, cert_path, key_path,
		       expires_at, status, created_at, updated_at
		FROM certificates`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []*models.Certificate
	for rows.Next() {
		var c models.Certificate
		var domainNames string
		if err := rows.Scan(
			&c.ID, &c.Name, &c.Provider, &domainNames,
			&c.CertPath, &c.KeyPath,
			&c.ExpiresAt, &c.Status,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		c.DomainNames = json.RawMessage(domainNames)
		certs = append(certs, &c)
	}
	return certs, rows.Err()
}

func (s *CaddySyncService) loadAccessLists(ctx context.Context) ([]models.AccessList, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, created_at, updated_at FROM access_lists`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []models.AccessList
	for rows.Next() {
		var al models.AccessList
		if err := rows.Scan(&al.ID, &al.Name, &al.CreatedAt, &al.UpdatedAt); err != nil {
			return nil, err
		}
		lists = append(lists, al)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range lists {
		al := &lists[i]

		clientRows, err := s.db.QueryContext(ctx, `
			SELECT id, access_list_id, address, directive, sort_order
			FROM access_list_clients WHERE access_list_id = ? ORDER BY sort_order`, al.ID)
		if err != nil {
			return nil, err
		}
		for clientRows.Next() {
			var c models.AccessListClient
			if err := clientRows.Scan(&c.ID, &c.AccessListID, &c.Address, &c.Directive, &c.SortOrder); err != nil {
				clientRows.Close()
				return nil, err
			}
			al.Clients = append(al.Clients, c)
		}
		if err := clientRows.Err(); err != nil {
			clientRows.Close()
			return nil, err
		}
		if err := clientRows.Close(); err != nil {
			return nil, err
		}
	}
	return lists, nil
}
