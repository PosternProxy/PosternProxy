package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/security"
)

// RollbackFn applies a JSON snapshot to restore an entity to a previous state.
// callerID is the user performing the rollback; ip is their remote address.
type RollbackFn func(ctx context.Context, callerID *int64, id int64, snapshot []byte, ip string) error

// ConfigHistoryService records entity snapshots before changes and supports rollback.
type ConfigHistoryService struct {
	key      [32]byte
	db       *sql.DB
	rollback map[string]RollbackFn
}

// NewConfigHistoryService creates a ConfigHistoryService backed by db.
func NewConfigHistoryService(db *sql.DB, key [32]byte) *ConfigHistoryService {
	return &ConfigHistoryService{
		db:       db,
		key:      key,
		rollback: make(map[string]RollbackFn),
	}
}

// Register wires a rollback function for a given resource type.
// Call this from main.go after all services are constructed.
func (s *ConfigHistoryService) Register(resourceType string, fn RollbackFn) {
	s.rollback[resourceType] = fn
}

// Snapshot records the current state of an entity before a mutating operation.
// It must be called BEFORE the mutation so the snapshot reflects the prior state.
// Pass nil entity for 'create' actions (nothing to snapshot before creation).
func (s *ConfigHistoryService) Snapshot(ctx context.Context, resourceType string, resourceID int64, action string, entity any, callerID *int64) error {
	var snapshotJSON string
	if entity != nil {
		b, err := json.Marshal(entity)
		if err != nil {
			return fmt.Errorf("marshal snapshot: %w", err)
		}
		snapshotJSON = string(b)
	} else {
		snapshotJSON = "null"
	}
	if snapshotJSON != "null" {
		protected, err := security.Encrypt(s.key, snapshotJSON)
		if err != nil {
			return fmt.Errorf("encrypt configuration snapshot: %w", err)
		}
		snapshotJSON = "enc:v1:" + protected
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO config_history (resource_type, resource_id, action, snapshot, created_by)
		VALUES (?, ?, ?, ?, ?)`,
		resourceType, resourceID, action, snapshotJSON, callerID,
	)
	return err
}

// List returns history entries for a resource, newest first.
func (s *ConfigHistoryService) List(ctx context.Context, resourceType string, resourceID int64) ([]models.ConfigHistory, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, resource_type, resource_id, action, snapshot, created_by, created_at
		FROM config_history
		WHERE resource_type = ? AND resource_id = ?
		ORDER BY created_at DESC
		LIMIT 100`,
		resourceType, resourceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.ConfigHistory
	for rows.Next() {
		var e models.ConfigHistory
		if err := rows.Scan(&e.ID, &e.ResourceType, &e.ResourceID, &e.Action, &e.Snapshot, &e.CreatedBy, &e.CreatedAt); err != nil {
			return nil, err
		}
		if e.Snapshot != "null" {
			e.Snapshot = "{}"
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Rollback restores an entity to the state captured in a history entry.
func (s *ConfigHistoryService) Rollback(ctx context.Context, callerID *int64, historyID int64, ip string) error {
	var entry models.ConfigHistory
	err := s.db.QueryRowContext(ctx, `
		SELECT id, resource_type, resource_id, action, snapshot
		FROM config_history WHERE id = ?`, historyID,
	).Scan(&entry.ID, &entry.ResourceType, &entry.ResourceID, &entry.Action, &entry.Snapshot)
	if err == sql.ErrNoRows {
		return fmt.Errorf("history entry not found")
	}
	if err != nil {
		return err
	}

	if entry.Snapshot == "null" || entry.Snapshot == "" {
		return fmt.Errorf("cannot roll back a 'create' snapshot (entity did not exist before this action)")
	}

	fn, ok := s.rollback[entry.ResourceType]
	if !ok {
		return fmt.Errorf("no rollback handler registered for resource type %q", entry.ResourceType)
	}

	if strings.HasPrefix(entry.Snapshot, "enc:v1:") {
		plain, err := security.Decrypt(s.key, strings.TrimPrefix(entry.Snapshot, "enc:v1:"))
		if err != nil {
			return fmt.Errorf("decrypt configuration snapshot: %w", err)
		}
		entry.Snapshot = plain
	}
	return fn(ctx, callerID, entry.ResourceID, []byte(entry.Snapshot), ip)
}

// ProtectExisting can resume after a partial run; encrypted rows are left alone.
func (s *ConfigHistoryService) ProtectExisting(ctx context.Context) error {
	for {
		rows, err := s.db.QueryContext(ctx, `SELECT id,snapshot FROM config_history WHERE snapshot NOT LIKE 'enc:v1:%' AND snapshot NOT IN ('','null') LIMIT 100`)
		if err != nil {
			return fmt.Errorf("read legacy configuration history: %w", err)
		}
		type legacy struct {
			id       int64
			snapshot string
		}
		var batch []legacy
		for rows.Next() {
			var item legacy
			if err := rows.Scan(&item.id, &item.snapshot); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, item)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		for _, item := range batch {
			protected, err := security.Encrypt(s.key, item.snapshot)
			if err != nil {
				return fmt.Errorf("encrypt legacy history: %w", err)
			}
			if _, err := s.db.ExecContext(ctx, `UPDATE config_history SET snapshot=? WHERE id=? AND snapshot=?`, "enc:v1:"+protected, item.id, item.snapshot); err != nil {
				return fmt.Errorf("protect legacy history: %w", err)
			}
		}
	}
}

// ListAll returns recent history across all resource types (for the audit log UI).
func (s *ConfigHistoryService) ListAll(ctx context.Context, limit int) ([]models.ConfigHistory, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, resource_type, resource_id, action, snapshot, created_by, created_at
		FROM config_history
		ORDER BY created_at DESC
		LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.ConfigHistory
	for rows.Next() {
		var e models.ConfigHistory
		if err := rows.Scan(&e.ID, &e.ResourceType, &e.ResourceID, &e.Action, &e.Snapshot, &e.CreatedBy, &e.CreatedAt); err != nil {
			return nil, err
		}
		if e.Snapshot != "null" {
			e.Snapshot = "{}"
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
