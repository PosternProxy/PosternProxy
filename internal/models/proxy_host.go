package models

import "encoding/json"

// ProxyHost is a reverse-proxy virtual host that forwards incoming requests to
// an upstream service. It is the primary entity managed by the application.
type ProxyHost struct {
	ID             int64           `json:"id"`
	ServerID       int64           `json:"server_id"`
	DomainNames    json.RawMessage `json:"domain_names"`
	ForwardScheme  string          `json:"forward_scheme"`
	ForwardHost    string          `json:"forward_host"`
	ForwardPort    int             `json:"forward_port"`
	CertificateID  *int64          `json:"certificate_id"`
	SSLForced      bool            `json:"ssl_forced"`
	HSTSEnabled    bool            `json:"hsts_enabled"`
	HSTSSubdomains bool            `json:"hsts_subdomains"`
	AccessListID   *int64          `json:"access_list_id"`
	AdvancedConfig string          `json:"advanced_config"`
	Locations      json.RawMessage `json:"locations"`
	Enabled        bool            `json:"enabled"`
	CaddyID        string          `json:"caddy_id,omitempty"`

	// Health check configuration
	HCEnabled                 bool   `json:"hc_enabled"`
	HCActivePath              string `json:"hc_active_path"`
	HCActiveInterval          int    `json:"hc_active_interval"`
	HCActiveTimeout           int    `json:"hc_active_timeout"`
	HCActiveExpectStatus      int    `json:"hc_active_expect_status"`
	HCPassiveFailDuration     int    `json:"hc_passive_fail_duration"`
	HCPassiveMaxFails         int    `json:"hc_passive_max_fails"`
	HCPassiveUnhealthyLatency int    `json:"hc_passive_unhealthy_latency"`

	// Rate limiting (requires caddy-ratelimit plugin)
	RateLimitEnabled  bool   `json:"rate_limit_enabled"`
	RateLimitRequests int    `json:"rate_limit_requests"`
	RateLimitWindow   string `json:"rate_limit_window"`

	// TLS/SNI passthrough (requires caddy-l4 plugin)
	TLSPassthrough bool `json:"tls_passthrough"`

	// HTTPOnly disables TLS entirely — host is served over HTTP only.
	// Use for internal services that cannot satisfy Let's Encrypt HTTP-01.
	HTTPOnly bool `json:"http_only"`

	// NTLMEnabled selects the http_ntlm upstream transport. It is off when
	// the column is absent from older rows and snapshots.
	NTLMEnabled bool `json:"ntlm_enabled"`

	// Custom error pages: JSON map of status code string → HTML body.
	// e.g. {"404":"<html>…</html>","502":"<html>…</html>"}
	// Empty object {} means no custom pages configured.
	ErrorPages json.RawMessage `json:"error_pages"`

	// Custom header injection rules applied via Caddy's headers handler.
	// JSON array of CustomHeaderRule objects.
	CustomHeaders json.RawMessage `json:"custom_headers"`

	// Load balancing: additional upstream backends beyond the primary forward_host/port.
	// JSON array of ExtraUpstream objects. Empty = single-upstream mode.
	ExtraUpstreams json.RawMessage `json:"extra_upstreams"`
	// LBPolicy is the Caddy selection module name used when ≥2 upstreams are present.
	// Supported: random, round_robin, least_conn, ip_hash, uri_hash, first
	LBPolicy string `json:"lb_policy"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ExtraUpstream is an additional backend for load-balanced proxy hosts.
// The scheme/transport is shared with the primary upstream (forward_scheme).
type ExtraUpstream struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// CustomHeaderRule defines a single request or response header mutation rule.
type CustomHeaderRule struct {
	Direction string `json:"direction"` // "request" | "response"
	Operation string `json:"operation"` // "set" | "add" | "delete"
	Name      string `json:"name"`      // header name
	Value     string `json:"value"`     // header value (empty for delete)
}

// ProxyHostLocation is a path-specific upstream override within a ProxyHost.
// Requests whose path starts with Path are forwarded to this location's upstream
// instead of the host's default upstream.
type ProxyHostLocation struct {
	Path           string `json:"path"`
	ForwardScheme  string `json:"forward_scheme"`
	ForwardHost    string `json:"forward_host"`
	ForwardPort    int    `json:"forward_port"`
	AdvancedConfig string `json:"advanced_config"`
}

// CreateProxyHostInput carries all fields accepted when creating or updating a
// ProxyHost. It is reused for both create and update operations.
type CreateProxyHostInput struct {
	ServerID       int64           `json:"server_id"`
	DomainNames    json.RawMessage `json:"domain_names"`
	ForwardScheme  string          `json:"forward_scheme"`
	ForwardHost    string          `json:"forward_host"`
	ForwardPort    int             `json:"forward_port"`
	CertificateID  *int64          `json:"certificate_id"`
	SSLForced      bool            `json:"ssl_forced"`
	HSTSEnabled    bool            `json:"hsts_enabled"`
	HSTSSubdomains bool            `json:"hsts_subdomains"`
	AccessListID   *int64          `json:"access_list_id"`
	AdvancedConfig string          `json:"advanced_config"`
	Locations      json.RawMessage `json:"locations"`

	// Health check configuration
	HCEnabled                 bool   `json:"hc_enabled"`
	HCActivePath              string `json:"hc_active_path"`
	HCActiveInterval          int    `json:"hc_active_interval"`
	HCActiveTimeout           int    `json:"hc_active_timeout"`
	HCActiveExpectStatus      int    `json:"hc_active_expect_status"`
	HCPassiveFailDuration     int    `json:"hc_passive_fail_duration"`
	HCPassiveMaxFails         int    `json:"hc_passive_max_fails"`
	HCPassiveUnhealthyLatency int    `json:"hc_passive_unhealthy_latency"`

	// Rate limiting
	RateLimitEnabled  bool   `json:"rate_limit_enabled"`
	RateLimitRequests int    `json:"rate_limit_requests"`
	RateLimitWindow   string `json:"rate_limit_window"`

	// TLS/SNI passthrough
	TLSPassthrough bool `json:"tls_passthrough"`

	// HTTPOnly disables TLS — served over plain HTTP.
	HTTPOnly bool `json:"http_only"`

	// NTLMEnabled selects the http_ntlm upstream transport.
	NTLMEnabled bool `json:"ntlm_enabled"`

	// Custom error pages: JSON map of status code string → HTML body.
	ErrorPages json.RawMessage `json:"error_pages"`

	// Custom header injection rules.
	CustomHeaders json.RawMessage `json:"custom_headers"`

	// Load balancing
	ExtraUpstreams json.RawMessage `json:"extra_upstreams"`
	LBPolicy       string          `json:"lb_policy"`
}
