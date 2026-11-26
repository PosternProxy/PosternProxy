package caddy

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/posternproxy/posternproxy/internal/models"
)

func testProxyHost(domain string) models.ProxyHost {
	return models.ProxyHost{
		ID:          1,
		ServerID:    1,
		CaddyID:     "proxy_host_1",
		DomainNames: json.RawMessage(`["` + domain + `"]`),
		ForwardHost: "192.168.1.10",
		ForwardPort: 8080,
		Enabled:     true,
	}
}

func TestFullConfigPreservesInstallerAdminListener(t *testing.T) {
	cfg, err := BuildFullConfig(&BuildInput{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Admin == nil || cfg.Admin.Listen != "127.0.0.1:2019" {
		t.Fatalf("generated admin listener differs from native/container bootstrap: %+v", cfg.Admin)
	}
}

// httpServers unmarshals the http app back out of a built config for inspection.
func httpServers(t *testing.T, cfg *Config) map[string]*HTTPServer {
	t.Helper()
	raw, ok := cfg.Apps["http"]
	if !ok {
		t.Fatal("config has no http app")
	}
	var app HTTPApp
	if err := json.Unmarshal(raw, &app); err != nil {
		t.Fatalf("unmarshal http app: %v", err)
	}
	return app.Servers
}

// hostMatched reports whether any route carries a host matcher for domain.
func hostMatched(routes []*Route, domain string) bool {
	for _, r := range routes {
		for _, m := range r.Match {
			if slices.Contains(m.Host, domain) {
				return true
			}
		}
	}
	return false
}

func errorRouteBody(routes []*Route, domain string, statusCode int) (string, bool) {
	for _, r := range routes {
		matchedHost := false
		matchedStatus := false
		for _, m := range r.Match {
			if slices.Contains(m.Host, domain) {
				matchedHost = true
			}
			if strings.Contains(m.Expression, fmt.Sprintf("== %d", statusCode)) {
				matchedStatus = true
			}
		}
		if !matchedHost || !matchedStatus || len(r.Handle) == 0 {
			continue
		}
		return r.Handle[0].Body, true
	}
	return "", false
}

func TestBuildFullConfig_NonSSLForcedServesOverHTTP(t *testing.T) {
	h := testProxyHost("app.test")
	h.SSLForced = false // no cert, no forced SSL

	cfg, err := BuildFullConfig(&BuildInput{
		ProxyHosts:        []models.ProxyHost{h},
		ControllerVersion: "v9.9.9",
	})
	if err != nil {
		t.Fatalf("BuildFullConfig: %v", err)
	}
	servers := httpServers(t, cfg)

	if !hostMatched(servers["http"].Routes, "app.test") {
		t.Error("non-SSL-forced host should be served directly on the :80 server")
	}
	if hostMatched(servers["https"].Routes, "app.test") {
		t.Error("non-SSL-forced host with no certificate should not be on the :443 server")
	}
}

func TestBuildFullConfig_SSLForcedRedirectsHTTP(t *testing.T) {
	h := testProxyHost("secure.test")
	h.SSLForced = true

	cfg, err := BuildFullConfig(&BuildInput{ProxyHosts: []models.ProxyHost{h}})
	if err != nil {
		t.Fatalf("BuildFullConfig: %v", err)
	}
	servers := httpServers(t, cfg)

	if !hostMatched(servers["https"].Routes, "secure.test") {
		t.Error("SSL-forced host should be served on the :443 server")
	}
	if hostMatched(servers["http"].Routes, "secure.test") {
		t.Error("SSL-forced host should not have a direct :80 route (it is redirected)")
	}
	// The :80 server should still carry the redirect catch-all.
	if len(servers["http"].Routes) == 0 {
		t.Error("expected a redirect catch-all on the :80 server")
	}
}

func TestBuildFullConfig_NonSSLForcedWithCertAlsoServesHTTPS(t *testing.T) {
	certID := int64(7)
	h := testProxyHost("dual.test")
	h.SSLForced = false
	h.CertificateID = &certID

	cfg, err := BuildFullConfig(&BuildInput{ProxyHosts: []models.ProxyHost{h}})
	if err != nil {
		t.Fatalf("BuildFullConfig: %v", err)
	}
	servers := httpServers(t, cfg)

	if !hostMatched(servers["http"].Routes, "dual.test") {
		t.Error("host should be served on :80")
	}
	if !hostMatched(servers["https"].Routes, "dual.test") {
		t.Error("host with an assigned certificate should also be served on :443")
	}
}

func testRedirectionHost(domain string) models.RedirectionHost {
	return models.RedirectionHost{
		ID:                1,
		ServerID:          1,
		CaddyID:           "redirection_host_1",
		DomainNames:       json.RawMessage(`["` + domain + `"]`),
		ForwardScheme:     "https",
		ForwardDomainName: "target.example.com",
		StatusCode:        301,
		Enabled:           true,
	}
}

func TestBuildFullConfig_RedirectionNonSSLForcedServesOverHTTP(t *testing.T) {
	h := testRedirectionHost("old.test")
	h.SSLForced = false

	cfg, err := BuildFullConfig(&BuildInput{RedirectionHosts: []models.RedirectionHost{h}})
	if err != nil {
		t.Fatalf("BuildFullConfig: %v", err)
	}
	servers := httpServers(t, cfg)

	if !hostMatched(servers["http"].Routes, "old.test") {
		t.Error("non-SSL-forced redirection host should be served directly on :80")
	}
	if hostMatched(servers["https"].Routes, "old.test") {
		t.Error("non-SSL-forced redirection host with no cert should not be on :443")
	}
}

func TestBuildFullConfig_RedirectionSSLForcedOnHTTPS(t *testing.T) {
	h := testRedirectionHost("old.test")
	h.SSLForced = true

	cfg, err := BuildFullConfig(&BuildInput{RedirectionHosts: []models.RedirectionHost{h}})
	if err != nil {
		t.Fatalf("BuildFullConfig: %v", err)
	}
	servers := httpServers(t, cfg)

	if !hostMatched(servers["https"].Routes, "old.test") {
		t.Error("SSL-forced redirection host should be on :443")
	}
	if hostMatched(servers["http"].Routes, "old.test") {
		t.Error("SSL-forced redirection host should not have a direct :80 route")
	}
}

func TestBuildFullConfig_DeadHostServedOnBothPorts(t *testing.T) {
	h := models.DeadHost{
		ID:          1,
		ServerID:    1,
		CaddyID:     "dead_host_1",
		DomainNames: json.RawMessage(`["parked.test"]`),
		StatusCode:  404,
		Enabled:     true,
	}

	cfg, err := BuildFullConfig(&BuildInput{DeadHosts: []models.DeadHost{h}})
	if err != nil {
		t.Fatalf("BuildFullConfig: %v", err)
	}
	servers := httpServers(t, cfg)

	if !hostMatched(servers["http"].Routes, "parked.test") {
		t.Error("dead host should be served on :80")
	}
	if !hostMatched(servers["https"].Routes, "parked.test") {
		t.Error("dead host should be served on :443")
	}
}

func TestBuildProxyHostRoute_AdvancedConfigMerged(t *testing.T) {
	h := testProxyHost("app.test")
	h.AdvancedConfig = `{"terminal":false,"custom_key":"custom_value"}`

	route, err := BuildProxyHostRoute(&h, nil)
	if err != nil {
		t.Fatalf("BuildProxyHostRoute: %v", err)
	}
	raw, err := json.Marshal(route)
	if err != nil {
		t.Fatalf("marshal route: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal route: %v", err)
	}

	if string(got["custom_key"]) != `"custom_value"` {
		t.Errorf("advanced config key not merged: got %s", raw)
	}
	if string(got["terminal"]) != "false" {
		t.Errorf("advanced config should override terminal: got %s", got["terminal"])
	}
}

func TestBuildProxyHostRoute_InvalidAdvancedConfigIgnored(t *testing.T) {
	h := testProxyHost("app.test")
	h.AdvancedConfig = `not valid json`

	route, err := BuildProxyHostRoute(&h, nil)
	if err != nil {
		t.Fatalf("BuildProxyHostRoute: %v", err)
	}
	// Marshaling must still succeed (invalid advanced config is ignored, not fatal).
	if _, err := json.Marshal(route); err != nil {
		t.Fatalf("marshal route with invalid advanced config: %v", err)
	}
}

func TestBuildFullConfig_HTTP2Disabled(t *testing.T) {
	cfg, err := BuildFullConfig(&BuildInput{HTTP2Disabled: true})
	if err != nil {
		t.Fatalf("BuildFullConfig: %v", err)
	}
	protocols := httpServers(t, cfg)["https"].Protocols
	if len(protocols) != 1 || protocols[0] != "h1" {
		t.Errorf("HTTP2Disabled should restrict https protocols to [h1], got %v", protocols)
	}
}

func TestBuildFullConfig_HTTP2EnabledByDefault(t *testing.T) {
	cfg, err := BuildFullConfig(&BuildInput{})
	if err != nil {
		t.Fatalf("BuildFullConfig: %v", err)
	}
	if p := httpServers(t, cfg)["https"].Protocols; p != nil {
		t.Errorf("default should leave protocols unset (Caddy default), got %v", p)
	}
}

func TestBuildFullConfig_DisablesCaddyAutoRedirects(t *testing.T) {
	cfg, err := BuildFullConfig(&BuildInput{})
	if err != nil {
		t.Fatalf("BuildFullConfig: %v", err)
	}
	servers := httpServers(t, cfg)
	if servers["https"].AutomaticHTTPS == nil || !servers["https"].AutomaticHTTPS.DisableRedirects {
		t.Error("https server should disable Caddy's automatic HTTP→HTTPS redirects")
	}
}

func TestBuildFullConfig_ProxyErrorRoutesApplyToHTTPAndHTTPS(t *testing.T) {
	h := testProxyHost("app.test")

	cfg, err := BuildFullConfig(&BuildInput{ProxyHosts: []models.ProxyHost{h}})
	if err != nil {
		t.Fatalf("BuildFullConfig: %v", err)
	}
	servers := httpServers(t, cfg)

	for name, server := range servers {
		if server.Errors == nil {
			t.Fatalf("%s server has no error routes", name)
		}
		if _, ok := errorRouteBody(server.Errors.Routes, "app.test", 502); !ok {
			t.Errorf("%s server has no 502 host-down route", name)
		}
		if _, ok := errorRouteBody(server.Errors.Routes, "app.test", 503); !ok {
			t.Errorf("%s server has no 503 host-down route", name)
		}
	}
}

func TestBuildErrorRoutes_CustomPagesOverrideDefaults(t *testing.T) {
	h := testProxyHost("app.test")
	h.ErrorPages = json.RawMessage(`{"502":"custom bad gateway"}`)

	routes := BuildErrorRoutes([]models.ProxyHost{h}, "v9.9.9")

	body, ok := errorRouteBody(routes, "app.test", 502)
	if !ok {
		t.Fatal("missing 502 error route")
	}
	if body != "custom bad gateway" {
		t.Errorf("502 body = %q, want custom body", body)
	}
	body, ok = errorRouteBody(routes, "app.test", 503)
	if !ok {
		t.Fatal("missing default 503 error route")
	}
	if body == "" || body == "custom bad gateway" {
		t.Errorf("503 body should keep default, got %q", body)
	}
}

func TestBuildDeadHostRoute_DefaultParkedPage(t *testing.T) {
	h := &models.DeadHost{
		ID: 1, ServerID: 1, CaddyID: "dead_host_1",
		DomainNames: json.RawMessage(`["parked.test"]`),
		StatusCode:  410, Enabled: true,
	}
	route, err := BuildDeadHostRoute(h, "v1.2.3")
	if err != nil {
		t.Fatalf("BuildDeadHostRoute: %v", err)
	}
	body := route.Handle[0].Body
	for _, want := range []string{"PosternProxy", "v1.2.3", "410", "gone"} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
			t.Errorf("parked page missing %q", want)
		}
	}
}

func TestBuildDeadHostRoute_CustomBodyWins(t *testing.T) {
	h := &models.DeadHost{
		DomainNames: json.RawMessage(`["parked.test"]`),
		StatusCode:  404, CustomBody: "my own page",
	}
	route, err := BuildDeadHostRoute(h, "v1")
	if err != nil {
		t.Fatalf("BuildDeadHostRoute: %v", err)
	}
	if route.Handle[0].Body != "my own page" {
		t.Errorf("custom body should win, got %q", route.Handle[0].Body)
	}
}

func TestBuildDeadHostRoute_200GetsBrandedPageWithoutBigCode(t *testing.T) {
	h := &models.DeadHost{
		DomainNames: json.RawMessage(`["parked.test"]`),
		StatusCode:  200, Enabled: true,
	}
	route, err := BuildDeadHostRoute(h, "v1")
	if err != nil {
		t.Fatalf("BuildDeadHostRoute: %v", err)
	}
	body := route.Handle[0].Body
	if !strings.Contains(strings.ToLower(body), "parked") {
		t.Error("200 parked page should mention 'parked'")
	}
	if strings.Contains(body, `class="code"`) {
		t.Error("200 page should not render the giant status-code block")
	}
}

func TestBuildErrorRoutes_DefaultPageIncludesBrandAndVersion(t *testing.T) {
	h := testProxyHost("app.test")

	routes := BuildErrorRoutes([]models.ProxyHost{h}, "v1.2.3")

	body, ok := errorRouteBody(routes, "app.test", 502)
	if !ok {
		t.Fatal("missing 502 error route")
	}
	for _, want := range []string{"PosternProxy", "v1.2.3", "Upstream issue", "What to check"} {
		if !strings.Contains(body, want) {
			t.Errorf("default page missing %q", want)
		}
	}
}

func TestParkedHTTPOnlyHasNoHTTPSRoute(t *testing.T) {
	cfg, err := BuildFullConfig(&BuildInput{DeadHosts: []models.DeadHost{{ID: 99, CaddyID: "park_http", Enabled: true, HTTPOnly: true, DomainNames: json.RawMessage(`["park-http.example"]`)}}})
	if err != nil {
		t.Fatal(err)
	}
	var app HTTPApp
	if err := json.Unmarshal(cfg.Apps["http"], &app); err != nil {
		t.Fatal(err)
	}
	httpJSON, _ := json.Marshal(app.Servers["http"])
	httpsJSON, _ := json.Marshal(app.Servers["https"])
	if !strings.Contains(string(httpJSON), "park-http.example") || strings.Contains(string(httpsJSON), "park-http.example") {
		t.Fatal("HTTP-only parked domain leaked into HTTPS routes")
	}
}
