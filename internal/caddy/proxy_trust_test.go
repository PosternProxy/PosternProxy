package caddy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/posternproxy/posternproxy/internal/models"
)

func TestProxyPolicyValidation(t *testing.T) {
	for _, raw := range []string{`[]`, `null`, `{"0":[]}`, `{"01":[]}`, `{"1":["private_ranges"]}`, `{"1":["0.0.0.0/0"]}`, `{"1":["::/0"]}`, `{"1":[],"1":[]}`, `{"1":[]} {}`, `{"1":["::ffff:127.0.0.1/128"]}`} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseTrustedProxyPolicies(raw); err == nil {
				t.Fatal("accepted invalid trust policy")
			}
		})
	}
	p, err := ParseTrustedProxyPolicies(`{"1":["192.0.2.1/32"],"2":["2001:db8::1/128"]}`)
	if err != nil || len(p) != 2 || len(p[3]) != 0 || p[2][0] != "2001:db8::1/128" {
		t.Fatalf("per-server policy: %v %v", p, err)
	}
}

func TestGeneratedClientIdentityAndLimits(t *testing.T) {
	h := testProxyHost("identity.test")
	h.HTTPOnly = true
	h.RateLimitEnabled, h.RateLimitRequests = true, 2
	id := int64(9)
	h.AccessListID = &id
	cfg, err := BuildFullConfig(&BuildInput{TrustedProxies: []string{"192.0.2.1/32"}, ProxyHosts: []models.ProxyHost{h}, AccessLists: []models.AccessList{{ID: id, Clients: []models.AccessListClient{{Address: "198.51.100.1", Directive: "allow"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range httpServers(t, cfg) {
		if s.TrustedProxiesStrict != 1 || s.TrustedProxies.Ranges[0] != "192.0.2.1/32" || s.ReadHeaderTimeout != "10s" || s.IdleTimeout != "2m" || s.MaxHeaderBytes != 1<<20 {
			t.Fatalf("incorrect server safeguards: %+v", s)
		}
	}
	raw, _ := json.Marshal(cfg)
	for _, key := range []string{`"client_ip"`, `{http.vars.client_ip}`} {
		if !strings.Contains(string(raw), key) {
			t.Fatalf("missing %s", key)
		}
	}
	if strings.Contains(string(raw), `"remote_ip"`) || strings.Contains(string(raw), "http.request.remote.host") {
		t.Fatal("visitor policy still uses socket peer")
	}
	direct, err := BuildFullConfig(&BuildInput{})
	if err != nil || httpServers(t, direct)["http"].TrustedProxies != nil {
		t.Fatal("default configuration trusts proxies")
	}
}
