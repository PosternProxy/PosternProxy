package caddy

import (
	"encoding/json"
	"fmt"

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
	}, nil
}

// BuildRedirectionHostRoute generates a Caddy route for a redirection host.
func BuildRedirectionHostRoute(h *models.RedirectionHost) (*Route, error) {
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

	return &Route{
		ID:    h.CaddyID,
		Match: []Matcher{{Host: domains}},
		Handle: []Handler{{
			Handler:       "static_response",
			StatusCode:    h.StatusCode,
			StaticHeaders: map[string][]string{"Location": {target}},
		}},
		Terminal: true,
	}, nil
}

// BuildDeadHostRoute generates a Caddy route returning a parked-domain response.
func BuildDeadHostRoute(h *models.DeadHost) (*Route, error) {
	var domains []string
	if err := json.Unmarshal(h.DomainNames, &domains); err != nil {
		return nil, fmt.Errorf("parse domain names: %w", err)
	}

	code := h.StatusCode
	if code == 0 {
		code = 404
	}

	body := h.CustomBody
	if body == "" && code != 200 {
		titles := map[int]string{404: "404 Not Found", 410: "410 Gone", 503: "503 Service Unavailable"}
		messages := map[int]string{
			404: "This domain is not available.",
			410: "This domain has been permanently removed.",
			503: "This domain is temporarily unavailable.",
		}
		title := titles[code]
		if title == "" {
			title = "Not Available"
		}
		msg := messages[code]
		if msg == "" {
			msg = "This domain is not available."
		}
		body = "<!DOCTYPE html><html><head><title>" + title + "</title></head><body><h1>" + title + "</h1><p>" + msg + "</p></body></html>"
	}

	return &Route{
		ID:    h.CaddyID,
		Match: []Matcher{{Host: domains}},
		Handle: []Handler{{
			Handler:       "static_response",
			StatusCode:    code,
			Body:          body,
			StaticHeaders: map[string][]string{"Content-Type": {"text/html; charset=utf-8"}},
		}},
		Terminal: true,
	}, nil
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
		transport.TLS = &TLSOptions{}
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
		Headers: &HeaderOps{
			Request: &HeaderMutation{
				Set: map[string][]string{
					"X-Forwarded-Proto": {"{http.request.scheme}"},
					"X-Real-IP":         {"{http.request.remote.host}"},
				},
			},
		},
		HealthChecks: buildHealthChecks(h),
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
		transport.TLS = &TLSOptions{}
	}

	return &Route{
		Match: []Matcher{{Path: []string{loc.Path + "*"}}},
		Handle: []Handler{{
			Handler:   "reverse_proxy",
			Upstreams: []*Upstream{upstream},
			Transport: transport,
		}},
		Terminal: true,
	}
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
			Match:    []Matcher{{RemoteIP: &RemoteIPMatcher{Ranges: denyRanges}}},
			Handle:   []Handler{{Handler: "static_response", StatusCode: 403}},
			Terminal: true,
		})
	}

	// Allow rules create an allowlist — block everyone not in the list.
	if len(allowRanges) > 0 {
		routes = append(routes, &Route{
			Match: []Matcher{{
				Not: []Matcher{{RemoteIP: &RemoteIPMatcher{Ranges: allowRanges}}},
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
				Key:       "{http.request.remote.host}",
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

// BuildErrorRoutes generates the server-level error handling routes from all
// enabled proxy hosts that have custom error pages configured.
//
// Caddy's apps.http.servers.<name>.errors.routes[] supports matchers with
// both "host" and "error" keys AND'd together, so we can serve per-host
// custom pages without conflicting across virtual hosts.
//
// The returned slice is placed in HTTPServer.Errors.Routes.
func BuildErrorRoutes(hosts []models.ProxyHost) []*Route {
	var routes []*Route

	for i := range hosts {
		h := &hosts[i]
		if !h.Enabled || len(h.ErrorPages) == 0 {
			continue
		}

		// Decode error_pages: map of "404" → "<html>…</html>"
		var pages map[string]string
		if err := json.Unmarshal(h.ErrorPages, &pages); err != nil || len(pages) == 0 {
			continue
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
					Host:  domains,
					Error: &ErrorMatcher{StatusCodes: []int{statusCode}},
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
