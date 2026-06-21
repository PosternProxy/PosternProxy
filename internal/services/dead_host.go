package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/rs/zerolog/log"
)

// DeadHostService manages dead (404-returning) virtual hosts in SQLite and Caddy.
type DeadHostService struct {
	db    *sql.DB
	caddy *caddy.Client
	sync  *CaddySyncService
	audit *AuditService
}

// NewDeadHostService creates a DeadHostService backed by db.
func NewDeadHostService(db *sql.DB, caddyClient *caddy.Client, audit *AuditService, sync *CaddySyncService) *DeadHostService {
	return &DeadHostService{db: db, caddy: caddyClient, sync: sync, audit: audit}
}

// ── Input type ────────────────────────────────────────────────────────────────

// CreateDeadHostInput carries all fields accepted when creating or updating a
// DeadHost.
type CreateDeadHostInput struct {
	ServerID    int64           `json:"server_id"`
	DomainNames json.RawMessage `json:"domain_names"`
	StatusCode  int             `json:"status_code"`
	CustomBody  string          `json:"custom_body"`
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

// List returns all dead hosts ordered by ID.
func (s *DeadHostService) List(ctx context.Context) ([]models.DeadHost, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, server_id, domain_names, status_code, custom_body, enabled, caddy_id, created_at, updated_at
		FROM dead_hosts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeadHosts(rows)
}

// ListByServer returns dead hosts for a specific server, ordered by ID.
func (s *DeadHostService) ListByServer(ctx context.Context, serverID int64) ([]models.DeadHost, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, server_id, domain_names, status_code, custom_body, enabled, caddy_id, created_at, updated_at
		FROM dead_hosts WHERE server_id = ? ORDER BY id`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDeadHosts(rows)
}

// GetByID returns the dead host with the given ID, or nil if not found.
func (s *DeadHostService) GetByID(ctx context.Context, id int64) (*models.DeadHost, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, server_id, domain_names, status_code, custom_body, enabled, caddy_id, created_at, updated_at
		FROM dead_hosts WHERE id = ?`, id)
	h, err := scanDeadHostRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

// Create inserts a new dead host and registers a 404 route in Caddy.
func (s *DeadHostService) Create(ctx context.Context, callerID *int64, input CreateDeadHostInput, ip string) (*models.DeadHost, error) {
	if err := validateDomainNames(input.DomainNames); err != nil {
		return nil, err
	}
	if input.ServerID == 0 {
		input.ServerID = 1
	}
	if input.StatusCode == 0 {
		input.StatusCode = 404
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO dead_hosts (server_id, domain_names, status_code, custom_body)
		VALUES (?,?,?,?)`,
		input.ServerID, string(input.DomainNames), input.StatusCode, input.CustomBody,
	)
	if err != nil {
		return nil, fmt.Errorf("insert dead host: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	caddyID := fmt.Sprintf("dead_host_%d", id)
	if _, err := tx.ExecContext(ctx, "UPDATE dead_hosts SET caddy_id = ? WHERE id = ?", caddyID, id); err != nil {
		return nil, fmt.Errorf("set caddy_id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	host, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.pushToCaddy(ctx, host); err != nil {
		log.Error().Err(err).Int64("dead_host_id", id).Msg("failed to apply dead host to Caddy")
	}
	s.audit.Log(ctx, callerID, "create", "dead_host", &id, map[string]any{"domains": string(input.DomainNames)}, ip)
	return host, nil
}

func (s *DeadHostService) Update(ctx context.Context, callerID *int64, id int64, input CreateDeadHostInput, ip string) (*models.DeadHost, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("dead host not found")
	}
	if err := validateDomainNames(input.DomainNames); err != nil {
		return nil, err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE dead_hosts SET domain_names=?, status_code=?, custom_body=?, updated_at=datetime('now')
		WHERE id=?`,
		string(input.DomainNames), input.StatusCode, input.CustomBody, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update dead host: %w", err)
	}

	host, _ := s.GetByID(ctx, id)
	if err := s.pushToCaddy(ctx, host); err != nil {
		log.Error().Err(err).Int64("dead_host_id", id).Msg("failed to apply dead host to Caddy")
	}
	s.audit.Log(ctx, callerID, "update", "dead_host", &id, map[string]any{"domains": string(input.DomainNames)}, ip)
	return host, nil
}

func (s *DeadHostService) Delete(ctx context.Context, callerID *int64, id int64, ip string) error {
	host, err := s.GetByID(ctx, id)
	if err != nil || host == nil {
		return fmt.Errorf("dead host not found")
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM dead_hosts WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	s.syncAfterDelete(ctx, host.ServerID, host.CaddyID)
	s.audit.Log(ctx, callerID, "delete", "dead_host", &id, nil, ip)
	return nil
}

func (s *DeadHostService) SetEnabled(ctx context.Context, callerID *int64, id int64, enabled bool, ip string) error {
	if _, err := s.db.ExecContext(ctx,
		"UPDATE dead_hosts SET enabled=?, updated_at=datetime('now') WHERE id=?",
		boolToInt(enabled), id,
	); err != nil {
		return fmt.Errorf("update enabled: %w", err)
	}
	host, _ := s.GetByID(ctx, id)
	if host == nil {
		return fmt.Errorf("dead host not found")
	}
	if enabled {
		if err := s.pushToCaddy(ctx, host); err != nil {
			log.Error().Err(err).Int64("dead_host_id", id).Msg("failed to enable dead host in Caddy")
		}
	} else {
		s.syncAfterDelete(ctx, host.ServerID, host.CaddyID)
	}
	action := "enable"
	if !enabled {
		action = "disable"
	}
	s.audit.Log(ctx, callerID, action, "dead_host", &id, nil, ip)
	return nil
}

// pushToCaddy rebuilds the full config. Dead hosts are served on both the :80 and
// :443 servers, so a single :443 patch can't represent them.
func (s *DeadHostService) pushToCaddy(ctx context.Context, h *models.DeadHost) error {
	if s.sync == nil {
		return nil
	}
	return s.sync.SyncServer(ctx, h.ServerID)
}

func (s *DeadHostService) syncAfterDelete(ctx context.Context, serverID int64, _ string) {
	if s.sync == nil {
		return
	}
	if err := s.sync.SyncServer(ctx, serverID); err != nil {
		log.Warn().Err(err).Int64("server_id", serverID).Msg("dead host: sync after delete failed")
	}
}

// ── scan helpers ──────────────────────────────────────────────────────────────

func scanDeadHosts(rows *sql.Rows) ([]models.DeadHost, error) {
	var hosts []models.DeadHost
	for rows.Next() {
		h, err := scanDeadHostRow(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, *h)
	}
	return hosts, rows.Err()
}

func scanDeadHostRow(row scanner) (*models.DeadHost, error) {
	var h models.DeadHost
	var enabled int
	var domainNames string
	err := row.Scan(
		&h.ID, &h.ServerID, &domainNames, &h.StatusCode, &h.CustomBody, &enabled, &h.CaddyID, &h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	h.DomainNames = json.RawMessage(domainNames)
	h.Enabled = enabled == 1
	return &h, nil
}
