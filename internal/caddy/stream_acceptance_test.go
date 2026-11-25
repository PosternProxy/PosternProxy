//go:build integration

package caddy

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/posternproxy/posternproxy/internal/caddybuild"
	"github.com/posternproxy/posternproxy/internal/models"
)

// requirePinnedCaddyBinary returns the pinned native Caddy binary from the
// environment and verifies it embeds the exact pinned Caddy/L4/rate-limit module
// versions, so acceptance runs cannot silently use an unpinned build.
func requirePinnedCaddyBinary(t *testing.T) string {
	t.Helper()
	binary := os.Getenv("POSTERNPROXY_TEST_CADDY")
	if binary == "" {
		t.Fatal("set POSTERNPROXY_TEST_CADDY to the pinned native Caddy binary")
	}
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	pin := func(name string) string {
		match := regexp.MustCompile(`(?m)^` + name + `=([^\r\n]+)`).FindStringSubmatch(caddybuild.Script)
		if len(match) != 2 {
			t.Fatalf("missing pin %s", name)
		}
		return match[1]
	}
	for module, version := range map[string]string{
		"github.com/caddyserver/caddy/v2":  pin("CADDY_VERSION"),
		"github.com/mholt/caddy-l4":        pin("CADDY_L4_VERSION"),
		"github.com/mholt/caddy-ratelimit": pin("CADDY_RATELIMIT_VERSION"),
	} {
		found := false
		for _, dep := range info.Deps {
			if dep.Path == module && dep.Version == version && dep.Replace == nil {
				found = true
			}
		}
		if !found {
			t.Fatalf("Caddy binary does not contain %s@%s", module, version)
		}
	}
	return binary
}

// launchCaddy writes cfg to a temp file and runs the pinned Caddy binary,
// registering cleanup that cancels the process and dumps logs on failure.
func launchCaddy(t *testing.T, binary string, cfg *Config) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "caddy.json")
	configBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, configBytes, 0600); err != nil {
		t.Fatal(err)
	}
	logFile, err := os.Create(filepath.Join(dir, "caddy.log"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	cmd := exec.CommandContext(ctx, binary, "run", "--config", configPath)
	cmd.Env = append(os.Environ(), "XDG_DATA_HOME="+dir, "XDG_CONFIG_HOME="+dir)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
		if err := logFile.Close(); err != nil {
			t.Error(err)
		}
		if t.Failed() {
			if data, err := os.ReadFile(logFile.Name()); err == nil {
				t.Log(string(data))
			}
		}
	})
}

// neutralizeHTTP rewrites the generated HTTP app so it cannot bind privileged
// ports or trigger ACME during a local acceptance run. When keepHTTPS is false
// the :443 server is removed entirely; when true its listener is cleared (used
// by the TLS-passthrough fallthrough bridge) so the config stays valid.
func neutralizeHTTP(t *testing.T, cfg *Config, keepHTTPS bool) {
	t.Helper()
	raw, ok := cfg.Apps["http"]
	if !ok {
		return
	}
	var app HTTPApp
	if err := json.Unmarshal(raw, &app); err != nil {
		t.Fatal(err)
	}
	if s := app.Servers["http"]; s != nil {
		s.Listen = []string{"127.0.0.1:" + strconv.Itoa(freeTCPPort(t))}
		s.AutomaticHTTPS = &AutoHTTPS{Disable: true}
	}
	if keepHTTPS {
		if s := app.Servers["https"]; s != nil {
			s.Listen = []string{}
			s.AutomaticHTTPS = &AutoHTTPS{Disable: true}
		}
	} else {
		delete(app.Servers, "https")
	}
	b, err := json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Apps["http"] = b
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	port := c.LocalAddr().(*net.UDPAddr).Port
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

// TestPinnedCaddyUDPForwarding proves the pinned L4 build forwards real UDP
// datagrams from a generated stream host to the configured upstream.
func TestPinnedCaddyUDPForwarding(t *testing.T) {
	binary := requirePinnedCaddyBinary(t)

	// UDP echo upstream.
	upstream, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer upstream.Close()
	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := upstream.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = upstream.WriteToUDP(append([]byte("echo:"), buf[:n]...), addr)
		}
	}()
	upstreamPort := upstream.LocalAddr().(*net.UDPAddr).Port

	listenPort := freeUDPPort(t)
	cfg, err := BuildFullConfig(&BuildInput{
		StreamHosts: []models.StreamHost{{
			ID: 7, Enabled: true, Protocol: "udp",
			IncomingPort: listenPort, ForwardHost: "127.0.0.1", ForwardPort: upstreamPort,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	neutralizeHTTP(t, cfg, false)

	// Bind the UDP listener to loopback on our chosen free port.
	var l4 Layer4App
	if err := json.Unmarshal(cfg.Apps["layer4"], &l4); err != nil {
		t.Fatal(err)
	}
	l4.Servers["stream_7"].Listen = []string{fmt.Sprintf("udp/127.0.0.1:%d", listenPort)}
	cfg.Apps["layer4"], err = json.Marshal(l4)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Admin.Listen = "127.0.0.1:" + strconv.Itoa(freeTCPPort(t))

	launchCaddy(t, binary, cfg)

	// UDP has no connect handshake, so retry the echo until the listener is up.
	target := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: listenPort}
	var got string
	for range 100 {
		conn, err := net.DialUDP("udp", nil, target)
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
		if _, err := conn.Write([]byte("ping")); err != nil {
			conn.Close()
			time.Sleep(50 * time.Millisecond)
			continue
		}
		buf := make([]byte, 2048)
		n, err := conn.Read(buf)
		conn.Close()
		if err == nil {
			got = string(buf[:n])
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got != "echo:ping" {
		t.Fatalf("UDP forward failed, got %q", got)
	}
}

// TestPinnedCaddyTLSPassthrough proves a passthrough proxy host routes a raw TLS
// connection by SNI to the upstream without terminating: the client sees the
// upstream's certificate, not a Caddy-minted one.
func TestPinnedCaddyTLSPassthrough(t *testing.T) {
	binary := requirePinnedCaddyBinary(t)

	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "passthrough-upstream")
	}))
	defer upstream.Close()
	u, err := net.ResolveTCPAddr("tcp", upstream.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := BuildFullConfig(&BuildInput{
		ProxyHosts: []models.ProxyHost{{
			ID: 9, CaddyID: "acceptance_passthrough", Enabled: true, TLSPassthrough: true,
			DomainNames: json.RawMessage(`["passthrough.test"]`),
			ForwardHost: "127.0.0.1", ForwardPort: u.Port,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	neutralizeHTTP(t, cfg, false)

	// Bind the L4 :443 server to loopback on a free port.
	l4Port := freeTCPPort(t)
	var l4 Layer4App
	if err := json.Unmarshal(cfg.Apps["layer4"], &l4); err != nil {
		t.Fatal(err)
	}
	server := l4.Servers["tls_passthrough_443"]
	// Keep only the SNI-matched passthrough route; drop the generated fallthrough
	// (tls+http bridge) which would need a live terminating HTTP app not present
	// in this harness. The fallthrough path belongs to the broader HTTPS matrix.
	kept := server.Routes[:0]
	for _, r := range server.Routes {
		if len(r.Match) > 0 {
			kept = append(kept, r)
		}
	}
	server.Routes = kept
	server.Listen = []string{"127.0.0.1:" + strconv.Itoa(l4Port)}
	cfg.Apps["layer4"], err = json.Marshal(l4)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Admin.Listen = "127.0.0.1:" + strconv.Itoa(freeTCPPort(t))

	launchCaddy(t, binary, cfg)

	ready := false
	for range 100 {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(l4Port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("Caddy failed to bind the TLS passthrough listener")
	}

	// Send SNI "passthrough.test" so Caddy matches the passthrough route, but
	// verify the server presents the upstream's exact certificate. If Caddy had
	// terminated TLS instead of passing through, a different cert would appear.
	dialer := &net.Dialer{Timeout: time.Second}
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				raw, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:"+strconv.Itoa(l4Port))
				if err != nil {
					return nil, err
				}
				conn := tls.Client(raw, &tls.Config{
					ServerName:         "passthrough.test",
					InsecureSkipVerify: true, // hostname differs from the upstream cert; identity is checked below
					VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
						if len(rawCerts) == 0 || !bytes.Equal(rawCerts[0], upstream.Certificate().Raw) {
							return fmt.Errorf("server presented a non-upstream certificate; TLS was not passed through")
						}
						return nil
					},
				})
				if err := conn.HandshakeContext(ctx); err != nil {
					raw.Close()
					return nil, err
				}
				return conn, nil
			},
		},
	}
	resp, err := client.Get("https://passthrough.test/")
	if err != nil {
		t.Fatalf("passthrough request failed: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || string(body) != "passthrough-upstream" {
		t.Fatalf("TLS passthrough failed: %d %q", resp.StatusCode, body)
	}
}
