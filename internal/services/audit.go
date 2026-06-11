package services

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/posternproxy/posternproxy/internal/models"
)

// AuditService writes user-action records to the audit_log table.
type AuditService struct {
	db *sql.DB
}

// NewAuditService creates an AuditService backed by db.
func NewAuditService(db *sql.DB) *AuditService {
	return &AuditService{db: db}
}

// Log records a single user action. It is best-effort: callers should not
// propagate errors from this method.
func (s *AuditService) Log(ctx context.Context, userID *int64, action, resourceType string, resourceID *int64, details any, ipAddress string) error {
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO audit_log (user_id, action, resource_type, resource_id, details, ip_address) VALUES (?, ?, ?, ?, ?, ?)`,
		userID, action, resourceType, resourceID, string(detailsJSON), ipAddress,
	)
	return err
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
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.ResourceType, &e.ResourceID, &e.Details, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}
