package caddy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/posternproxy/posternproxy/internal/models"
)

func TestNTLMRouteSelectsTransportAndKeepsOtherPolicy(t *testing.T) {
	host := testProxyHost("ntlm.example")
	host.NTLMEnabled = true
	host.ForwardScheme = "https"
	host.RateLimitEnabled = true
	host.RateLimitRequests = 20
	host.HCEnabled = true
	host.HCActivePath = "/health"
	list := &models.AccessList{Clients: []models.AccessListClient{{Address: "192.0.2.0/24", Directive: "deny"}}}

	route, err := BuildProxyHostRoute(&host, list)
	if err != nil {
		t.Fatal(err)
	}
	raw := mustJSON(t, route)
	assertContains(t, raw, `"protocol":"http_ntlm"`)
	assertContains(t, raw, `"handshake_timeout":"10s"`)
	assertContains(t, raw, `"handler":"rate_limit"`)
	assertContains(t, raw, "192.0.2.0/24")
	assertContains(t, raw, `"/health"`)
	for _, forbidden := range []string{"insecure_skip_verify", `"versions"`, "keep_alive", "max_conns_per_host"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("route contains %s: %s", forbidden, raw)
		}
	}
	if strings.Contains(strings.ReplaceAll(raw, `"protocol":"http_ntlm"`, ""), `"protocol":`) {
		t.Fatalf("route emitted another transport protocol: %s", raw)
	}
}

func TestDisabledNTLMRouteStaysHTTP(t *testing.T) {
	host := testProxyHost("plain.example")
	route, err := BuildProxyHostRoute(&host, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw := mustJSON(t, route)
	assertContains(t, raw, `"protocol":"http"`)
	if strings.Contains(raw, "http_ntlm") {
		t.Fatalf("ordinary route selected NTLM: %s", raw)
	}

	location := buildLocationRoute(models.ProxyHostLocation{Path: "/api", ForwardScheme: "https", ForwardHost: "10.0.0.2", ForwardPort: 9})
	locRaw := mustJSON(t, location)
	assertContains(t, locRaw, `"protocol":"http"`)
	assertContains(t, locRaw, `"handshake_timeout":"10s"`)
	if strings.Contains(locRaw, "http_ntlm") || strings.Contains(locRaw, "insecure_skip_verify") {
		t.Fatalf("location route changed transport: %s", locRaw)
	}
}

func TestNTLMLocationsFailBeforeARouteIsEmitted(t *testing.T) {
	host := testProxyHost("locations.example")
	host.NTLMEnabled = true
	host.Locations = json.RawMessage(`[{"path":"/secret"}]`)
	route, err := BuildProxyHostRoute(&host, nil)
	if route != nil || err == nil || err.Error() != models.NTLMErrLocations {
		t.Fatalf("route=%v err=%v", route, err)
	}
}

func TestPoisonedNTLMHostBlocksTheServerConfig(t *testing.T) {
	good := testProxyHost("good.example")
	good.ID = 7
	good.NTLMEnabled = true
	bad := testProxyHost("bad.example")
	bad.ID = 8
	bad.NTLMEnabled = true
	bad.TLSPassthrough = true
	_, err := BuildFullConfig(&BuildInput{ProxyHosts: []models.ProxyHost{good, bad}})
	if err == nil || !strings.Contains(err.Error(), "proxy host 8") || !strings.Contains(err.Error(), models.NTLMErrTLSPassthrough) {
		t.Fatal(err)
	}
}

func TestNTLMDoesNotForceHTTP1(t *testing.T) {
	host := testProxyHost("ntlm.example")
	host.NTLMEnabled = true
	cfg, err := BuildFullConfig(&BuildInput{ProxyHosts: []models.ProxyHost{host}})
	if err != nil {
		t.Fatal(err)
	}
	raw := mustJSON(t, cfg)
	if strings.Contains(raw, `"protocols"`) {
		t.Fatalf("NTLM forced protocols: %s", raw)
	}
	assertContains(t, raw, `"protocol":"http_ntlm"`)
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func assertContains(t *testing.T, raw, needle string) {
	t.Helper()
	if !strings.Contains(raw, needle) {
		t.Fatalf("missing %s in %s", needle, raw)
	}
}
