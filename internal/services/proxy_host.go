package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/rs/zerolog/log"
)

// proxyHostColumns is the canonical SELECT column list for proxy_hosts.
// Keep in sync with scanProxyHostRow.
const proxyHostColumns = `
	id, server_id, domain_names, forward_scheme, forward_host, forward_port,
	certificate_id, ssl_forced, hsts_enabled, hsts_subdomains,
	access_list_id, advanced_config,
	locations, enabled, caddy_id,
	hc_enabled, hc_active_path, hc_active_interval, hc_active_timeout,
	hc_active_expect_status, hc_passive_fail_duration, hc_passive_max_fails,
	hc_passive_unhealthy_latency,
	rate_limit_enabled, rate_limit_requests, rate_limit_window,
	tls_passthrough, http_only,
	error_pages,
	custom_headers,
	extra_upstreams, lb_policy,
	created_at, updated_at
`

// ProxyHostService manages reverse-proxy virtual hosts in SQLite and Caddy.
type ProxyHostService struct {
	db      *sql.DB
	caddy   *caddy.Client
	sync    *CaddySyncService
	audit   *AuditService
	history *ConfigHistoryService
}

// NewProxyHostService creates a ProxyHostService backed by db.
func NewProxyHostService(db *sql.DB, caddyClient *caddy.Client, audit *AuditService, history *ConfigHistoryService, sync *CaddySyncService) *ProxyHostService {
	return &ProxyHostService{db: db, caddy: caddyClient, sync: sync, audit: audit, history: history}
}

// List returns all proxy hosts ordered by ID.
func (s *ProxyHostService) List(ctx context.Context) ([]models.ProxyHost, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT`+proxyHostColumns+`FROM proxy_hosts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProxyHosts(rows)
}

// ListByServer returns proxy hosts for a specific server, ordered by ID.
func (s *ProxyHostService) ListByServer(ctx context.Context, serverID int64) ([]models.ProxyHost, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT`+proxyHostColumns+`FROM proxy_hosts WHERE server_id = ? ORDER BY id`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProxyHosts(rows)
}

// GetByID returns the proxy host with the given ID, or nil if not found.
func (s *ProxyHostService) GetByID(ctx context.Context, id int64) (*models.ProxyHost, error) {
	row := s.db.QueryRowContext(ctx, `SELECT`+proxyHostColumns+`FROM proxy_hosts WHERE id = ?`, id)
	h, err := scanProxyHost(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return h, err
}

func (s *ProxyHostService) Create(ctx context.Context, callerID *int64, input models.CreateProxyHostInput, ip string) (*models.ProxyHost, error) {
	if err := validateProxyHostInput(input.DomainNames, input.ForwardHost, input.ForwardPort); err != nil {
		return nil, err
	}
	if err := validateAdvancedConfig(input.AdvancedConfig); err != nil {
		return nil, err
	}
	if input.ServerID == 0 {
		input.ServerID = 1
	}
	if string(input.Locations) == "" {
		input.Locations = json.RawMessage("[]")
	}
	if input.RateLimitWindow == "" {
		input.RateLimitWindow = "1m"
	}
	if len(input.ErrorPages) == 0 {
		input.ErrorPages = json.RawMessage("{}")
	}
	if len(input.CustomHeaders) == 0 {
		input.CustomHeaders = json.RawMessage("[]")
	}
	if len(input.ExtraUpstreams) == 0 {
		input.ExtraUpstreams = json.RawMessage("[]")
	}
	if input.LBPolicy == "" {
		input.LBPolicy = "random"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO proxy_hosts
		  (server_id, domain_names, forward_scheme, forward_host, forward_port,
		   certificate_id, ssl_forced, hsts_enabled, hsts_subdomains,
		   access_list_id, advanced_config, locations,
		   hc_enabled, hc_active_path, hc_active_interval, hc_active_timeout,
		   hc_active_expect_status, hc_passive_fail_duration, hc_passive_max_fails,
		   hc_passive_unhealthy_latency,
		   rate_limit_enabled, rate_limit_requests, rate_limit_window,
		   tls_passthrough, http_only, error_pages, custom_headers,
		   extra_upstreams, lb_policy)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		input.ServerID, string(input.DomainNames), input.ForwardScheme,
		input.ForwardHost, input.ForwardPort, input.CertificateID,
		boolToInt(input.SSLForced), boolToInt(input.HSTSEnabled),
		boolToInt(input.HSTSSubdomains),
		input.AccessListID, input.AdvancedConfig, string(input.Locations),
		boolToInt(input.HCEnabled), input.HCActivePath, nullableInt(input.HCActiveInterval),
		nullableInt(input.HCActiveTimeout), nullableInt(input.HCActiveExpectStatus),
		nullableInt(input.HCPassiveFailDuration), nullableInt(input.HCPassiveMaxFails),
		nullableInt(input.HCPassiveUnhealthyLatency),
		boolToInt(input.RateLimitEnabled), input.RateLimitRequests, input.RateLimitWindow,
		boolToInt(input.TLSPassthrough), boolToInt(input.HTTPOnly), string(input.ErrorPages), string(input.CustomHeaders),
		string(input.ExtraUpstreams), input.LBPolicy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert proxy host: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	caddyID := fmt.Sprintf("proxy_host_%d", id)
	if _, err := tx.ExecContext(ctx, "UPDATE proxy_hosts SET caddy_id = ? WHERE id = ?", caddyID, id); err != nil {
		return nil, fmt.Errorf("set caddy_id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	host, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Push to Caddy — passthrough hosts trigger a full sync (L4 config restructure).

	// Snapshot the newly created host so rollback can restore it.
	if s.history != nil {
		if err := s.history.Snapshot(ctx, "proxy_host", id, "create", host, callerID); err != nil {
			log.Warn().Err(err).Int64("proxy_host_id", id).Msg("failed to snapshot proxy host after create")
		}
	}
	s.audit.HostChange(ctx, callerID, "create", "proxy_host", id, nil, host, ip)
	return host, syncServers(ctx, s.sync, host.ServerID)
}

func (s *ProxyHostService) Update(ctx context.Context, callerID *int64, id int64, input models.CreateProxyHostInput, ip string) (*models.ProxyHost, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("proxy host not found")
	}
	// Snapshot BEFORE mutation so rollback restores the prior state.
	if s.history != nil {
		if err := s.history.Snapshot(ctx, "proxy_host", id, "update", existing, callerID); err != nil {
			log.Warn().Err(err).Int64("proxy_host_id", id).Msg("failed to snapshot proxy host before update")
		}
	}
	if err := validateProxyHostInput(input.DomainNames, input.ForwardHost, input.ForwardPort); err != nil {
		return nil, err
	}
	if err := validateAdvancedConfig(input.AdvancedConfig); err != nil {
		return nil, err
	}
	if string(input.Locations) == "" {
		input.Locations = json.RawMessage("[]")
	}
	if input.RateLimitWindow == "" {
		input.RateLimitWindow = "1m"
	}
	if len(input.ErrorPages) == 0 {
		input.ErrorPages = json.RawMessage("{}")
	}
	if len(input.CustomHeaders) == 0 {
		input.CustomHeaders = json.RawMessage("[]")
	}
	if len(input.ExtraUpstreams) == 0 {
		input.ExtraUpstreams = json.RawMessage("[]")
	}
	if input.LBPolicy == "" {
		input.LBPolicy = "random"
	}

	if input.ServerID == 0 {
		input.ServerID = existing.ServerID
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE proxy_hosts SET
		  server_id=?,
		  domain_names=?, forward_scheme=?, forward_host=?, forward_port=?,
		  certificate_id=?, ssl_forced=?, hsts_enabled=?, hsts_subdomains=?,
		  access_list_id=?, advanced_config=?, locations=?,
		  hc_enabled=?, hc_active_path=?, hc_active_interval=?, hc_active_timeout=?,
		  hc_active_expect_status=?, hc_passive_fail_duration=?, hc_passive_max_fails=?,
		  hc_passive_unhealthy_latency=?,
		  rate_limit_enabled=?, rate_limit_requests=?, rate_limit_window=?,
		  tls_passthrough=?, http_only=?, error_pages=?, custom_headers=?,
		  extra_upstreams=?, lb_policy=?,
		  updated_at=datetime('now')
		WHERE id=?`,
		input.ServerID, string(input.DomainNames), input.ForwardScheme, input.ForwardHost, input.ForwardPort,
		input.CertificateID, boolToInt(input.SSLForced), boolToInt(input.HSTSEnabled),
		boolToInt(input.HSTSSubdomains),
		input.AccessListID, input.AdvancedConfig, string(input.Locations),
		boolToInt(input.HCEnabled), input.HCActivePath, nullableInt(input.HCActiveInterval),
		nullableInt(input.HCActiveTimeout), nullableInt(input.HCActiveExpectStatus),
		nullableInt(input.HCPassiveFailDuration), nullableInt(input.HCPassiveMaxFails),
		nullableInt(input.HCPassiveUnhealthyLatency),
		boolToInt(input.RateLimitEnabled), input.RateLimitRequests, input.RateLimitWindow,
		boolToInt(input.TLSPassthrough), boolToInt(input.HTTPOnly), string(input.ErrorPages), string(input.CustomHeaders),
		string(input.ExtraUpstreams), input.LBPolicy,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("update proxy host: %w", err)
	}

	host, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.audit.HostChange(ctx, callerID, "update", "proxy_host", id, existing, host, ip)
	return host, syncServers(ctx, s.sync, existing.ServerID, host.ServerID)
}

func (s *ProxyHostService) Delete(ctx context.Context, callerID *int64, id int64, ip string) error {
	host, err := s.GetByID(ctx, id)
	if err != nil || host == nil {
		return fmt.Errorf("proxy host not found")
	}
	// Snapshot BEFORE deletion — this is the last known state for potential restore.
	if s.history != nil {
		if err := s.history.Snapshot(ctx, "proxy_host", id, "delete", host, callerID); err != nil {
			log.Warn().Err(err).Int64("proxy_host_id", id).Msg("failed to snapshot proxy host before delete")
		}
	}

	if _, err := s.db.ExecContext(ctx, "DELETE FROM proxy_hosts WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete proxy host: %w", err)
	}

	s.audit.HostChange(ctx, callerID, "delete", "proxy_host", id, host, nil, ip)
	return syncServers(ctx, s.sync, host.ServerID)
}

func (s *ProxyHostService) SetEnabled(ctx context.Context, callerID *int64, id int64, enabled bool, ip string) error {
	before, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if before == nil {
		return fmt.Errorf("host not found")
	}

	if _, err := s.db.ExecContext(ctx,
		"UPDATE proxy_hosts SET enabled=?, updated_at=datetime('now') WHERE id=?",
		boolToInt(enabled), id,
	); err != nil {
		return fmt.Errorf("update enabled: %w", err)
	}

	host, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if host == nil {
		return fmt.Errorf("proxy host not found")
	}

	action := "enable"
	if !enabled {
		action = "disable"
	}
	s.audit.HostChange(ctx, callerID, action, "proxy_host", id, before, host, ip)
	return syncServers(ctx, s.sync, host.ServerID)
}

// --- scan helpers ---

func scanProxyHosts(rows *sql.Rows) ([]models.ProxyHost, error) {
	var hosts []models.ProxyHost
	for rows.Next() {
		h, err := scanProxyHostRow(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, *h)
	}
	return hosts, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProxyHost(row *sql.Row) (*models.ProxyHost, error) {
	return scanProxyHostRow(row)
}

func scanProxyHostRow(row scanner) (*models.ProxyHost, error) {
	var h models.ProxyHost
	var (
		sslForced, hstsEnabled, hstsSubdomains, hcEnabled int
		rateLimitEnabled, tlsPassthrough, httpOnly        int
	)
	var (
		domainNames, locations, errorPages, customHeaders, extraUpstreams, lbPolicy string
		hcActivePath                                                                sql.NullString
		hcActiveInterval, hcActiveTimeout, hcActiveExpectStatus                     sql.NullInt64
		hcPassiveFailDuration, hcPassiveMaxFails, hcPassiveUnhealthyLatency         sql.NullInt64
	)
	err := row.Scan(
		&h.ID, &h.ServerID, &domainNames, &h.ForwardScheme, &h.ForwardHost, &h.ForwardPort,
		&h.CertificateID, &sslForced, &hstsEnabled, &hstsSubdomains,
		&h.AccessListID, &h.AdvancedConfig,
		&locations, &h.Enabled, &h.CaddyID,
		&hcEnabled, &hcActivePath, &hcActiveInterval, &hcActiveTimeout,
		&hcActiveExpectStatus, &hcPassiveFailDuration, &hcPassiveMaxFails,
		&hcPassiveUnhealthyLatency,
		&rateLimitEnabled, &h.RateLimitRequests, &h.RateLimitWindow,
		&tlsPassthrough, &httpOnly,
		&errorPages,
		&customHeaders,
		&extraUpstreams, &lbPolicy,
		&h.CreatedAt, &h.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	h.DomainNames = json.RawMessage(domainNames)
	h.Locations = json.RawMessage(locations)
	if errorPages == "" || errorPages == "null" {
		h.ErrorPages = json.RawMessage("{}")
	} else {
		h.ErrorPages = json.RawMessage(errorPages)
	}
	if customHeaders == "" || customHeaders == "null" {
		h.CustomHeaders = json.RawMessage("[]")
	} else {
		h.CustomHeaders = json.RawMessage(customHeaders)
	}
	if extraUpstreams == "" || extraUpstreams == "null" {
		h.ExtraUpstreams = json.RawMessage("[]")
	} else {
		h.ExtraUpstreams = json.RawMessage(extraUpstreams)
	}
	if lbPolicy == "" {
		h.LBPolicy = "random"
	} else {
		h.LBPolicy = lbPolicy
	}
	h.SSLForced = sslForced == 1
	h.HSTSEnabled = hstsEnabled == 1
	h.HSTSSubdomains = hstsSubdomains == 1
	h.HCEnabled = hcEnabled == 1
	h.RateLimitEnabled = rateLimitEnabled == 1
	h.TLSPassthrough = tlsPassthrough == 1
	h.HTTPOnly = httpOnly == 1
	if hcActivePath.Valid {
		h.HCActivePath = hcActivePath.String
	}
	if hcActiveInterval.Valid {
		h.HCActiveInterval = int(hcActiveInterval.Int64)
	}
	if hcActiveTimeout.Valid {
		h.HCActiveTimeout = int(hcActiveTimeout.Int64)
	}
	if hcActiveExpectStatus.Valid {
		h.HCActiveExpectStatus = int(hcActiveExpectStatus.Int64)
	}
	if hcPassiveFailDuration.Valid {
		h.HCPassiveFailDuration = int(hcPassiveFailDuration.Int64)
	}
	if hcPassiveMaxFails.Valid {
		h.HCPassiveMaxFails = int(hcPassiveMaxFails.Int64)
	}
	if hcPassiveUnhealthyLatency.Valid {
		h.HCPassiveUnhealthyLatency = int(hcPassiveUnhealthyLatency.Int64)
	}
	return &h, nil
}

// validateAdvancedConfig ensures the optional advanced-config snippet is either
// empty or a JSON object (it is shallow-merged into the route at build time).
func validateAdvancedConfig(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return fmt.Errorf("advanced config must be a JSON object")
	}
	return nil
}

func validateProxyHostInput(domainNames json.RawMessage, host string, port int) error {
	var domains []string
	if err := json.Unmarshal(domainNames, &domains); err != nil || len(domains) == 0 {
		return fmt.Errorf("at least one domain name is required")
	}
	if host == "" {
		return fmt.Errorf("forward host is required")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("forward port must be between 1 and 65535")
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nullableInt returns nil when v == 0 (stores NULL in SQLite).
func nullableInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}
