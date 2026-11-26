package caddy

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"

	"github.com/posternproxy/posternproxy/internal/models"
)

// BuildProxyHostRoute generates a Caddy HTTP route for a proxy host.
func BuildProxyHostRoute(h *models.ProxyHost, al *models.AccessList) (*Route, error) {
	var domains []string
	if err := json.Unmarshal(h.DomainNames, &domains); err != nil {
		return nil, fmt.Errorf("parse domain names: %w", err)
	}

	var locations []models.ProxyHostLocation
	if len(h.Locations) > 0 {
		json.Unmarshal(h.Locations, &locations)
	}

	// Build inner handlers (access list + proxy)
	var innerRoutes []*Route

	// IP restriction from access list
	if al != nil && len(al.Clients) > 0 {
		innerRoutes = append(innerRoutes, buildIPRestrictionRoutes(al)...)
	}

	// Custom locations (path-specific upstreams before the default)
	for _, loc := range locations {
		locRoute := buildLocationRoute(loc)
		innerRoutes = append(innerRoutes, locRoute)
	}

	// Default upstream route
	innerRoutes = append(innerRoutes, buildReverseProxyRoute(h))

	// Wrap in subroute
	subroute := Handler{
		Handler: "subroute",
		Routes:  innerRoutes,
	}

	// Assemble the handler chain for this route.
	var handlers []Handler
	// 1. Rate limiting
	if h.RateLimitEnabled && h.RateLimitRequests > 0 {
		handlers = append(handlers, buildRateLimitHandler(h))
	}
	// 3. Custom request/response header injection
	if hdrHandler := buildCustomHeadersHandler(h); hdrHandler != nil {
		handlers = append(handlers, *hdrHandler)
	}
	// 4. Subroute (IP restriction + locations + reverse_proxy)
	handlers = append(handlers, subroute)

	return &Route{
		ID:       h.CaddyID,
		Match:    []Matcher{{Host: domains}},
		Handle:   handlers,
		Terminal: true,
		Extra:    advancedConfigJSON(h.AdvancedConfig),
	}, nil
}

// advancedConfigJSON returns the host's advanced-config snippet as raw JSON to be
// merged into the route, or nil when it is blank/empty so nothing is merged.
func advancedConfigJSON(s string) json.RawMessage {
	switch strings.TrimSpace(s) {
	case "", "{}":
		return nil
	default:
		return json.RawMessage(s)
	}
}

// BuildRedirectionHostRoute generates a Caddy route for a redirection host.
func BuildRedirectionHostRoute(h *models.RedirectionHost, lists ...*models.AccessList) (*Route, error) {
	var domains []string
	if err := json.Unmarshal(h.DomainNames, &domains); err != nil {
		return nil, fmt.Errorf("parse domain names: %w", err)
	}

	scheme := h.ForwardScheme
	if scheme == "auto" {
		scheme = "https"
	}

	target := scheme + "://" + h.ForwardDomainName + h.ForwardPath
	if h.PreservePath {
		target += "{http.request.uri}"
	}

	route := &Route{
		ID:    h.CaddyID,
		Match: []Matcher{{Host: domains}},
		Handle: []Handler{{
			Handler:       "static_response",
			StatusCode:    h.StatusCode,
			StaticHeaders: map[string][]string{"Location": {target}},
		}},
		Terminal: true,
	}
	if len(lists) > 0 && lists[0] != nil {
		restrictRoute(route, lists[0])
	}
	return route, nil
}

// BuildDeadHostRoute generates a Caddy route returning a parked-domain response.
// When no custom body is supplied, a branded PosternProxy parked-domain page is
// served (for 200/404/410/503 and any other code).
func BuildDeadHostRoute(h *models.DeadHost, version string, lists ...*models.AccessList) (*Route, error) {
	var domains []string
	if err := json.Unmarshal(h.DomainNames, &domains); err != nil {
		return nil, fmt.Errorf("parse domain names: %w", err)
	}

	code := h.StatusCode
	if code == 0 {
		code = 404
	}

	body := h.CustomBody
	if body == "" {
		body = defaultParkedPage(code, version)
	}

	route := &Route{
		ID:    h.CaddyID,
		Match: []Matcher{{Host: domains}},
		Handle: []Handler{{
			Handler:       "static_response",
			StatusCode:    code,
			Body:          body,
			StaticHeaders: map[string][]string{"Content-Type": {"text/html; charset=utf-8"}},
		}},
		Terminal: true,
	}
	if len(lists) > 0 && lists[0] != nil {
		restrictRoute(route, lists[0])
	}
	return route, nil
}

// --- helpers ---

func buildReverseProxyRoute(h *models.ProxyHost) *Route {
	// Primary upstream is always first.
	upstreams := []*Upstream{
		{Dial: fmt.Sprintf("%s:%d", h.ForwardHost, h.ForwardPort)},
	}

	// Append extra upstreams from the load-balancing config.
	if len(h.ExtraUpstreams) > 0 {
		var extras []models.ExtraUpstream
		if err := json.Unmarshal(h.ExtraUpstreams, &extras); err == nil {
			for _, e := range extras {
				if e.Host != "" && e.Port > 0 {
					upstreams = append(upstreams, &Upstream{
						Dial: fmt.Sprintf("%s:%d", e.Host, e.Port),
					})
				}
			}
		}
	}

	transport := &Transport{Protocol: "http"}
	if h.ForwardScheme == "https" {
		transport.TLS = &TLSOptions{HandshakeTimeout: "10s"}
	}

	// Load balancing — only emit when there are multiple upstreams.
	var lb *LoadBalancing
	if len(upstreams) > 1 {
		policy := h.LBPolicy
		if policy == "" {
			policy = "random"
		}
		lb = &LoadBalancing{
			SelectionPolicy: &SelectionPolicy{Policy: policy},
			// Retry all other upstreams before giving up.
			Retries: len(upstreams) - 1,
		}
	}

	proxyHandler := Handler{
		Handler:       "reverse_proxy",
		Upstreams:     upstreams,
		Transport:     transport,
		LoadBalancing: lb,
		Headers:       clientHeaders(),
		HealthChecks:  buildHealthChecks(h),
	}

	return &Route{Handle: []Handler{proxyHandler}}
}

// buildHealthChecks constructs the Caddy health_checks block from model fields.
// Returns nil when health checking is disabled.
func buildHealthChecks(h *models.ProxyHost) *HealthChecks {
	if !h.HCEnabled {
		return nil
	}

	hc := &HealthChecks{}

	// Active health checks — only add if a path is configured.
	if h.HCActivePath != "" {
		active := &ActiveHealthCheck{
			URI: h.HCActivePath,
		}
		if h.HCActiveInterval > 0 {
			active.Interval = fmt.Sprintf("%ds", h.HCActiveInterval)
		}
		if h.HCActiveTimeout > 0 {
			active.Timeout = fmt.Sprintf("%ds", h.HCActiveTimeout)
		}
		if h.HCActiveExpectStatus > 0 {
			active.ExpectStatus = h.HCActiveExpectStatus
		}
		hc.Active = active
	}

	// Passive health checks — always include when HC is enabled.
	passive := &PassiveHealthCheck{}
	if h.HCPassiveFailDuration > 0 {
		passive.FailDuration = fmt.Sprintf("%ds", h.HCPassiveFailDuration)
	}
	if h.HCPassiveMaxFails > 0 {
		passive.MaxFails = h.HCPassiveMaxFails
	}
	if h.HCPassiveUnhealthyLatency > 0 {
		passive.UnhealthyLatency = fmt.Sprintf("%dms", h.HCPassiveUnhealthyLatency)
	}
	// Only attach passive block if at least one field is set.
	if passive.FailDuration != "" || passive.MaxFails > 0 || passive.UnhealthyLatency != "" {
		hc.Passive = passive
	}

	// If neither active nor passive is configured, return nil (avoid empty object).
	if hc.Active == nil && hc.Passive == nil {
		return nil
	}
	return hc
}

func buildLocationRoute(loc models.ProxyHostLocation) *Route {
	upstream := &Upstream{
		Dial: fmt.Sprintf("%s:%d", loc.ForwardHost, loc.ForwardPort),
	}
	transport := &Transport{Protocol: "http"}
	if loc.ForwardScheme == "https" {
		transport.TLS = &TLSOptions{HandshakeTimeout: "10s"}
	}

	return &Route{
		Match: []Matcher{{Path: []string{loc.Path + "*"}}},
		Handle: []Handler{{
			Handler:   "reverse_proxy",
			Upstreams: []*Upstream{upstream},
			Transport: transport,
			Headers:   clientHeaders(),
		}},
		Terminal: true,
	}
}

func clientHeaders() *HeaderOps {
	return &HeaderOps{Request: &HeaderMutation{Set: map[string][]string{
		"X-Real-IP":       {"{http.vars.client_ip}"},
		"X-Forwarded-For": {"{http.vars.client_ip}"},
	}}}
}

func buildIPRestrictionRoutes(al *models.AccessList) []*Route {
	var allowRanges, denyRanges []string
	for _, c := range al.Clients {
		if c.Directive == "allow" {
			allowRanges = append(allowRanges, c.Address)
		} else {
			denyRanges = append(denyRanges, c.Address)
		}
	}

	var routes []*Route

	// Explicit deny rules run first — block these IPs regardless of allow list.
	if len(denyRanges) > 0 {
		routes = append(routes, &Route{
			Match:    []Matcher{{ClientIP: &RemoteIPMatcher{Ranges: denyRanges}}},
			Handle:   []Handler{{Handler: "static_response", StatusCode: 403}},
			Terminal: true,
		})
	}

	// Allow rules create an allowlist — block everyone not in the list.
	if len(allowRanges) > 0 {
		routes = append(routes, &Route{
			Match: []Matcher{{
				Not: []Matcher{{ClientIP: &RemoteIPMatcher{Ranges: allowRanges}}},
			}},
			Handle:   []Handler{{Handler: "static_response", StatusCode: 403}},
			Terminal: true,
		})
	}

	return routes
}

// buildRateLimitHandler returns a rate_limit handler for the proxy host.
// Requires Caddy to be built with github.com/mholt/caddy-ratelimit.
func buildRateLimitHandler(h *models.ProxyHost) Handler {
	window := h.RateLimitWindow
	if window == "" {
		window = "1m"
	}
	zoneName := fmt.Sprintf("proxy_host_%d", h.ID)
	return Handler{
		Handler: "rate_limit",
		RateLimits: map[string]*RateLimitZone{
			zoneName: {
				Key:       "{http.vars.client_ip}",
				Window:    window,
				MaxEvents: h.RateLimitRequests,
			},
		},
	}
}

// BuildPassthroughL4Route returns an L4 route that SNI-matches the proxy host
// domains and proxies raw TLS bytes to the upstream without termination.
func BuildPassthroughL4Route(h *models.ProxyHost) (*L4Route, error) {
	var domains []string
	if err := json.Unmarshal(h.DomainNames, &domains); err != nil {
		return nil, fmt.Errorf("parse domain names: %w", err)
	}
	upstream := fmt.Sprintf("%s:%d", h.ForwardHost, h.ForwardPort)
	return &L4Route{
		Match: []L4Matcher{{TLS: &L4TLSMatcher{SNI: domains}}},
		Handle: []L4Handler{{
			Handler:   "proxy",
			Upstreams: []L4Upstream{{Dial: []string{upstream}}},
		}},
	}, nil
}

// BuildErrorRoutes generates server-level error handling routes for enabled
// HTTP proxy hosts. Each host gets a default 502/503 page, and configured custom
// pages override those defaults.
//
// Each route AND's a "host" matcher with an "expression" matcher on
// {http.error.status_code}, so per-host pages don't conflict across virtual
// hosts. Caddy has no "error" matcher module, so the status code must be matched
// via the CEL expression matcher.
//
// The returned slice is placed in HTTPServer.Errors.Routes.
func BuildErrorRoutes(hosts []models.ProxyHost, version string) []*Route {
	var routes []*Route

	for i := range hosts {
		h := &hosts[i]
		if !h.Enabled || h.TLSPassthrough {
			continue
		}

		pages := map[string]string{
			"502": defaultProxyErrorPage(502, version),
			"503": defaultProxyErrorPage(503, version),
		}
		if len(h.ErrorPages) > 0 {
			var custom map[string]string
			if err := json.Unmarshal(h.ErrorPages, &custom); err == nil {
				for code, body := range custom {
					if body != "" {
						pages[code] = body
					}
				}
			}
		}

		// Resolve the domain list for the host matcher.
		var domains []string
		if err := json.Unmarshal(h.DomainNames, &domains); err != nil || len(domains) == 0 {
			continue
		}

		// Emit one route per error code so each gets its own status + body.
		for code, body := range pages {
			if body == "" {
				continue
			}
			statusCode := 0
			fmt.Sscanf(code, "%d", &statusCode)
			if statusCode < 400 || statusCode > 599 {
				continue // ignore invalid codes
			}

			routes = append(routes, &Route{
				Match: []Matcher{{
					Host:       domains,
					Expression: fmt.Sprintf("{http.error.status_code} == %d", statusCode),
				}},
				Handle: []Handler{{
					Handler:    "static_response",
					StatusCode: statusCode,
					Body:       body,
					StaticHeaders: map[string][]string{
						"Content-Type": {"text/html; charset=utf-8"},
					},
				}},
				Terminal: true,
			})
		}
	}

	return routes
}

func defaultProxyErrorPage(statusCode int, version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "unknown"
	}

	title := "Service Unavailable"
	summary := "The request reached PosternProxy, but the upstream service is not available right now."
	detail := "Caddy could not get a usable response from the configured upstream."
	if statusCode == 502 {
		title = "Bad Gateway"
		detail = "Caddy could not connect to the configured upstream, or the upstream closed the connection."
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%d %s - PosternProxy</title>
<style>
%s
</style>
</head>
<body>
<main class="wrap">
<header class="top">
<div class="brand">
%s
</div>
<div class="badge">Upstream issue</div>
</header>
<section class="main">
<div class="hero">
<div class="eyebrow">Request interrupted</div>
<div class="code">%d</div>
<h1>%s</h1>
<p>%s %s</p>
</div>
<aside class="meta">
<h2>Connection details</h2>
<div class="kv">
<div class="row"><span>Host</span><code>{http.request.host}</code></div>
<div class="row"><span>Status</span><code>%d %s</code></div>
<div class="row"><span>Request ID</span><code>{http.request.uuid}</code></div>
<div class="row"><span>Proxy</span><code>PosternProxy %s</code></div>
</div>
<div class="checks">
<p>What to check:</p>
<ul>
<li>Confirm the upstream service is running and listening on the configured port.</li>
<li>Verify firewall, VPN, routing, and private-network access from the proxy server.</li>
<li>Enable health checks in PosternProxy to surface backend reachability before users hit this page.</li>
</ul>
</div>
</aside>
</section>
<footer class="foot">
<span>Generated by PosternProxy</span>
<span>Refresh shortly, or contact the site operator if this continues.</span>
</footer>
</main>
</body>
</html>`, statusCode, html.EscapeString(title), defaultPageStyles, defaultPageLogo, statusCode, html.EscapeString(title), html.EscapeString(summary), html.EscapeString(detail), statusCode, html.EscapeString(title), html.EscapeString(version))
}

// defaultParkedPage renders the branded PosternProxy parked-domain page shown for
// dead hosts that have no custom body. It is visitor-facing (no operator
// internals), with copy tailored to the status code. 200/404/410/503 get specific
// wording; any other code falls back to a generic notice.
func defaultParkedPage(statusCode int, version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		version = "unknown"
	}

	type meta struct{ badge, eyebrow, title, message string }
	metas := map[int]meta{
		200: {"Parked domain", "Coming soon", "This domain is parked", "This domain is registered but isn't serving a website yet. Check back later."},
		404: {"Not found", "Nothing here", "Page not available", "This domain isn't currently serving any content."},
		410: {"Gone", "Removed", "This domain is gone", "The content that was here has been permanently removed."},
		503: {"Unavailable", "Offline", "Temporarily unavailable", "This domain is temporarily offline. Please check back soon."},
	}
	m, ok := metas[statusCode]
	if !ok {
		m = meta{"Notice", "Notice", "This domain is not available", "This domain is not currently serving any content."}
	}

	// A giant "200" reads oddly on a parked page, so only show the big code for errors.
	codeBlock := ""
	if statusCode >= 400 {
		codeBlock = fmt.Sprintf(`<div class="code">%d</div>`, statusCode)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s - PosternProxy</title>
<style>
%s
</style>
</head>
<body>
<main class="wrap parked">
<header class="top">
<div class="brand">
%s
</div>
<div class="badge">%s</div>
</header>
<section class="hero">
<div class="eyebrow">%s</div>
%s
<h1>%s</h1>
<p>%s</p>
</section>
<footer class="foot">
<span>{http.request.host}</span>
<span>PosternProxy %s</span>
</footer>
</main>
</body>
</html>`,
		html.EscapeString(m.title), defaultPageStyles, defaultPageLogo, html.EscapeString(m.badge), html.EscapeString(m.eyebrow),
		codeBlock, html.EscapeString(m.title), html.EscapeString(m.message), html.EscapeString(version))
}

// buildCustomHeadersHandler returns a Caddy "headers" handler that injects,
// modifies, or removes request and response headers for the proxy host.
// Returns nil when no rules are configured and HSTS is disabled.
//
// Handler chain position: after auth/rate-limit, before the subroute.
// The response mutation uses Deferred:true so it runs at response-write time,
// correctly intercepting headers from the upstream reverse_proxy response.
func buildCustomHeadersHandler(h *models.ProxyHost) *Handler {
	// HSTS is meaningless (and harmful) over plain HTTP.
	hstsApplies := h.HSTSEnabled && !h.HTTPOnly
	if len(h.CustomHeaders) == 0 && !hstsApplies {
		return nil
	}

	var rules []models.CustomHeaderRule
	if len(h.CustomHeaders) > 0 {
		// Invalid JSON in the DB column produces an empty rule slice, which
		// is the correct fallback (no custom headers applied).
		_ = json.Unmarshal(h.CustomHeaders, &rules)
	}

	reqMut := &HeaderMutation{
		Set: make(map[string][]string),
		Add: make(map[string][]string),
	}
	resMut := &ResponseHeaderMutation{
		Set:      make(map[string][]string),
		Add:      make(map[string][]string),
		Deferred: true,
	}

	hasReq, hasRes := false, false

	for _, rule := range rules {
		if rule.Name == "" {
			continue
		}
		switch rule.Direction {
		case "request":
			hasReq = true
			switch rule.Operation {
			case "set":
				reqMut.Set[rule.Name] = []string{rule.Value}
			case "add":
				reqMut.Add[rule.Name] = append(reqMut.Add[rule.Name], rule.Value)
			case "delete":
				reqMut.Delete = append(reqMut.Delete, rule.Name)
			}
		case "response":
			hasRes = true
			switch rule.Operation {
			case "set":
				resMut.Set[rule.Name] = []string{rule.Value}
			case "add":
				resMut.Add[rule.Name] = append(resMut.Add[rule.Name], rule.Value)
			case "delete":
				resMut.Delete = append(resMut.Delete, rule.Name)
			}
		}
	}

	// Inject HSTS if enabled (and not HTTP-only — HSTS over plain HTTP is a no-op).
	if hstsApplies {
		hstsValue := "max-age=31536000"
		if h.HSTSSubdomains {
			hstsValue += "; includeSubDomains"
		}
		resMut.Set["Strict-Transport-Security"] = []string{hstsValue}
		hasRes = true
	}

	if !hasReq && !hasRes {
		return nil
	}

	hdr := &Handler{Handler: "headers"}
	if hasReq {
		// Clean up empty maps to keep JSON tidy
		if len(reqMut.Set) == 0 {
			reqMut.Set = nil
		}
		if len(reqMut.Add) == 0 {
			reqMut.Add = nil
		}
		hdr.Request = reqMut
	}
	if hasRes {
		if len(resMut.Set) == 0 {
			resMut.Set = nil
		}
		if len(resMut.Add) == 0 {
			resMut.Add = nil
		}
		hdr.Response = resMut
	}
	return hdr
}

func restrictRoute(route *Route, al *models.AccessList) {
	routes := buildIPRestrictionRoutes(al)
	if len(routes) == 0 {
		return
	}
	routes = append(routes, &Route{Handle: route.Handle, Terminal: true})
	route.Handle = []Handler{{Handler: "subroute", Routes: routes}}
}
