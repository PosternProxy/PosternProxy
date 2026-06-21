package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/provisioner"
	"github.com/posternproxy/posternproxy/internal/security"
	"github.com/rs/zerolog/log"
)

// ServerService manages the servers table and orchestrates provisioning.
type ServerService struct {
	db            *sql.DB
	audit         *AuditService
	hub           *agent.Hub
	tokens        *APITokenService
	caddy         *caddy.Client // local Caddy admin API, for the local server's liveness
	encKey        [32]byte
	controllerURL string // e.g. "wss://my-controller:81" — sent to agent config
	agentBinPath  string // optional; path to local caddyproxy-agent binary

	// runsMu protects runs.
	// runs holds the active or most-recent ProvisionRun per server. An entry
	// being present + not Done() means a provisioning attempt is in flight.
	runsMu sync.Mutex
	runs   map[int64]*ProvisionRun
}

func NewServerService(db *sql.DB, audit *AuditService, hub *agent.Hub, tokens *APITokenService, caddyClient *caddy.Client, encKey [32]byte, controllerURL, agentBinPath string) *ServerService {
	return &ServerService{
		db:            db,
		audit:         audit,
		hub:           hub,
		tokens:        tokens,
		caddy:         caddyClient,
		encKey:        encKey,
		controllerURL: controllerURL,
		agentBinPath:  agentBinPath,
		runs:          make(map[int64]*ProvisionRun),
	}
}

// reconcileStatus overwrites the derived liveness fields on a server row before
// it is returned to callers. Remote servers track liveness via the agent's
// WebSocket connection. The local server has no agent — its real liveness is
// whether the local Caddy admin API answers, so we probe it here instead of
// trusting the seeded "online" value (which was previously never reconciled and
// made the dashboard show green even when Caddy was down or unconfigured).
func (s *ServerService) reconcileStatus(srv *models.Server) {
	if srv.IsLocal {
		if s.caddy != nil && s.caddy.Ping() == nil {
			srv.Online = true
			srv.Status = "online"
		} else {
			srv.Online = false
			srv.Status = "offline"
		}
		return
	}
	srv.Online = s.hub.IsOnline(srv.ID)
}

// GetProvisionRun returns the most-recent ProvisionRun for the given server,
// or nil if none has occurred since this controller process started.
//
// Callers should check Done() to distinguish a live run (subscribe for streaming)
// from a finished run (its Result is final; the DB has the canonical status).
func (s *ServerService) GetProvisionRun(serverID int64) *ProvisionRun {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	return s.runs[serverID]
}

// startRun atomically claims the provisioning slot for serverID. It returns
// the new run, or ErrProvisioningInProgress if a run is currently active.
// A finished run is replaced — the slot is only "busy" while Done() is false.
func (s *ServerService) startRun(serverID int64) (*ProvisionRun, error) {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	if existing, ok := s.runs[serverID]; ok && !existing.Done() {
		return nil, ErrProvisioningInProgress
	}
	run := &ProvisionRun{}
	s.runs[serverID] = run
	return run, nil
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

func (s *ServerService) List(ctx context.Context) ([]*models.Server, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, hostname, ssh_port, ssh_user, ssh_auth_method,
		       agent_token_hash, status, provision_log, is_local, last_seen_at,
		       created_at, updated_at, caddy_version, uptime_seconds, active_routes, last_status_at,
		       deployment_type, ssh_host_key, http2_enabled
		FROM servers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*models.Server
	for rows.Next() {
		srv, err := scanServerRow(rows)
		if err != nil {
			return nil, err
		}
		s.reconcileStatus(srv)
		servers = append(servers, srv)
	}
	return servers, rows.Err()
}

func (s *ServerService) GetByID(ctx context.Context, id int64) (*models.Server, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, hostname, ssh_port, ssh_user, ssh_auth_method,
		       agent_token_hash, status, provision_log, is_local, last_seen_at,
		       created_at, updated_at, caddy_version, uptime_seconds, active_routes, last_status_at,
		       deployment_type, ssh_host_key, http2_enabled
		FROM servers WHERE id = ?`, id)
	srv, err := scanServerRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.reconcileStatus(srv)
	return srv, nil
}

// Create registers a new remote server and starts provisioning in the
// background. Callers can watch progress via GetProvisionRun(serverID) and
// subscribing to the returned ProvisionRun.
func (s *ServerService) Create(ctx context.Context, callerID *int64, input models.CreateServerInput, ip string) (*models.Server, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if input.Hostname == "" {
		return nil, fmt.Errorf("hostname is required")
	}
	if input.SSHPort == 0 {
		input.SSHPort = 22
	}
	if input.SSHUser == "" {
		input.SSHUser = "root"
	}

	// Encrypt SSH credentials before storing.
	encCreds, err := security.Encrypt(s.encKey, input.SSHCredentials)
	if err != nil {
		return nil, fmt.Errorf("encrypt credentials: %w", err)
	}

	deploymentType := input.DeploymentType
	if deploymentType == "" {
		deploymentType = "systemd"
	}

	// Insert the server row first (we need the ID for the agent token).
	// Use a placeholder token hash — it will be replaced right after.
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO servers (name, hostname, ssh_port, ssh_user, ssh_auth_method,
		                     ssh_credentials_encrypted, agent_token_hash, status, deployment_type)
		VALUES (?, ?, ?, ?, ?, ?, '', 'provisioning', ?)`,
		input.Name, input.Hostname, input.SSHPort, input.SSHUser,
		input.SSHAuthMethod, encCreds, deploymentType,
	)
	if err != nil {
		return nil, fmt.Errorf("insert server: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	// Create the agent token via the unified token service.
	// Also keep a bcrypt hash in servers.agent_token_hash for WebSocket auth.
	token, err := s.tokens.CreateForServer(ctx, id, callerID)
	if err != nil {
		// Best-effort cleanup: the original error is being returned regardless,
		// so a delete failure here would mask the real problem.
		if _, delErr := s.db.ExecContext(ctx, "DELETE FROM servers WHERE id=?", id); delErr != nil {
			log.Warn().Err(delErr).Int64("server_id", id).Msg("failed to clean up orphan server row after token creation failed")
		}
		return nil, fmt.Errorf("create agent token: %w", err)
	}
	tokenHash, _ := security.BcryptHash(token)
	if _, err := s.db.ExecContext(ctx, `UPDATE servers SET agent_token_hash=? WHERE id=?`, tokenHash, id); err != nil {
		log.Warn().Err(err).Int64("server_id", id).Msg("failed to write agent_token_hash; agent will not be able to authenticate")
	}

	srv, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.audit.Log(ctx, callerID, "create", "server", &id, map[string]any{"name": input.Name, "hostname": input.Hostname}, ip)

	// startRun for a brand-new server cannot conflict — no prior run exists.
	run, err := s.startRun(id)
	if err != nil {
		return nil, err
	}

	pInput := provisioner.ProvisionInput{
		Host:           input.Hostname,
		SSHPort:        input.SSHPort,
		SSHUser:        input.SSHUser,
		SSHAuthMethod:  input.SSHAuthMethod,
		SSHCredentials: input.SSHCredentials,
		ControllerURL:  s.controllerURL,
		AgentToken:     token,
		ServerID:       id,
		AgentBinPath:   s.agentBinPath,
		DeploymentType: deploymentType,
		// SSHHostKey is empty on first provision — TOFU will accept and persist the key.
		OnHostKeyAccepted: func(fp string) {
			// Background work — request context would cancel host-key persistence if the user navigated away.
			bgCtx := context.Background()
			if _, err := s.db.ExecContext(bgCtx, `UPDATE servers SET ssh_host_key=? WHERE id=?`, fp, id); err != nil {
				log.Warn().Err(err).Int64("server_id", id).Msg("failed to persist accepted SSH host key; future connections will re-TOFU")
			}
		},
	}
	s.runProvisioner(id, pInput, run)

	return srv, nil
}

// UpdateCredentials updates the editable fields of a server record (name,
// hostname, SSH connection parameters, and re-encrypted credentials).
// is_local=1 servers are not updatable.
func (s *ServerService) UpdateCredentials(ctx context.Context, callerID *int64, id int64, input models.CreateServerInput, ip string) error {
	if input.Hostname == "" {
		return fmt.Errorf("hostname is required")
	}
	encCreds, err := security.Encrypt(s.encKey, input.SSHCredentials)
	if err != nil {
		return fmt.Errorf("encrypt credentials: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE servers SET name=?, hostname=?, ssh_port=?, ssh_user=?, ssh_auth_method=?,
		                   ssh_credentials_encrypted=?, updated_at=datetime('now')
		WHERE id=? AND is_local=0`,
		input.Name, input.Hostname, input.SSHPort, input.SSHUser, input.SSHAuthMethod, encCreds, id,
	); err != nil {
		return fmt.Errorf("update server: %w", err)
	}
	s.audit.Log(ctx, callerID, "update", "server", &id, nil, ip)
	return nil
}

// SetHTTP2 toggles the server-wide HTTP/2 setting. Allowed for the local server
// too (HTTP/2 is a property of each server's HTTPS listener). The caller is
// responsible for re-syncing Caddy so the change takes effect.
func (s *ServerService) SetHTTP2(ctx context.Context, callerID *int64, id int64, enabled bool, ip string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE servers SET http2_enabled=?, updated_at=datetime('now') WHERE id=?`,
		boolToInt(enabled), id)
	if err != nil {
		return fmt.Errorf("update http2_enabled: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrServerNotFound
	}
	s.audit.Log(ctx, callerID, "update", "server", &id, map[string]any{"http2_enabled": enabled}, ip)
	return nil
}

// Delete removes a server record (not allowed for the local server).
func (s *ServerService) Delete(ctx context.Context, callerID *int64, id int64, ip string) error {
	if id == 1 {
		return fmt.Errorf("cannot delete the local server")
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM servers WHERE id=? AND is_local=0", id); err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	s.audit.Log(ctx, callerID, "delete", "server", &id, nil, ip)
	return nil
}

// ErrAgentConnected is returned by ResetJoin when force=false and an agent
// is currently online for the target server. Operators should disconnect or
// uninstall the agent first, or pass force=true.
var ErrAgentConnected = fmt.Errorf("agent is currently connected")

// ResetJoin clears agent_token_hash on the server row so a new join token can
// be issued. If force is false, returns ErrAgentConnected when an agent is
// currently online for this server.
//
// Side effect: any future connection attempt by the previously-enrolled agent
// will be rejected. Operators should uninstall the old agent on the target.
func (s *ServerService) ResetJoin(ctx context.Context, callerID *int64, serverID int64, force bool, remoteAddr string) error {
	if serverID == 1 {
		return fmt.Errorf("cannot reset join on the local server")
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM servers WHERE id = ?`, serverID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return ErrServerNotFound
		}
		return fmt.Errorf("lookup server: %w", err)
	}
	if !force && s.hub != nil && s.hub.IsOnline(serverID) {
		return ErrAgentConnected
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE servers
		   SET agent_token_hash = '', status = 'pending', updated_at = datetime('now')
		 WHERE id = ?`, serverID); err != nil {
		return fmt.Errorf("clear agent token: %w", err)
	}
	s.audit.Log(ctx, callerID, "server.reset_join", "server", &serverID, map[string]any{"force": force}, remoteAddr)
	return nil
}

// Reprovision re-runs the provisioning script on an existing server.
// Returns ErrProvisioningInProgress if a run is already active for this server
// (the caller should map that to HTTP 409 Conflict).
func (s *ServerService) Reprovision(ctx context.Context, callerID *int64, id int64, ip string) error {
	srv, err := s.GetByID(ctx, id)
	if err != nil || srv == nil {
		return fmt.Errorf("server not found")
	}
	if srv.IsLocal {
		return fmt.Errorf("cannot reprovision the local server")
	}

	// Claim the run slot BEFORE rotating the agent token. If we rotated first
	// and then discovered another run was in progress, we'd have invalidated
	// the in-flight run's token — exactly the bug H-5 calls out.
	run, err := s.startRun(id)
	if err != nil {
		return err
	}

	// Decrypt stored credentials.
	var encCreds string
	if err := s.db.QueryRowContext(ctx, `SELECT ssh_credentials_encrypted FROM servers WHERE id=?`, id).Scan(&encCreds); err != nil {
		// Release the slot on early failure so the user can retry.
		run.finish(provisioner.Result{Success: false, Error: err.Error()})
		return fmt.Errorf("fetch credentials: %w", err)
	}
	creds, err := security.Decrypt(s.encKey, encCreds)
	if err != nil {
		run.finish(provisioner.Result{Success: false, Error: err.Error()})
		return fmt.Errorf("decrypt credentials: %w", err)
	}

	// Rotate the agent token via the unified token service.
	token, err := s.tokens.CreateForServer(ctx, id, callerID) // also revokes old agent token
	if err != nil {
		run.finish(provisioner.Result{Success: false, Error: err.Error()})
		return fmt.Errorf("rotate agent token: %w", err)
	}
	tokenHash, _ := security.BcryptHash(token)
	if _, err := s.db.ExecContext(ctx, `UPDATE servers SET status='provisioning', agent_token_hash=?, provision_log='', updated_at=datetime('now') WHERE id=?`,
		tokenHash, id); err != nil {
		log.Warn().Err(err).Int64("server_id", id).Msg("failed to mark server as provisioning; agent may fail to authenticate on reconnect")
	}

	pInput := provisioner.ProvisionInput{
		Host:           srv.Hostname,
		SSHPort:        srv.SSHPort,
		SSHUser:        srv.SSHUser,
		SSHAuthMethod:  srv.SSHAuthMethod,
		SSHCredentials: creds,
		ControllerURL:  s.controllerURL,
		AgentToken:     token,
		ServerID:       id,
		AgentBinPath:   s.agentBinPath,
		DeploymentType: srv.DeploymentType,
		// SSHHostKey may be empty for servers provisioned before TOFU was introduced;
		// in that case we fall back to TOFU and store the key going forward.
		SSHHostKey: srv.SSHHostKey,
		OnHostKeyAccepted: func(fp string) {
			// Background work — request context would cancel host-key persistence if the user navigated away.
			bgCtx := context.Background()
			if _, err := s.db.ExecContext(bgCtx, `UPDATE servers SET ssh_host_key=? WHERE id=?`, fp, id); err != nil {
				log.Warn().Err(err).Int64("server_id", id).Msg("failed to persist accepted SSH host key; future connections will re-TOFU")
			}
		},
	}
	s.runProvisioner(id, pInput, run)

	s.audit.Log(ctx, callerID, "update", "server", &id, map[string]any{"action": "reprovision"}, ip)
	return nil
}

// runProvisioner drains the provisioner's log channel into the ProvisionRun
// and persists the final result. It is the single producer that broadcasts
// log lines to all active subscribers, and the single point where the DB row
// transitions from "provisioning" to "online" or "error".
func (s *ServerService) runProvisioner(id int64, pInput provisioner.ProvisionInput, run *ProvisionRun) {
	logCh := make(chan string, 256)
	resultCh := make(chan provisioner.Result, 1)

	// 1. The provisioner goroutine produces log lines + a single result.
	go provisioner.Provision(pInput, logCh, resultCh)

	// 2. Drainer goroutine fans logCh out to all run subscribers.
	logsDone := make(chan struct{})
	go func() {
		for line := range logCh {
			run.appendLine(line)
		}
		close(logsDone)
	}()

	// 3. Result handler waits for the provisioner to finish, closes logCh so
	//    the drainer exits, persists final status, and finishes the run.
	go func() {
		// Background work — request context would cancel provisioning if the user navigated away.
		ctx := context.Background()
		res := <-resultCh
		close(logCh)
		<-logsDone
		status := "online"
		if !res.Success {
			status = "error"
		}
		// res.Log already contains the full log assembled by the provisioner;
		// prefer it over our in-memory copy so the stored log matches whatever
		// the provisioner deemed canonical.
		stored := res.Log
		if stored == "" {
			// Fall back to our drained buffer.
			history, _ := run.Subscribe()
			stored = strings.Join(history, "\n")
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE servers SET status=?, provision_log=?, updated_at=datetime('now') WHERE id=?`,
			status, stored, id); err != nil {
			log.Warn().Err(err).Int64("server_id", id).Str("final_status", status).Msg("failed to persist final provisioning status; server row may be stale")
		}
		run.finish(res)
	}()
}

// UpdateFromStatus persists a status heartbeat from an agent.
// Errors are logged at warn level rather than returned — heartbeats arrive
// every 30 s, so a transient DB lock is recoverable on the next tick; silent
// failure would leave the server appearing offline indefinitely (audit M-9).
func (s *ServerService) UpdateFromStatus(ctx context.Context, serverID int64, p agent.StatusPayload) {
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx, `
		UPDATE servers
		SET status='online', last_seen_at=?, last_status_at=?,
		    caddy_version=?, uptime_seconds=?, active_routes=?,
		    updated_at=datetime('now')
		WHERE id=?`,
		now, now, p.CaddyVersion, p.Uptime, p.ActiveRoutes, serverID); err != nil {
		log.Warn().Err(err).Int64("server_id", serverID).Msg("failed to persist agent heartbeat")
	}
}

// dummyAgentTokenHash is compared when the requested server_id does not exist,
// ensuring ValidateAgentToken runs in constant time regardless of input.
var dummyAgentTokenHash = func() string {
	h, err := security.BcryptHash("posternproxy-agent-dummy-token")
	if err != nil {
		// crypto failure at startup is fatal; fall back to a constant so the
		// service can still start and validation still returns false.
		return "$2a$12$invalid"
	}
	return h
}()

// ValidateAgentToken checks whether plaintext token matches the stored hash
// for the given server. Returns true on success. Runs in roughly constant
// time regardless of whether serverID exists, to prevent ID enumeration.
func (s *ServerService) ValidateAgentToken(ctx context.Context, serverID int64, token string) bool {
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT agent_token_hash FROM servers WHERE id=?`, serverID).Scan(&hash)
	if err != nil || hash == "" {
		// Run bcrypt against a dummy hash so timing matches the happy path.
		_ = security.BcryptCompare(dummyAgentTokenHash, token)
		return false
	}
	return security.BcryptCompare(hash, token) == nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

type serverScanner interface {
	Scan(dest ...any) error
}

func scanServerRow(row serverScanner) (*models.Server, error) {
	var srv models.Server
	var http2 int
	err := row.Scan(
		&srv.ID, &srv.Name, &srv.Hostname, &srv.SSHPort, &srv.SSHUser,
		&srv.SSHAuthMethod, &srv.AgentTokenHash, &srv.Status, &srv.ProvisionLog,
		&srv.IsLocal, &srv.LastSeenAt, &srv.CreatedAt, &srv.UpdatedAt,
		&srv.CaddyVersion, &srv.UptimeSeconds, &srv.ActiveRoutes, &srv.LastStatusAt,
		&srv.DeploymentType, &srv.SSHHostKey, &http2,
	)
	srv.HTTP2Enabled = http2 == 1
	return &srv, err
}
