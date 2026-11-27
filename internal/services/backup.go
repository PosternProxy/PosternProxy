package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/posternproxy/posternproxy/internal/certstore"
	"github.com/posternproxy/posternproxy/internal/models"
	"sort"
	"strings"
	"time"
)

// BackupPayload is the JSON structure for export/import.
// Users table is intentionally excluded — it is never overwritten by a restore.
type BackupPayload struct {
	Version    string                      `json:"version"`
	ExportedAt string                      `json:"exported_at"`
	Tables     map[string][]map[string]any `json:"tables"`
}

var backupVersion = "1.0"

// tableOrder determines insert order on restore to respect FK constraints.
var tableOrder = []string{
	"certificates",
	"access_lists",
	"access_list_clients",
	"proxy_hosts",
	"redirection_hosts",
	"stream_hosts",
	"dead_hosts",
	"settings",
}

// tableColumns is the per-table allowlist of valid column names accepted during
// backup import. Column names are interpolated into SQL (they cannot be
// parameterised), so any name not on this list is rejected outright.
//
// Keep this map in sync with migrations: add a column here when you add it to
// the schema, remove it when a DROP COLUMN migration lands.
var tableColumns = map[string]map[string]bool{
	"certificates": strset(
		"id", "name", "provider", "domain_names", "cert_path", "key_path",
		"expires_at", "status", "error_message", "created_at", "updated_at",
	),
	"access_lists": strset(
		"id", "name", "created_at", "updated_at",
	),
	"access_list_clients": strset(
		"id", "access_list_id", "address", "directive", "sort_order",
	),
	"proxy_hosts": strset(
		"id", "server_id", "domain_names", "forward_scheme", "forward_host", "forward_port",
		"certificate_id", "ssl_forced", "hsts_enabled", "hsts_subdomains",
		"access_list_id",
		"advanced_config", "locations", "enabled", "caddy_id",
		// health checks
		"hc_enabled", "hc_active_path", "hc_active_interval", "hc_active_timeout",
		"hc_active_expect_status", "hc_passive_fail_duration", "hc_passive_max_fails",
		"hc_passive_unhealthy_latency",
		// rate limiting
		"rate_limit_enabled", "rate_limit_requests", "rate_limit_window",
		// misc feature columns
		"tls_passthrough", "error_pages", "custom_headers",
		"extra_upstreams", "lb_policy", "http_only", "ntlm_enabled",
		"created_at", "updated_at",
	),
	"redirection_hosts": strset(
		"id", "server_id", "domain_names", "forward_scheme", "forward_domain_name",
		"forward_path", "preserve_path", "status_code", "certificate_id",
		"ssl_forced", "hsts_enabled", "access_list_id", "advanced_config",
		"enabled", "caddy_id", "created_at", "updated_at",
	),
	"stream_hosts": strset(
		"id", "server_id", "incoming_port", "forward_host", "forward_port",
		"protocol", "tcp_sni_match", "tcp_proxy_protocol", "udp_timeout",
		"enabled", "caddy_id", "created_at", "updated_at",
	),
	"dead_hosts": strset(
		"id", "server_id", "domain_names", "certificate_id", "ssl_forced",
		"access_list_id", "advanced_config", "enabled", "caddy_id",
		"status_code", "custom_body", "http_only", "created_at", "updated_at",
	),
	"settings": strset("key", "value"),
}

func strset(cols ...string) map[string]bool {
	m := make(map[string]bool, len(cols))
	for _, c := range cols {
		m[c] = true
	}
	return m
}

// BackupService handles database export and import.
type BackupService struct {
	store   *certstore.Store
	db      *sql.DB
	syncSvc *CaddySyncService
}

func NewBackupService(db *sql.DB, syncSvc *CaddySyncService) *BackupService {
	return &BackupService{db: db, syncSvc: syncSvc}
}

// Export dumps all configured tables to a BackupPayload.
func (s *BackupService) Export(ctx context.Context) (*BackupPayload, error) {
	payload, _, err := s.exportSnapshot(ctx)
	return payload, err
}

func (s *BackupService) exportSnapshot(ctx context.Context) (*BackupPayload, []portableServer, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	payload := &BackupPayload{
		Version:    backupVersion,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Tables:     make(map[string][]map[string]any),
	}

	for _, table := range tableOrder {
		rows, err := dumpTable(ctx, tx, table)
		if err != nil {
			return nil, nil, fmt.Errorf("dump %s: %w", table, err)
		}
		payload.Tables[table] = rows
	}

	rows, err := tx.QueryContext(ctx, `SELECT id,name,hostname,is_local FROM servers ORDER BY id`)
	if err != nil {
		return nil, nil, err
	}
	var servers []portableServer
	for rows.Next() {
		var srv portableServer
		if err := rows.Scan(&srv.ID, &srv.Name, &srv.Hostname, &srv.Local); err != nil {
			rows.Close()
			return nil, nil, err
		}
		servers = append(servers, srv)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	return payload, servers, nil
}

// Import restores a BackupPayload into the database and re-syncs Caddy.
// The users table is never touched. All other tables are cleared and replaced.
func (s *BackupService) Import(ctx context.Context, payload *BackupPayload) (restored int, err error) {
	if err := validateBackup(payload); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	restored, err = replaceBackupTables(ctx, tx, payload)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return restored, s.applyRestored(ctx)
}

func (s *BackupService) ConfigurationStates(ctx context.Context) ([]ConfigState, error) {
	if s.syncSvc == nil {
		return []ConfigState{}, nil
	}
	return s.syncSvc.States(ctx)
}

// dumpTable returns all rows from a table as a slice of generic maps.
func dumpTable(ctx context.Context, tx *sql.Tx, table string) ([]map[string]any, error) {
	rows, err := tx.QueryContext(ctx, "SELECT * FROM "+table) // #nosec G202 -- table name comes from a fixed internal schema list, not user input
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, col := range cols {
			// SQLite returns []byte for TEXT/BLOB; convert to string.
			if b, ok := vals[i].([]byte); ok {
				m[col] = string(b)
			} else {
				m[col] = vals[i]
			}
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// insertRow builds and executes an INSERT OR REPLACE for a single backup row.
// Column names are validated against tableColumns before being interpolated into
// the query — they cannot be parameterised by the database/sql driver, so an
// allowlist is the only safe approach.
// Columns are sorted before insertion for deterministic query shape.
func insertRow(ctx context.Context, tx *sql.Tx, table string, row map[string]any) error {
	if len(row) == 0 {
		return nil
	}

	allowed, ok := tableColumns[table]
	if !ok {
		return fmt.Errorf("table %q is not in the backup allowlist", table)
	}

	cols := make([]string, 0, len(row))
	for col := range row {
		if !allowed[col] {
			return fmt.Errorf("column %q is not a known column of table %q — backup may be malformed or tampered", col, table)
		}
		cols = append(cols, col)
	}
	sort.Strings(cols)

	vals := make([]any, len(cols))
	placeholders := make([]string, len(cols))
	for i, col := range cols {
		vals[i] = row[col]
		placeholders[i] = "?"
	}

	query := fmt.Sprintf( // #nosec G201 -- table and column names come from a fixed internal schema, not user input
		"INSERT OR REPLACE INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)
	_, err := tx.ExecContext(ctx, query, vals...)
	return err
}

// MarshalPayload marshals the payload to JSON bytes.
func MarshalPayload(p *BackupPayload) ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// UnmarshalPayload parses JSON bytes into a BackupPayload.
func UnmarshalPayload(data []byte) (*BackupPayload, error) {
	var p BackupPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.Version == "" {
		return nil, fmt.Errorf("missing version field — may not be a valid PosternProxy backup")
	}
	return &p, nil
}

func replaceBackupTables(ctx context.Context, tx *sql.Tx, payload *BackupPayload) (restored int, err error) {
	for _, row := range payload.Tables["proxy_hosts"] {
		if err := prepareBackupProxyHostRow(row); err != nil {
			return 0, err
		}
	}
	// Clear existing data in reverse FK order.
	clearOrder := []string{
		"settings",
		"dead_hosts",
		"stream_hosts",
		"redirection_hosts",
		"proxy_hosts",
		"access_list_clients",
		"access_lists",
		"certificates",
	}
	for _, table := range clearOrder {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil { // #nosec G202 -- table name comes from the fixed internal clearOrder list, not user input
			return 0, fmt.Errorf("clear %s: %w", table, err)
		}
	}

	// Re-insert in FK-safe order.
	for _, table := range tableOrder {
		rows, ok := payload.Tables[table]
		if !ok || len(rows) == 0 {
			continue
		}
		for _, row := range rows {
			if err := insertRow(ctx, tx, table, row); err != nil {
				return 0, fmt.Errorf("insert into %s: %w", table, err)
			}
			restored++
		}
	}

	return restored, nil
}

func prepareBackupProxyHostRow(row map[string]any) error {
	_, present := row["ntlm_enabled"]
	enabled, err := decodeBackupZeroOne(row["ntlm_enabled"], present)
	if err != nil {
		return fmt.Errorf("invalid ntlm_enabled value")
	}
	if present {
		row["ntlm_enabled"] = int64(boolToInt(enabled))
	}
	if !enabled {
		return nil
	}
	_, tlsPresent := row["tls_passthrough"]
	tls, err := decodeBackupZeroOne(row["tls_passthrough"], tlsPresent)
	if err != nil {
		return fmt.Errorf("invalid tls_passthrough value")
	}
	advanced, err := backupString(row, "advanced_config")
	if err != nil {
		return err
	}
	locations, err := backupRaw(row, "locations")
	if err != nil {
		return err
	}
	extra, err := backupRaw(row, "extra_upstreams")
	if err != nil {
		return err
	}
	headers, err := backupRaw(row, "custom_headers")
	if err != nil {
		return err
	}
	pages, err := backupRaw(row, "error_pages")
	if err != nil {
		return err
	}
	return models.ValidateNTLMPassthrough(models.NTLMPassthrough{
		Enabled:        true,
		TLSPassthrough: tls,
		AdvancedConfig: advanced,
		Locations:      locations,
		ExtraUpstreams: extra,
		CustomHeaders:  headers,
		ErrorPages:     pages,
	})
}

func decodeBackupZeroOne(value any, present bool) (bool, error) {
	if !present {
		return false, nil
	}
	switch v := value.(type) {
	case bool:
		return v, nil
	case int64:
		if v == 0 || v == 1 {
			return v == 1, nil
		}
	case float64:
		if v == float64(int64(v)) && (v == 0 || v == 1) {
			return v == 1, nil
		}
	}
	return false, fmt.Errorf("invalid boolean value")
}

func backupString(row map[string]any, key string) (string, error) {
	value, ok := row[key]
	if !ok || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s", models.NTLMErrJSON)
	}
	return text, nil
}

func backupRaw(row map[string]any, key string) (json.RawMessage, error) {
	value, ok := row[key]
	if !ok || value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case string:
		return json.RawMessage(v), nil
	case []byte:
		return json.RawMessage(append([]byte(nil), v...)), nil
	case json.RawMessage:
		return v, nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("%s", models.NTLMErrJSON)
		}
		return encoded, nil
	}
}

func validateBackup(payload *BackupPayload) error {
	if payload == nil || payload.Version != backupVersion {
		return fmt.Errorf("unsupported backup version")
	}
	for table, rows := range payload.Tables {
		allowed, ok := tableColumns[table]
		if !ok {
			return fmt.Errorf("unsupported backup table %q", table)
		}
		seen := map[string]bool{}
		for _, row := range rows {
			if len(row) == 0 {
				return fmt.Errorf("empty backup row")
			}
			identifier := row["id"]
			if table == "settings" {
				identifier = row["key"]
			}
			if identifier != nil {
				key := fmt.Sprint(identifier)
				if seen[key] {
					return fmt.Errorf("duplicate backup identifier in %s", table)
				}
				seen[key] = true
			}
			for col := range row {
				if !allowed[col] {
					return fmt.Errorf("unsupported column %q in %s", col, table)
				}
			}
		}
	}
	return nil
}

func (s *BackupService) Audit(ctx context.Context, actorID *int64, action string, details map[string]any, ip string) {
	NewAuditService(s.db).Log(ctx, actorID, action, "backup", nil, details, ip)
}

// RestoreApplyError reports application failure after the data transaction committed.
type RestoreApplyError struct{ cause error }

func (e *RestoreApplyError) Error() string {
	return "Recovery data committed; configuration has not applied. Check server configuration status."
}
func (e *RestoreApplyError) Unwrap() error { return e.cause }
func (s *BackupService) applyRestored(ctx context.Context) error {
	if s.syncSvc != nil {
		if err := s.syncSvc.SyncDirty(ctx); err != nil {
			return &RestoreApplyError{cause: err}
		}
	}
	return nil
}
