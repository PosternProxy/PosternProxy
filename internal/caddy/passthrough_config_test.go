package caddy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/posternproxy/posternproxy/internal/models"
)

// A TLS-passthrough host must produce a config the pinned Caddy can load: the L4
// fallthrough bridges to the internal HTTPS listener via PROXY protocol, and no
// route uses the nonexistent layer4.handlers.http module (which Caddy rejects,
// taking down the whole data plane).
func TestPassthroughConfigBridgesWithProxyProtocol(t *testing.T) {
	cfg, err := BuildFullConfig(&BuildInput{
		ProxyHosts: []models.ProxyHost{{
			ID: 1, Enabled: true, TLSPassthrough: true,
			DomainNames: json.RawMessage(`["pass.test"]`),
			ForwardHost: "10.0.0.2", ForwardPort: 8443,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if s := string(cfg.Apps["layer4"]); strings.Contains(s, `"handler":"http"`) {
		t.Fatalf("passthrough config uses the nonexistent layer4 http handler: %s", s)
	}

	var l4 Layer4App
	if err := json.Unmarshal(cfg.Apps["layer4"], &l4); err != nil {
		t.Fatal(err)
	}
	server := l4.Servers["tls_passthrough_443"]
	if server == nil {
		t.Fatal("missing tls_passthrough_443 server")
	}
	bridgeFound := false
	for _, r := range server.Routes {
		if len(r.Match) != 0 {
			continue // SNI-matched passthrough route, not the bridge
		}
		bridgeFound = true
		h := r.Handle[0]
		if h.Handler != "proxy" || h.ProxyProtocol != "v2" {
			t.Fatalf("fallthrough must be a proxy_protocol v2 proxy, got %+v", h)
		}
		if len(h.Upstreams) == 0 || len(h.Upstreams[0].Dial) == 0 || h.Upstreams[0].Dial[0] != internalHTTPSAddr {
			t.Fatalf("fallthrough must dial %s, got %+v", internalHTTPSAddr, h.Upstreams)
		}
	}
	if !bridgeFound {
		t.Fatal("no fallthrough bridge route generated")
	}

	var httpApp HTTPApp
	if err := json.Unmarshal(cfg.Apps["http"], &httpApp); err != nil {
		t.Fatal(err)
	}
	https := httpApp.Servers["https"]
	if https == nil || len(https.Listen) != 1 || https.Listen[0] != internalHTTPSAddr {
		t.Fatalf("https server must listen on %s, got %+v", internalHTTPSAddr, https)
	}
	if len(https.ListenerWrappers) != 2 ||
		https.ListenerWrappers[0].Wrapper != "proxy_protocol" ||
		https.ListenerWrappers[1].Wrapper != "tls" {
		t.Fatalf("https must wrap proxy_protocol then tls, got %+v", https.ListenerWrappers)
	}
	if len(https.ListenerWrappers[0].Allow) != 1 || https.ListenerWrappers[0].Allow[0] != "127.0.0.1/32" {
		t.Fatalf("proxy_protocol wrapper must only trust loopback, got %+v", https.ListenerWrappers[0].Allow)
	}
}

// Without any passthrough host, the HTTPS server keeps its public :443 listener
// and no PROXY-protocol bridge is introduced.
func TestNonPassthroughConfigKeepsPublicHTTPS(t *testing.T) {
	cfg, err := BuildFullConfig(&BuildInput{
		ProxyHosts: []models.ProxyHost{{
			ID: 1, Enabled: true,
			DomainNames:   json.RawMessage(`["normal.test"]`),
			ForwardScheme: "http", ForwardHost: "10.0.0.2", ForwardPort: 80,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var httpApp HTTPApp
	if err := json.Unmarshal(cfg.Apps["http"], &httpApp); err != nil {
		t.Fatal(err)
	}
	https := httpApp.Servers["https"]
	if https == nil || len(https.Listen) != 1 || https.Listen[0] != ":443" {
		t.Fatalf("https server must listen on :443, got %+v", https)
	}
	if len(https.ListenerWrappers) != 0 {
		t.Fatalf("no listener wrappers expected without passthrough, got %+v", https.ListenerWrappers)
	}
}
