package caddy

import (
	"encoding/json"
	"fmt"

	"github.com/posternproxy/posternproxy/internal/models"
)

// BuildFullConfig constructs the complete Caddy JSON configuration from all
// enabled entities. This is the DB → Caddy translation layer.
func BuildFullConfig(in *BuildInput) (*Config, error) {
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

	// --- Normal proxy hosts → HTTPS routes ---
	for i := range normalHosts {
		h := &normalHosts[i]
		var al *models.AccessList
		if h.AccessListID != nil {
			al = in.accessListByID(*h.AccessListID)
		}
		route, err := BuildProxyHostRoute(h, al)
		if err != nil {
			return nil, fmt.Errorf("proxy host %d: %w", h.ID, err)
		}
		httpRoutes = append(httpRoutes, route)
	}

	// --- HTTP-only proxy hosts → plain :80 routes (no TLS, no redirect) ---
	var http80Routes []*Route
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
	for _, h := range in.RedirectionHosts {
		if !h.Enabled {
			continue
		}
		route, err := BuildRedirectionHostRoute(&h)
		if err != nil {
			return nil, fmt.Errorf("redirection host %d: %w", h.ID, err)
		}
		httpRoutes = append(httpRoutes, route)
	}

	// --- Dead hosts ---
	for _, h := range in.DeadHosts {
		if !h.Enabled {
			continue
		}
		route, err := BuildDeadHostRoute(&h)
		if err != nil {
			return nil, fmt.Errorf("dead host %d: %w", h.ID, err)
		}
		httpRoutes = append(httpRoutes, route)
	}

	// --- Stream hosts (Layer 4) ---
	for _, h := range in.StreamHosts {
		if !h.Enabled {
			continue
		}
		key := StreamServerKey(&h)
		l4Servers[key] = BuildStreamServer(&h)
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

		// Fallthrough route: terminate TLS and route to the Caddy HTTP app
		// (handles all non-passthrough SNI traffic on port 443).
		l4Routes = append(l4Routes, &L4Route{
			Handle: []L4Handler{
				{Handler: "tls"},
				{Handler: "http"},
			},
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
		httpsListen = []string{} // L4 feeds the HTTP app directly
	}

	// Build per-host custom error pages → server-level error routes.
	// All proxy hosts (passthrough and normal) are eligible.
	errorRoutes := BuildErrorRoutes(in.ProxyHosts)
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
			},
			"http": {
				Listen: []string{":80"},
				Routes: buildHTTP80Routes(http80Routes),
			},
		},
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
		Admin: &AdminConfig{Listen: "localhost:2019"},
		Apps:  apps,
	}, nil
}

// BuildInput holds all entities needed to generate the full Caddy config.
type BuildInput struct {
	ProxyHosts       []models.ProxyHost
	RedirectionHosts []models.RedirectionHost
	StreamHosts      []models.StreamHost
	DeadHosts        []models.DeadHost
	Certificates     []*models.Certificate
	AccessLists      []models.AccessList
	// AgentCertDir is set for remote servers. When non-empty, BuildTLSApp uses
	// predictable agent-side paths instead of the controller's local file paths.
	AgentCertDir string
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
