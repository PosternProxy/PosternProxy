package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/models"
)

// StreamHostService manages Layer-4 stream hosts in SQLite and Caddy.
type StreamHostService struct {
	db    *sql.DB
	caddy *caddy.Client
	sync  *CaddySyncService
	audit *AuditService
}

// NewStreamHostService creates a StreamHostService backed by db.
func NewStreamHostService(db *sql.DB, caddyClient *caddy.Client, audit *AuditService, sync *CaddySyncService) *StreamHostService {
	return &StreamHostService{db: db, caddy: caddyClient, sync: sync, audit: audit}
}

// ── Input type ────────────────────────────────────────────────────────────────

// CreateStreamHostInput carries all fields accepted when creating or updating a
// StreamHost.
type CreateStreamHostInput struct {
	ServerID         int64   `json:"server_id"`
	IncomingPort     int     `json:"incoming_port"`
	ForwardHost      string  `json:"forward_host"`
	ForwardPort      int     `json:"forward_port"`
	Protocol         string  `json:"protocol"`
	TCPSNIMatch      *string `json:"tcp_sni_match"`
	TCPProxyProtocol bool    `json:"tcp_proxy_protocol"`
	UDPTimeout       int     `json:"udp_timeout"`
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

// List returns all stream hosts ordered by ID.
func (s *StreamHostService) List(ctx context.Context) ([]models.StreamHost, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, server_id, incoming_port, forward_host, forward_port, protocol,
		       tcp_sni_match, tcp_proxy_protocol, udp_timeout, enabled, caddy_id,
		       created_at, updated_at
		FROM stream_hosts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStreamHosts(rows)
}

// ListByServer returns stream hosts for a specific server, ordered by ID.
func (s *StreamHostService) ListByServer(ctx context.Context, serverID int64) ([]models.StreamHost, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, server_id, incoming_port, forward_host, forward_port, protocol,
		       tcp_sni_match, tcp_proxy_protocol, udp_timeout, enabled, caddy_id,
		       created_at, updated_at
		FROM stream_hosts WHERE server_id = ? ORDER BY id`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanStreamHosts(rows)
}

// GetByID returns the stream host with the given ID, or nil if not found.
func (s *StreamHostService) GetByID(ctx context.Context, id int64) (*models.StreamHost, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, server_id, incoming_port, forward_host, forward_port, protocol,
		       tcp_sni_match, tcp_proxy_protocol, udp_timeout, enabled, caddy_id,
		       created_at, updated_at
		FROM stream_hosts WHERE id = ?`, id)
	h, err := scanStreamHostRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

// Create inserts a new stream host and registers it in Caddy's layer4 app.
func (s *StreamHostService) Create(ctx context.Context, callerID *int64, input CreateStreamHostInput, ip string) (*models.StreamHost, error) {
	if err := validateStreamInput(input); err != nil {
		return nil, err
	}
	if input.ServerID == 0 {
		input.ServerID = 1
	}
	if input.Protocol == "" {
		input.Protocol = "tcp"
	}
	if input.UDPTimeout == 0 {
		input.UDPTimeout = 30
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO stream_hosts
		  (server_id, incoming_port, forward_host, forward_port, protocol,
		   tcp_sni_match, tcp_proxy_protocol, udp_timeout)
		VALUES (?,?,?,?,?,?,?,?)`,
		input.ServerID, input.IncomingPort, input.ForwardHost, input.ForwardPort,
		input.Protocol, input.TCPSNIMatch, boolToInt(input.TCPProxyProtocol), input.UDPTimeout,
	)
	if err != nil {
		return nil, fmt.Errorf("insert stream host: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	caddyID := fmt.Sprintf("stream_host_%d", id)
	if _, err := tx.ExecContext(ctx, "UPDATE stream_hosts SET caddy_id = ? WHERE id = ?", caddyID, id); err != nil {
		return nil, fmt.Errorf("set caddy_id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	host, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.audit.HostChange(ctx, callerID, "create", "stream_host", id, nil, host, ip)
	return host, syncServers(ctx, s.sync, host.ServerID)
}

func (s *StreamHostService) Update(ctx context.Context, callerID *int64, id int64, input CreateStreamHostInput, ip string) (*models.StreamHost, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("stream host not found")
	}
	if err := validateStreamInput(input); err != nil {
		return nil, err
	}

	if input.ServerID == 0 {
		input.ServerID = existing.ServerID
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE stream_hosts SET
		  server_id=?,
		  incoming_port=?, forward_host=?, forward_port=?, protocol=?,
		  tcp_sni_match=?, tcp_proxy_protocol=?, udp_timeout=?, updated_at=datetime('now')
		WHERE id=?`,
		input.ServerID, input.IncomingPort, input.ForwardHost, input.ForwardPort, input.Protocol,
		input.TCPSNIMatch, boolToInt(input.TCPProxyProtocol), input.UDPTimeout, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update stream host: %w", err)
	}

	host, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.audit.HostChange(ctx, callerID, "update", "stream_host", id, existing, host, ip)
	return host, syncServers(ctx, s.sync, existing.ServerID, host.ServerID)
}

func (s *StreamHostService) Delete(ctx context.Context, callerID *int64, id int64, ip string) error {
	host, err := s.GetByID(ctx, id)
	if err != nil || host == nil {
		return fmt.Errorf("stream host not found")
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM stream_hosts WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	s.audit.HostChange(ctx, callerID, "delete", "stream_host", id, host, nil, ip)
	return syncServers(ctx, s.sync, host.ServerID)
}

func (s *StreamHostService) SetEnabled(ctx context.Context, callerID *int64, id int64, enabled bool, ip string) error {
	before, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if before == nil {
		return fmt.Errorf("host not found")
	}

	if _, err := s.db.ExecContext(ctx,
		"UPDATE stream_hosts SET enabled=?, updated_at=datetime('now') WHERE id=?",
		boolToInt(enabled), id,
	); err != nil {
		return fmt.Errorf("update enabled: %w", err)
	}
	host, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if host == nil {
		return fmt.Errorf("stream host not found")
	}
	action := "enable"
	if !enabled {
		action = "disable"
	}
	s.audit.HostChange(ctx, callerID, action, "stream_host", id, before, host, ip)
	return syncServers(ctx, s.sync, host.ServerID)
}

func validateStreamInput(input CreateStreamHostInput) error {
	if input.IncomingPort < 1 || input.IncomingPort > 65535 {
		return fmt.Errorf("incoming port must be between 1 and 65535")
	}
	if input.ForwardHost == "" {
		return fmt.Errorf("forward host is required")
	}
	if input.ForwardPort < 1 || input.ForwardPort > 65535 {
		return fmt.Errorf("forward port must be between 1 and 65535")
	}
	return nil
}

// ── scan helpers ──────────────────────────────────────────────────────────────

func scanStreamHosts(rows *sql.Rows) ([]models.StreamHost, error) {
	var hosts []models.StreamHost
	for rows.Next() {
		h, err := scanStreamHostRow(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, *h)
	}
	return hosts, rows.Err()
}

func scanStreamHostRow(row scanner) (*models.StreamHost, error) {
	var h models.StreamHost
	var proxyProtocol, enabled int
	err := row.Scan(
		&h.ID, &h.ServerID, &h.IncomingPort, &h.ForwardHost, &h.ForwardPort, &h.Protocol,
		&h.TCPSNIMatch, &proxyProtocol, &h.UDPTimeout, &enabled, &h.CaddyID,
		&h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	h.TCPProxyProtocol = proxyProtocol == 1
	h.Enabled = enabled == 1
	return &h, nil
}
