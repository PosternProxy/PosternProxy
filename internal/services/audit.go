package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/rs/zerolog/log"
)

// AuditService writes user-action records to the audit_log table.
type AuditService struct {
	db *sql.DB
}

// NewAuditService creates an AuditService backed by db.
func NewAuditService(db *sql.DB) *AuditService {
	return &AuditService{db: db}
}

func (s *AuditService) HostChange(ctx context.Context, actor *int64, action, resource string, id int64, before, after any, ip string) {
	beforeSnapshot, err := hostAuditSnapshot(before)
	if err != nil {
		log.Error().Str("resource", resource).Msg("failed to prepare host audit snapshot")
		return
	}
	afterSnapshot, err := hostAuditSnapshot(after)
	if err != nil {
		log.Error().Str("resource", resource).Msg("failed to prepare host audit snapshot")
		return
	}
	if err := s.Log(ctx, actor, action, resource, &id, map[string]any{"before": beforeSnapshot, "after": afterSnapshot}, ip); err != nil {
		log.Error().Err(err).Str("resource", resource).Int64("resource_id", id).Msg("failed to audit host change")
	}
}

func hostAuditSnapshot(entity any) (map[string]json.RawMessage, error) {
	if entity == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(entity)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		return nil, err
	}
	safe := map[string]json.RawMessage{}
	// Route bodies, headers, and upstream configuration can carry credentials.
	for _, key := range []string{"id", "server_id", "domain_names", "enabled", "certificate_id", "access_list_id", "incoming_port", "forward_port", "protocol", "status_code", "http_only", "ssl_forced"} {
		if value, ok := fields[key]; ok {
			safe[key] = value
		}
	}
	return safe, nil
}

// Log records a single user action. It is best-effort: callers should not
// propagate errors from this method.
func (s *AuditService) Log(ctx context.Context, userID *int64, action, resourceType string, resourceID *int64, details any, ipAddress string) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	if _, err = s.db.ExecContext(ctx,
		`INSERT INTO audit_log (user_id, action, resource_type, resource_id, details, ip_address) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, action, resourceType, resourceID, string(detailsJSON), ipAddress,
	); err != nil {
		// Audit writes are best-effort and most privileged-action call sites
		// discard this error, so record the failure here. This keeps every
		// dropped audit record observable in the logs regardless of the caller.
		event := log.Error().Err(err).Str("action", action).Str("resource_type", resourceType)
		if resourceID != nil {
			event = event.Int64("resource_id", *resourceID)
		}
		event.Msg("failed to write audit record")
		return err
	}
	return nil
}

// sensitiveSettingKey reports whether a settings key's value must be masked in
// audit snapshots. The currently editable keys are non-secret, but this guards
// any future secret-bearing key (API keys, passwords, tokens).
func sensitiveSettingKey(key string) bool {
	lower := strings.ToLower(key)
	for _, marker := range []string{"secret", "password", "passwd", "token", "credential", "private", "key", "auth"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func redactSettings(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	safe := make(map[string]string, len(values))
	for k, v := range values {
		if sensitiveSettingKey(k) {
			safe[k] = "[redacted]"
		} else {
			safe[k] = v
		}
	}
	return safe
}

// redactValues masks every value, preserving only the keys. Used for a rejected
// settings attempt, whose values are unvalidated input (unknown keys bypass the
// name-based allowlist, so no value can be trusted as non-secret).
func redactValues(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	safe := make(map[string]string, len(values))
	for k := range values {
		safe[k] = "[redacted]"
	}
	return safe
}

// SettingsChange records a settings edit with a redacted before/after snapshot
// and an explicit success outcome. Like the other helpers it is best-effort.
// On a rejected change every attempted value is masked so an unknown or
// secret-bearing key cannot leak its value into the audit record.
func (s *AuditService) SettingsChange(ctx context.Context, actor *int64, success bool, before, after map[string]string, ip string) {
	afterSnapshot := redactSettings(after)
	if !success {
		afterSnapshot = redactValues(after)
	}
	_ = s.Log(ctx, actor, "update", "settings", nil, map[string]any{
		"success": success,
		"before":  redactSettings(before),
		"after":   afterSnapshot,
	}, ip)
}

// List returns a paginated slice of audit log entries and the total count.
// Pass an empty resourceType to return entries for all resource types.
func (s *AuditService) List(ctx context.Context, resourceType string, serverID *int64, page, limit int) ([]models.AuditLog, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit

	query := "SELECT id, user_id, action, resource_type, resource_id, details, ip_address, created_at FROM audit_log"
	countQuery := "SELECT COUNT(*) FROM audit_log"
	args := []any{}
	where := ""

	if resourceType != "" {
		where = " WHERE resource_type = ?"
		args = append(args, resourceType)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(ctx, query+where+" ORDER BY created_at DESC LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []models.AuditLog
	for rows.Next() {
		var e models.AuditLog
		// The driver returns the TEXT details column as a string, which can't be
		// scanned straight into json.RawMessage — scan into a string first, then
		// convert (the same pattern used for other JSON columns).
		var details string
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.ResourceType, &e.ResourceID, &details, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		if details == "" {
			details = "{}"
		}
		e.Details = json.RawMessage(details)
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}
