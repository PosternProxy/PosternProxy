package caddy

import "encoding/json"

// Config is the top-level Caddy JSON config structure.
type Config struct {
	Admin *AdminConfig               `json:"admin,omitempty"`
	Apps  map[string]json.RawMessage `json:"apps,omitempty"`
}

type AdminConfig struct {
	Listen string `json:"listen"`
}

// HTTPApp is apps.http
type HTTPApp struct {
	Servers map[string]*HTTPServer `json:"servers"`
}

// HTTPServer is a single HTTP server.
type HTTPServer struct {
	Listen         []string      `json:"listen"`
	Routes         []*Route      `json:"routes,omitempty"`
	Errors         *ServerErrors `json:"errors,omitempty"`
	AutomaticHTTPS *AutoHTTPS    `json:"automatic_https,omitempty"`
}

// ServerErrors holds custom error-handling routes for an HTTP server.
// These routes are matched after normal routing fails or when a handler
// returns an error (e.g. 502 from reverse_proxy, or 404 from no route match).
type ServerErrors struct {
	Routes []*Route `json:"routes,omitempty"`
}

type AutoHTTPS struct {
	Disable bool `json:"disable,omitempty"`
}

// Route is a single route entry.
type Route struct {
	ID       string    `json:"@id,omitempty"`
	Match    []Matcher `json:"match,omitempty"`
	Handle   []Handler `json:"handle"`
	Terminal bool      `json:"terminal,omitempty"`
}

// Matcher conditions. Fields in a single Matcher are AND'd together;
// multiple Matcher objects in a match array are OR'd.
type Matcher struct {
	Host     []string         `json:"host,omitempty"`
	Path     []string         `json:"path,omitempty"`
	Not      []Matcher        `json:"not,omitempty"`
	RemoteIP *RemoteIPMatcher `json:"remote_ip,omitempty"`
	// Error matches on HTTP error status codes (used in server.errors routes).
	Error *ErrorMatcher `json:"error,omitempty"`
}

type RemoteIPMatcher struct {
	Ranges []string `json:"ranges"`
}

// ErrorMatcher matches on one or more HTTP error status codes.
// Used in apps.http.servers.<name>.errors.routes to dispatch custom error pages.
type ErrorMatcher struct {
	StatusCodes []int `json:"status_codes,omitempty"`
}

// Handler wraps all handler types. Only one field should be set.
type Handler struct {
	// Common
	Handler string `json:"handler"`

	// subroute
	Routes []*Route `json:"routes,omitempty"`

	// reverse_proxy
	Upstreams    []*Upstream   `json:"upstreams,omitempty"`
	Transport    *Transport    `json:"transport,omitempty"`
	Headers      *HeaderOps    `json:"headers,omitempty"`
	HealthChecks *HealthChecks `json:"health_checks,omitempty"`

	// headers (standalone handler — distinct from reverse_proxy's nested "headers" key)
	// These fields appear at the top level of the handler object when handler == "headers".
	Request  *HeaderMutation         `json:"request,omitempty"`
	Response *ResponseHeaderMutation `json:"response,omitempty"`

	// reverse_proxy load balancing
	LoadBalancing *LoadBalancing `json:"load_balancing,omitempty"`

	// rate_limit (github.com/mholt/caddy-ratelimit)
	RateLimits map[string]*RateLimitZone `json:"rate_limits,omitempty"`

	// static_response
	StatusCode    int                 `json:"status_code,omitempty"`
	StaticHeaders map[string][]string `json:"static_headers,omitempty"`
	Body          string              `json:"body,omitempty"`

	// authentication
	Providers map[string]*AuthProvider `json:"providers,omitempty"`

	// file_server
	Root string `json:"root,omitempty"`
}

// RateLimitZone configures a caddy-ratelimit zone.
// Zone name is the map key in Handler.RateLimits.
type RateLimitZone struct {
	Key       string `json:"key"`        // e.g. "{http.request.remote.host}"
	Window    string `json:"window"`     // Go duration: "1m", "30s"
	MaxEvents int    `json:"max_events"` // max requests per window
}

type Upstream struct {
	Dial string `json:"dial"`
}

// LoadBalancing configures how Caddy distributes requests across upstreams.
type LoadBalancing struct {
	SelectionPolicy *SelectionPolicy `json:"selection_policy,omitempty"`
	// Retries is the number of times to try another upstream on failure.
	// Defaults to the number of upstreams minus one when not set.
	Retries int `json:"retries,omitempty"`
}

// SelectionPolicy names the Caddy upstream-selection module.
// Simple policies (no extra fields): random, round_robin, least_conn,
// ip_hash, uri_hash, first.
type SelectionPolicy struct {
	Policy string `json:"policy"`
}

type Transport struct {
	Protocol string      `json:"protocol"`
	TLS      *TLSOptions `json:"tls,omitempty"`
}

type TLSOptions struct {
	InsecureSkipVerify bool `json:"insecure_skip_verify,omitempty"`
}

// HealthChecks configures active and/or passive health checking for a reverse_proxy handler.
type HealthChecks struct {
	Active  *ActiveHealthCheck  `json:"active,omitempty"`
	Passive *PassiveHealthCheck `json:"passive,omitempty"`
}

// ActiveHealthCheck polls upstreams on an interval.
type ActiveHealthCheck struct {
	URI          string `json:"uri,omitempty"`           // path to poll, e.g. "/health"
	Interval     string `json:"interval,omitempty"`      // Go duration, e.g. "30s"
	Timeout      string `json:"timeout,omitempty"`       // Go duration, e.g. "5s"
	ExpectStatus int    `json:"expect_status,omitempty"` // HTTP status to consider healthy, e.g. 200
}

// PassiveHealthCheck marks upstreams unhealthy based on observed responses.
type PassiveHealthCheck struct {
	FailDuration     string `json:"fail_duration,omitempty"`     // how long to mark unhealthy, e.g. "30s"
	MaxFails         int    `json:"max_fails,omitempty"`         // fails before marking down
	UnhealthyStatus  []int  `json:"unhealthy_status,omitempty"`  // status codes considered failures
	UnhealthyLatency string `json:"unhealthy_latency,omitempty"` // latency threshold, e.g. "500ms"
}

type HeaderOps struct {
	Request  *HeaderMutation `json:"request,omitempty"`
	Response *HeaderMutation `json:"response,omitempty"`
}

type HeaderMutation struct {
	Set    map[string][]string `json:"set,omitempty"`
	Add    map[string][]string `json:"add,omitempty"`
	Delete []string            `json:"delete,omitempty"`
}

// ResponseHeaderMutation is like HeaderMutation but includes the Deferred flag.
// When Deferred is true Caddy applies the mutations at response-write time,
// which is required for a standalone "headers" handler placed before a subroute.
type ResponseHeaderMutation struct {
	Set      map[string][]string `json:"set,omitempty"`
	Add      map[string][]string `json:"add,omitempty"`
	Delete   []string            `json:"delete,omitempty"`
	Deferred bool                `json:"deferred,omitempty"`
}

type AuthProvider struct {
	Accounts []BasicAuthAccount `json:"accounts,omitempty"`
}

type BasicAuthAccount struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// TLSApp is apps.tls
type TLSApp struct {
	Certificates *TLSCertificates `json:"certificates,omitempty"`
}

type TLSCertificates struct {
	LoadFiles []CertFileLoader `json:"load_files,omitempty"`
}

type CertFileLoader struct {
	Certificate string   `json:"certificate"`
	Key         string   `json:"key"`
	Tags        []string `json:"tags,omitempty"`
}

// Layer4App is apps.layer4
type Layer4App struct {
	Servers map[string]*L4Server `json:"servers"`
}

type L4Server struct {
	Listen []string   `json:"listen"`
	Routes []*L4Route `json:"routes,omitempty"`
}

type L4Route struct {
	Match  []L4Matcher `json:"match,omitempty"`
	Handle []L4Handler `json:"handle"`
}

type L4Matcher struct {
	TLS *L4TLSMatcher `json:"tls,omitempty"`
}

type L4TLSMatcher struct {
	SNI []string `json:"sni,omitempty"`
}

// L4Handler covers all Layer 4 handler types:
//
//	handler: "proxy"  — TCP/UDP forward (Upstreams required)
//	handler: "tls"    — terminate TLS, pass decrypted bytes to next handler
//	handler: "http"   — route connection to Caddy HTTP app
type L4Handler struct {
	Handler   string       `json:"handler"`
	Upstreams []L4Upstream `json:"upstreams,omitempty"`
}

type L4Upstream struct {
	Dial []string `json:"dial"`
}
