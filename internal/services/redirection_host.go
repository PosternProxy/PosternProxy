package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/models"
)

// RedirectionHostService manages HTTP redirection virtual hosts in SQLite and Caddy.
type RedirectionHostService struct {
	db    *sql.DB
	caddy *caddy.Client
	sync  *CaddySyncService
	audit *AuditService
}

// NewRedirectionHostService creates a RedirectionHostService backed by db.
func NewRedirectionHostService(db *sql.DB, caddyClient *caddy.Client, audit *AuditService, sync *CaddySyncService) *RedirectionHostService {
	return &RedirectionHostService{db: db, caddy: caddyClient, sync: sync, audit: audit}
}

// ── Input type ────────────────────────────────────────────────────────────────

// CreateRedirectionHostInput carries all fields accepted when creating or
// updating a RedirectionHost. Only the core redirect fields are accepted;
// SSL, HSTS, access lists, and advanced config are not supported because
// they do not apply meaningfully to a pure HTTP redirect.
type CreateRedirectionHostInput struct {
	ServerID          int64           `json:"server_id"`
	DomainNames       json.RawMessage `json:"domain_names"`
	ForwardScheme     string          `json:"forward_scheme"`
	ForwardDomainName string          `json:"forward_domain_name"`
	ForwardPath       string          `json:"forward_path"`
	PreservePath      bool            `json:"preserve_path"`
	StatusCode        int             `json:"status_code"`
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

// List returns all redirection hosts ordered by ID.
func (s *RedirectionHostService) List(ctx context.Context) ([]models.RedirectionHost, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, server_id, domain_names, forward_scheme, forward_domain_name, forward_path,
		       preserve_path, status_code, certificate_id, ssl_forced, hsts_enabled,
		       access_list_id, advanced_config, enabled, caddy_id,
		       created_at, updated_at
		FROM redirection_hosts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRedirectionHosts(rows)
}

// ListByServer returns redirection hosts for a specific server, ordered by ID.
func (s *RedirectionHostService) ListByServer(ctx context.Context, serverID int64) ([]models.RedirectionHost, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, server_id, domain_names, forward_scheme, forward_domain_name, forward_path,
		       preserve_path, status_code, certificate_id, ssl_forced, hsts_enabled,
		       access_list_id, advanced_config, enabled, caddy_id,
		       created_at, updated_at
		FROM redirection_hosts WHERE server_id = ? ORDER BY id`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRedirectionHosts(rows)
}

func (s *RedirectionHostService) GetByID(ctx context.Context, id int64) (*models.RedirectionHost, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, server_id, domain_names, forward_scheme, forward_domain_name, forward_path,
		       preserve_path, status_code, certificate_id, ssl_forced, hsts_enabled,
		       access_list_id, advanced_config, enabled, caddy_id,
		       created_at, updated_at
		FROM redirection_hosts WHERE id = ?`, id)
	h, err := scanRedirectionHostRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

func (s *RedirectionHostService) Create(ctx context.Context, callerID *int64, input CreateRedirectionHostInput, ip string) (*models.RedirectionHost, error) {
	if err := validateDomainNames(input.DomainNames); err != nil {
		return nil, err
	}
	if input.ForwardDomainName == "" {
		return nil, fmt.Errorf("forward domain name is required")
	}
	if input.ServerID == 0 {
		input.ServerID = 1
	}
	if input.StatusCode == 0 {
		input.StatusCode = 301
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO redirection_hosts
		  (server_id, domain_names, forward_scheme, forward_domain_name, forward_path,
		   preserve_path, status_code)
		VALUES (?,?,?,?,?,?,?)`,
		input.ServerID, string(input.DomainNames), input.ForwardScheme,
		input.ForwardDomainName, input.ForwardPath, boolToInt(input.PreservePath),
		input.StatusCode,
	)
	if err != nil {
		return nil, fmt.Errorf("insert redirection host: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	caddyID := fmt.Sprintf("redirection_host_%d", id)
	if _, err := tx.ExecContext(ctx, "UPDATE redirection_hosts SET caddy_id = ? WHERE id = ?", caddyID, id); err != nil {
		return nil, fmt.Errorf("set caddy_id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	host, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.audit.HostChange(ctx, callerID, "create", "redirection_host", id, nil, host, ip)
	return host, syncServers(ctx, s.sync, host.ServerID)
}

func (s *RedirectionHostService) Update(ctx context.Context, callerID *int64, id int64, input CreateRedirectionHostInput, ip string) (*models.RedirectionHost, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("redirection host not found")
	}
	if err := validateDomainNames(input.DomainNames); err != nil {
		return nil, err
	}
	if input.ForwardDomainName == "" {
		return nil, fmt.Errorf("forward domain name is required")
	}

	if input.ServerID == 0 {
		input.ServerID = existing.ServerID
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE redirection_hosts SET
		  server_id=?,
		  domain_names=?, forward_scheme=?, forward_domain_name=?, forward_path=?,
		  preserve_path=?, status_code=?, updated_at=datetime('now')
		WHERE id=?`,
		input.ServerID, string(input.DomainNames), input.ForwardScheme, input.ForwardDomainName, input.ForwardPath,
		boolToInt(input.PreservePath), input.StatusCode, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update redirection host: %w", err)
	}

	host, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.audit.HostChange(ctx, callerID, "update", "redirection_host", id, existing, host, ip)
	return host, syncServers(ctx, s.sync, existing.ServerID, host.ServerID)
}

func (s *RedirectionHostService) Delete(ctx context.Context, callerID *int64, id int64, ip string) error {
	host, err := s.GetByID(ctx, id)
	if err != nil || host == nil {
		return fmt.Errorf("redirection host not found")
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM redirection_hosts WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	s.audit.HostChange(ctx, callerID, "delete", "redirection_host", id, host, nil, ip)
	return syncServers(ctx, s.sync, host.ServerID)
}

func (s *RedirectionHostService) SetEnabled(ctx context.Context, callerID *int64, id int64, enabled bool, ip string) error {
	before, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if before == nil {
		return fmt.Errorf("host not found")
	}

	if _, err := s.db.ExecContext(ctx,
		"UPDATE redirection_hosts SET enabled=?, updated_at=datetime('now') WHERE id=?",
		boolToInt(enabled), id,
	); err != nil {
		return fmt.Errorf("update enabled: %w", err)
	}
	host, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if host == nil {
		return fmt.Errorf("redirection host not found")
	}
	action := "enable"
	if !enabled {
		action = "disable"
	}
	s.audit.HostChange(ctx, callerID, action, "redirection_host", id, before, host, ip)
	return syncServers(ctx, s.sync, host.ServerID)
}

// ── scan helpers ──────────────────────────────────────────────────────────────

func scanRedirectionHosts(rows *sql.Rows) ([]models.RedirectionHost, error) {
	var hosts []models.RedirectionHost
	for rows.Next() {
		h, err := scanRedirectionHostRow(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, *h)
	}
	return hosts, rows.Err()
}

func scanRedirectionHostRow(row scanner) (*models.RedirectionHost, error) {
	var h models.RedirectionHost
	var preservePath, sslForced, hstsEnabled, enabled int
	var domainNames string
	err := row.Scan(
		&h.ID, &h.ServerID, &domainNames, &h.ForwardScheme, &h.ForwardDomainName, &h.ForwardPath,
		&preservePath, &h.StatusCode, &h.CertificateID, &sslForced, &hstsEnabled,
		&h.AccessListID, &h.AdvancedConfig, &enabled, &h.CaddyID,
		&h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	h.DomainNames = json.RawMessage(domainNames)
	h.PreservePath = preservePath == 1
	h.SSLForced = sslForced == 1
	h.HSTSEnabled = hstsEnabled == 1
	h.Enabled = enabled == 1
	return &h, nil
}

// validateDomainNames is a shared helper used by multiple host services.
func validateDomainNames(domainNames json.RawMessage) error {
	var domains []string
	if err := json.Unmarshal(domainNames, &domains); err != nil || len(domains) == 0 {
		return fmt.Errorf("at least one domain name is required")
	}
	return nil
}
