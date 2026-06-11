package services

import (
	"context"
	"database/sql"
	"fmt"
)

// SettingKey is a well-known settings key.
type SettingKey = string

const (
	SettingCaddyAPIURL     SettingKey = "caddy_api_url"
	SettingCertStoragePath SettingKey = "cert_storage_path"
	SettingSetupComplete   SettingKey = "setup_complete"
)

// AllowedKeys is the set of keys the API will expose and allow edits to.
var AllowedKeys = []SettingKey{
	SettingCaddyAPIURL,
	SettingCertStoragePath,
}

// SettingsService manages key/value settings stored in SQLite.
type SettingsService struct {
	db *sql.DB
}

// NewSettingsService creates a SettingsService backed by db.
func NewSettingsService(db *sql.DB) *SettingsService {
	return &SettingsService{db: db}
}

// GetAll returns all allowed settings as a map.
func (s *SettingsService) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

// Get returns a single setting by key. Returns ("", false, nil) if not found.
func (s *SettingsService) Get(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// Set upserts a setting. Only allowed keys may be set via API.
func (s *SettingsService) Set(ctx context.Context, key, value string) error {
	if !isAllowedKey(key) {
		return fmt.Errorf("unknown setting key: %s", key)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

// SetBulk upserts multiple settings at once. Rejects any disallowed key before
// writing anything.
func (s *SettingsService) SetBulk(ctx context.Context, updates map[string]string) error {
	for k := range updates {
		if !isAllowedKey(k) {
			return fmt.Errorf("unknown setting key: %s", k)
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// Standard Go pattern: rollback after a successful commit is a no-op
	// and returns sql.ErrTxDone, which is uninteresting.
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for k, v := range updates {
		if _, err := stmt.ExecContext(ctx, k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func isAllowedKey(key string) bool {
	for _, k := range AllowedKeys {
		if k == key {
			return true
		}
	}
	return false
}
