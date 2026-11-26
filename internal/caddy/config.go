package caddy

import (
	"encoding/json"
	"fmt"

	"github.com/posternproxy/posternproxy/internal/models"
)

// internalHTTPSAddr is the loopback address the HTTPS server listens on when a
// TLS-passthrough host has taken over :443 at Layer 4. caddy-l4 has no handler
// that serves the HTTP app in place, so the L4 fallthrough route proxies
// non-passthrough connections here (carrying PROXY protocol to preserve the
// client IP) where Caddy terminates TLS normally. Port 8443 is reserved for this
// bridge — the installer forbids it as a management port so the two cannot
// collide on a single host.
const internalHTTPSAddr = "127.0.0.1:8443"

// BuildFullConfig constructs the complete Caddy JSON configuration from all
// enabled entities. This is the DB → Caddy translation layer.
func BuildFullConfig(in *BuildInput) (*Config, error) {
	trusted, err := validateProxyRanges(in.TrustedProxies)
	if err != nil {
		return nil, err
	}
	httpRoutes := make([]*Route, 0)
	l4Servers := make(map[string]*L4Server)

	// Partition proxy hosts: passthrough (L4), HTTP-only (:80), or normal HTTPS
	var passthroughHosts []models.ProxyHost
	var httpOnlyHosts []models.ProxyHost
	var normalHosts []models.ProxyHost
	for _, h := range in.ProxyHosts {
		if !h.Enabled {
			continue
		}
		if h.TLSPassthrough {
			passthroughHosts = append(passthroughHosts, h)
		} else if h.HTTPOnly {
			httpOnlyHosts = append(httpOnlyHosts, h)
		} else {
			normalHosts = append(normalHosts, h)
		}
	}

	// --- Normal proxy hosts ---
	// A host is served over HTTPS (:443) when it forces SSL (Caddy provisions a
	// cert via ACME) or when a certificate is explicitly assigned. A host that
	// does NOT force SSL is also served directly over plain HTTP (:80) so it
	// works immediately without any certificate. SSL-forced hosts are not put on
	// :80 directly — the :80 redirect catch-all sends them to HTTPS.
	var http80Routes []*Route
	for i := range normalHosts {
		h := &normalHosts[i]
		var al *models.AccessList
		if h.AccessListID != nil {
			al = in.accessListByID(*h.AccessListID)
		}

		serveHTTPS := h.SSLForced || h.CertificateID != nil
		if serveHTTPS {
			route, err := BuildProxyHostRoute(h, al)
			if err != nil {
				return nil, fmt.Errorf("proxy host %d: %w", h.ID, err)
			}
			httpRoutes = append(httpRoutes, route)
		}

		if !h.SSLForced {
			route, err := BuildProxyHostRoute(h, al)
			if err != nil {
				return nil, fmt.Errorf("proxy host %d (http): %w", h.ID, err)
			}
			// When the same host is also on :443, drop the @id here so the two
			// routes don't collide (Caddy requires @id to be unique).
			if serveHTTPS {
				route.ID = ""
			}
			http80Routes = append(http80Routes, route)
		}
	}

	// --- HTTP-only proxy hosts → plain :80 routes (no TLS, no redirect) ---
	for i := range httpOnlyHosts {
		h := &httpOnlyHosts[i]
		var al *models.AccessList
		if h.AccessListID != nil {
			al = in.accessListByID(*h.AccessListID)
		}
		route, err := BuildProxyHostRoute(h, al)
		if err != nil {
			return nil, fmt.Errorf("http-only proxy host %d: %w", h.ID, err)
		}
		http80Routes = append(http80Routes, route)
	}

	// --- Redirection hosts ---
	// Same HTTP/HTTPS split as proxy hosts: served on :443 when SSL is forced or a
	// certificate is assigned, and directly on :80 when not forced so the redirect
	// works without a certificate. SSL-forced ones use the :80 redirect catch-all.
	for i := range in.RedirectionHosts {
		h := &in.RedirectionHosts[i]
		if !h.Enabled {
			continue
		}
		serveHTTPS := h.SSLForced || h.CertificateID != nil
		if serveHTTPS {
			route, err := BuildRedirectionHostRoute(h, in.optionalAccessList(h.AccessListID))
			if err != nil {
				return nil, fmt.Errorf("redirection host %d: %w", h.ID, err)
			}
			httpRoutes = append(httpRoutes, route)
		}
		if !h.SSLForced {
			route, err := BuildRedirectionHostRoute(h, in.optionalAccessList(h.AccessListID))
			if err != nil {
				return nil, fmt.Errorf("redirection host %d (http): %w", h.ID, err)
			}
			if serveHTTPS {
				route.ID = ""
			}
			http80Routes = append(http80Routes, route)
		}
	}

	// --- Dead hosts ---
	// A parked-domain response should answer over plain HTTP too, so serve dead
	// hosts on both the :443 and :80 servers.
	for i := range in.DeadHosts {
		h := &in.DeadHosts[i]
		if !h.Enabled {
			continue
		}
		if !h.HTTPOnly {
			route, err := BuildDeadHostRoute(h, in.ControllerVersion, in.optionalAccessList(h.AccessListID))
			if err != nil {
				return nil, fmt.Errorf("dead host %d: %w", h.ID, err)
			}
			httpRoutes = append(httpRoutes, route)
		}

		httpRoute, err := BuildDeadHostRoute(h, in.ControllerVersion, in.optionalAccessList(h.AccessListID))
		if err != nil {
			return nil, fmt.Errorf("dead host %d (http): %w", h.ID, err)
		}
		if !h.HTTPOnly {
			httpRoute.ID = ""
		}
		http80Routes = append(http80Routes, httpRoute)
	}

	// --- Stream hosts (Layer 4) ---
	for i := range in.StreamHosts {
		h := in.StreamHosts[i]
		if !h.Enabled {
			continue
		}
		for key, server := range BuildStreamServers(&h) {
			l4Servers[key] = server
		}
	}

	// --- TLS passthrough hosts → L4 server on :443 ---
	// When ANY passthrough host exists we take over port 443 at the L4 layer.
	// Non-passthrough traffic falls through to tls+http handlers which route
	// into the Caddy HTTP app without an explicit :443 listener on that app.
	hasTLSPassthrough := len(passthroughHosts) > 0
	if hasTLSPassthrough {
		l4Routes := make([]*L4Route, 0, len(passthroughHosts)+1)

		// One L4 route per passthrough host (SNI-matched raw TCP proxy).
		for i := range passthroughHosts {
			r, err := BuildPassthroughL4Route(&passthroughHosts[i])
			if err != nil {
				return nil, fmt.Errorf("tls passthrough host %d: %w", passthroughHosts[i].ID, err)
			}
			l4Routes = append(l4Routes, r)
		}

		// Fallthrough route: proxy all non-passthrough SNI traffic to the internal
		// HTTPS listener, which terminates TLS and serves the HTTP app. PROXY
		// protocol v2 carries the real client IP across the loopback hop.
		// (caddy-l4 has no "http" handler, so terminating in place is impossible.)
		l4Routes = append(l4Routes, &L4Route{
			Handle: []L4Handler{{
				Handler:       "proxy",
				ProxyProtocol: "v2",
				Upstreams:     []L4Upstream{{Dial: []string{internalHTTPSAddr}}},
			}},
		})

		l4Servers["tls_passthrough_443"] = &L4Server{
			Listen: []string{"0.0.0.0:443"},
			Routes: l4Routes,
		}
	}

	// --- Build TLS app ---
	tlsApp, err := BuildTLSApp(in.Certificates, in.AgentCertDir)
	if err != nil {
		return nil, fmt.Errorf("build tls app: %w", err)
	}

	// --- Assemble apps ---
	apps := make(map[string]json.RawMessage)

	// HTTP app listen strategy:
	// • No passthrough: HTTPS server explicitly listens on :443.
	// • Passthrough: L4 owns :443; HTTPS server receives connections from
	//   the L4 "http" handler and should NOT have an explicit :443 listener
	//   (the L4 internal bridge delivers connections to the HTTP app).
	httpsListen := []string{":443"}
	if hasTLSPassthrough {
		// L4 owns :443 and proxies non-passthrough traffic here over loopback.
		httpsListen = []string{internalHTTPSAddr}
	}

	// Build per-host custom error pages → server-level error routes.
	// All proxy hosts (passthrough and normal) are eligible.
	errorRoutes := BuildErrorRoutes(in.ProxyHosts, in.ControllerVersion)
	var serverErrors *ServerErrors
	if len(errorRoutes) > 0 {
		serverErrors = &ServerErrors{Routes: errorRoutes}
	}

	httpApp := &HTTPApp{
		Servers: map[string]*HTTPServer{
			"https": {
				Listen: httpsListen,
				Routes: httpRoutes,
				Errors: serverErrors,
				// We own the :80 server (direct-serve + redirect catch-all), so
				// keep Caddy from adding its own competing HTTP→HTTPS redirects.
				AutomaticHTTPS: &AutoHTTPS{DisableRedirects: true},
			},
			"http": {
				Listen: []string{":80"},
				Routes: buildHTTP80Routes(http80Routes),
				Errors: serverErrors,
			},
		},
	}

	for _, server := range httpApp.Servers {
		server.ReadHeaderTimeout = "10s"
		server.IdleTimeout = "2m"
		server.MaxHeaderBytes = 1 << 20
		if len(trusted) > 0 {
			server.TrustedProxies = &TrustedProxySource{Source: "static", Ranges: trusted}
			server.TrustedProxiesStrict = 1
			server.ClientIPHeaders = []string{"X-Forwarded-For"}
		}
	}

	if hasTLSPassthrough {
		// The L4 bridge hands non-passthrough connections to the loopback HTTPS
		// listener with a PROXY header; read it (only from loopback) to recover
		// the client IP, then terminate TLS. The tls wrapper must follow
		// proxy_protocol in the chain.
		httpApp.Servers["https"].ListenerWrappers = []ListenerWrapper{
			{Wrapper: "proxy_protocol", Allow: []string{"127.0.0.1/32"}},
			{Wrapper: "tls"},
		}
	}

	// HTTP/2 is a server-wide setting; restrict the HTTPS listener to HTTP/1.1
	// when disabled for this server. In passthrough mode the HTTPS server sits
	// behind a TCP-only L4 bridge, so HTTP/3 (QUIC/UDP) cannot reach it and would
	// otherwise advertise the private bridge port as its endpoint — offer h1/h2.
	switch {
	case in.HTTP2Disabled:
		httpApp.Servers["https"].Protocols = []string{"h1"}
	case hasTLSPassthrough:
		httpApp.Servers["https"].Protocols = []string{"h1", "h2"}
	}

	httpRaw, err := json.Marshal(httpApp)
	if err != nil {
		return nil, fmt.Errorf("marshal http app: %w", err)
	}
	apps["http"] = httpRaw

	// TLS app
	tlsRaw, err := json.Marshal(tlsApp)
	if err != nil {
		return nil, fmt.Errorf("marshal tls app: %w", err)
	}
	apps["tls"] = tlsRaw

	// Layer4 app (stream hosts and/or TLS passthrough)
	if len(l4Servers) > 0 {
		l4App := &Layer4App{Servers: l4Servers}
		l4Raw, err := json.Marshal(l4App)
		if err != nil {
			return nil, fmt.Errorf("marshal layer4 app: %w", err)
		}
		apps["layer4"] = l4Raw
	}

	return &Config{
		Admin: &AdminConfig{Listen: "127.0.0.1:2019"},
		Apps:  apps,
	}, nil
}

// BuildInput holds all entities needed to generate the full Caddy config.
type BuildInput struct {
	TrustedProxies   []string
	ProxyHosts       []models.ProxyHost
	RedirectionHosts []models.RedirectionHost
	StreamHosts      []models.StreamHost
	DeadHosts        []models.DeadHost
	Certificates     []*models.Certificate
	AccessLists      []models.AccessList
	// AgentCertDir is set for remote servers. When non-empty, BuildTLSApp uses
	// predictable agent-side paths instead of the controller's local file paths.
	AgentCertDir string
	// HTTP2Disabled, when true, restricts the HTTPS server to HTTP/1.1. Inverted
	// (zero value = enabled) so callers that don't set it keep the Caddy default.
	HTTP2Disabled bool
	// ControllerVersion is rendered into generated operator-facing pages.
	ControllerVersion string
}

// buildHTTP80Routes returns the route list for the plain-HTTP (:80) server.
// HTTP-only proxy host routes come first (terminal, host-matched) so they are
// served directly without being redirected to HTTPS. The final catch-all route
// performs the 301 redirect for everything else.
func buildHTTP80Routes(httpOnlyRoutes []*Route) []*Route {
	redirectRoute := &Route{
		Handle: []Handler{{
			Handler:    "static_response",
			StatusCode: 301,
			StaticHeaders: map[string][]string{
				"Location": {"https://{http.request.host}{http.request.uri}"},
			},
		}},
	}
	routes := make([]*Route, 0, len(httpOnlyRoutes)+1)
	routes = append(routes, httpOnlyRoutes...)
	routes = append(routes, redirectRoute)
	return routes
}

func (b *BuildInput) accessListByID(id int64) *models.AccessList {
	for i := range b.AccessLists {
		if b.AccessLists[i].ID == id {
			return &b.AccessLists[i]
		}
	}
	return nil
}

func (b *BuildInput) optionalAccessList(id *int64) *models.AccessList {
	if id == nil {
		return nil
	}
	return b.accessListByID(*id)
}
