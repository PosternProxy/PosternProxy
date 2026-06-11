package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/portforward"
	"github.com/rs/zerolog/log"
)

// PortForwardService manages port forwarding rules in the database and
// applies/removes the corresponding OS-level rules via the portforward package.
type PortForwardService struct {
	db    *sql.DB
	audit *AuditService
}

// NewPortForwardService creates a PortForwardService backed by db.
func NewPortForwardService(db *sql.DB, audit *AuditService) *PortForwardService {
	return &PortForwardService{db: db, audit: audit}
}

// CreatePortForwardInput is the input type for Create and Update.
type CreatePortForwardInput struct {
	Protocol     string `json:"protocol"`
	IncomingPort int    `json:"incoming_port"`
	ForwardHost  string `json:"forward_host"`
	ForwardPort  int    `json:"forward_port"`
	Interface    string `json:"interface"`
	Method       string `json:"method"`
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

// List returns all port-forward rules ordered by ID.
func (s *PortForwardService) List(ctx context.Context) ([]models.PortForward, error) {
	return s.listQuery(ctx, `
		SELECT id, server_id, protocol, incoming_port, forward_host, forward_port,
		       "interface", method, enabled, created_at, updated_at
		FROM port_forwards ORDER BY id`)
}

// ListByServer returns port-forward rules for a specific server, ordered by ID.
func (s *PortForwardService) ListByServer(ctx context.Context, serverID int64) ([]models.PortForward, error) {
	return s.listQuery(ctx, `
		SELECT id, server_id, protocol, incoming_port, forward_host, forward_port,
		       "interface", method, enabled, created_at, updated_at
		FROM port_forwards WHERE server_id = ? ORDER BY id`, serverID)
}

func (s *PortForwardService) listQuery(ctx context.Context, query string, args ...any) ([]models.PortForward, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pfs []models.PortForward
	for rows.Next() {
		var pf models.PortForward
		if err := rows.Scan(&pf.ID, &pf.ServerID, &pf.Protocol, &pf.IncomingPort,
			&pf.ForwardHost, &pf.ForwardPort, &pf.Interface, &pf.Method,
			&pf.Enabled, &pf.CreatedAt, &pf.UpdatedAt); err != nil {
			return nil, err
		}
		pfs = append(pfs, pf)
	}
	return pfs, rows.Err()
}

// GetByID returns the port-forward rule with the given ID, or nil if not found.
func (s *PortForwardService) GetByID(ctx context.Context, id int64) (*models.PortForward, error) {
	var pf models.PortForward
	err := s.db.QueryRowContext(ctx, `
		SELECT id, server_id, protocol, incoming_port, forward_host, forward_port,
		       "interface", method, enabled, created_at, updated_at
		FROM port_forwards WHERE id = ?`, id).
		Scan(&pf.ID, &pf.ServerID, &pf.Protocol, &pf.IncomingPort,
			&pf.ForwardHost, &pf.ForwardPort, &pf.Interface, &pf.Method,
			&pf.Enabled, &pf.CreatedAt, &pf.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &pf, err
}

// Create inserts a new port-forward rule and applies it at the OS level.
func (s *PortForwardService) Create(ctx context.Context, callerID *int64, input CreatePortForwardInput, ip string) (*models.PortForward, error) {
	if err := validatePortForwardInput(input); err != nil {
		return nil, err
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO port_forwards (protocol, incoming_port, forward_host, forward_port, "interface", method, enabled)
		VALUES (?, ?, ?, ?, ?, ?, 1)`,
		input.Protocol, input.IncomingPort, input.ForwardHost, input.ForwardPort, input.Interface, input.Method,
	)
	if err != nil {
		return nil, fmt.Errorf("insert port forward: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	pf, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.audit.Log(ctx, callerID, "create", "port_forward", &id, map[string]any{
		"incoming_port": input.IncomingPort,
		"forward":       fmt.Sprintf("%s:%d", input.ForwardHost, input.ForwardPort),
		"method":        input.Method,
	}, ip)

	// Apply rule immediately — records are created enabled.
	if err := portforward.ApplyRule(pf); err != nil {
		// Non-fatal: the record is saved; the rule can be re-applied on next startup.
		_ = err
	}

	return pf, nil
}

func (s *PortForwardService) Update(ctx context.Context, callerID *int64, id int64, input CreatePortForwardInput, ip string) (*models.PortForward, error) {
	if err := validatePortForwardInput(input); err != nil {
		return nil, err
	}

	existing, err := s.GetByID(ctx, id)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("port forward not found")
	}

	// Remove the old OS rule before changing the parameters.
	portforward.RemoveRule(existing)

	if _, err := s.db.ExecContext(ctx, `
		UPDATE port_forwards
		SET protocol=?, incoming_port=?, forward_host=?, forward_port=?,
		    "interface"=?, method=?, updated_at=datetime('now')
		WHERE id=?`,
		input.Protocol, input.IncomingPort, input.ForwardHost, input.ForwardPort,
		input.Interface, input.Method, id,
	); err != nil {
		return nil, fmt.Errorf("update port forward: %w", err)
	}

	pf, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Re-apply if still enabled.
	if pf.Enabled {
		if err := portforward.ApplyRule(pf); err != nil {
			log.Warn().Err(err).Int64("id", id).Msg("port forward: apply after update failed")
		}
	}

	s.audit.Log(ctx, callerID, "update", "port_forward", &id, map[string]any{
		"incoming_port": input.IncomingPort,
		"method":        input.Method,
	}, ip)
	return pf, nil
}

func (s *PortForwardService) Delete(ctx context.Context, callerID *int64, id int64, ip string) error {
	pf, err := s.GetByID(ctx, id)
	if err != nil || pf == nil {
		return fmt.Errorf("port forward not found")
	}

	portforward.RemoveRule(pf)

	if _, err := s.db.ExecContext(ctx, "DELETE FROM port_forwards WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete port forward: %w", err)
	}

	s.audit.Log(ctx, callerID, "delete", "port_forward", &id, nil, ip)
	return nil
}

func (s *PortForwardService) SetEnabled(ctx context.Context, callerID *int64, id int64, enabled bool, ip string) (*models.PortForward, error) {
	pf, err := s.GetByID(ctx, id)
	if err != nil || pf == nil {
		return nil, fmt.Errorf("port forward not found")
	}

	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE port_forwards SET enabled=?, updated_at=datetime('now') WHERE id=?`, enabledInt, id); err != nil {
		return nil, fmt.Errorf("update port forward: %w", err)
	}

	if enabled {
		if err := portforward.ApplyRule(pf); err != nil {
			log.Warn().Err(err).Int64("id", id).Msg("port forward: apply after enable failed")
		}
	} else {
		portforward.RemoveRule(pf)
	}

	action := "enable"
	if !enabled {
		action = "disable"
	}
	s.audit.Log(ctx, callerID, action, "port_forward", &id, nil, ip)

	pf.Enabled = enabled
	return pf, nil
}

// RestoreAll re-applies OS rules for all enabled port forwards.
// Called on application startup so rules persist across reboots.
func (s *PortForwardService) RestoreAll(ctx context.Context) {
	pfs, err := s.List(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("port forward: failed to list rules during startup restore")
		return
	}
	for i := range pfs {
		if pfs[i].Enabled {
			if err := portforward.ApplyRule(&pfs[i]); err != nil {
				log.Warn().Err(err).Int64("id", pfs[i].ID).Int("port", pfs[i].IncomingPort).Msg("port forward: failed to restore rule on startup")
			}
		}
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

var validPFProtocols = map[string]bool{"tcp": true, "udp": true, "tcp+udp": true}
var validPFMethods = map[string]bool{"iptables": true, "nftables": true, "socat": true}

func validatePortForwardInput(input CreatePortForwardInput) error {
	if !validPFProtocols[input.Protocol] {
		return fmt.Errorf("protocol must be tcp, udp, or tcp+udp")
	}
	if input.IncomingPort < 1 || input.IncomingPort > 65535 {
		return fmt.Errorf("incoming_port must be between 1 and 65535")
	}
	if input.ForwardHost == "" {
		return fmt.Errorf("forward_host is required")
	}
	if input.ForwardPort < 1 || input.ForwardPort > 65535 {
		return fmt.Errorf("forward_port must be between 1 and 65535")
	}
	if !validPFMethods[input.Method] {
		return fmt.Errorf("method must be iptables, nftables, or socat")
	}
	return nil
}
