//go:build integration

package caddy

import (
	"context"
	"debug/buildinfo"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestPinnedCaddyTraffic(t *testing.T) {
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

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "pinned-caddy-upstream")
	}))
	defer upstream.Close()
	u, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	upstreamPort, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	streamPort := freeTCPPort(t)
	cfg, err := BuildFullConfig(&BuildInput{
		ProxyHosts: []models.ProxyHost{{
			ID: 1, CaddyID: "acceptance_proxy", Enabled: true, HTTPOnly: true,
			DomainNames:   json.RawMessage(`["acceptance.test"]`),
			ForwardScheme: "http", ForwardHost: u.Hostname(), ForwardPort: upstreamPort,
			RateLimitEnabled: true, RateLimitRequests: 2, RateLimitWindow: "1m",
		}},
		StreamHosts: []models.StreamHost{{ID: 2, Enabled: true, Protocol: "tcp", IncomingPort: streamPort, ForwardHost: u.Hostname(), ForwardPort: upstreamPort}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var app HTTPApp
	if err := json.Unmarshal(cfg.Apps["http"], &app); err != nil {
		t.Fatal(err)
	}
	httpPort := freeTCPPort(t)
	app.Servers["http"].Listen = []string{"127.0.0.1:" + strconv.Itoa(httpPort)}
	// Moving an HTTP fixture off port 80 would otherwise trigger automatic TLS.
	app.Servers["http"].AutomaticHTTPS = &AutoHTTPS{Disable: true}
	delete(app.Servers, "https")
	cfg.Apps["http"], err = json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}
	var l4 Layer4App
	if err := json.Unmarshal(cfg.Apps["layer4"], &l4); err != nil {
		t.Fatal(err)
	}
	l4.Servers["stream_2"].Listen = []string{"127.0.0.1:" + strconv.Itoa(streamPort)}
	cfg.Apps["layer4"], err = json.Marshal(l4)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Admin.Listen = "127.0.0.1:" + strconv.Itoa(freeTCPPort(t))
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
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--config", configPath)
	cmd.Env = append(os.Environ(), "XDG_DATA_HOME="+dir, "XDG_CONFIG_HOME="+dir)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
		if err := logFile.Close(); err != nil {
			t.Error(err)
		}
		if t.Failed() {
			data, err := os.ReadFile(logFile.Name())
			if err != nil {
				t.Error(err)
			} else {
				t.Log(string(data))
			}
		}
	})
	client := &http.Client{Timeout: time.Second}
	endpoint := "http://127.0.0.1:" + strconv.Itoa(httpPort)
	ready := false
	for range 100 {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(httpPort), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("Caddy failed to bind the HTTP listener")
	}
	for _, want := range []int{200, 200, 429} {
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = "acceptance.test"
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != want {
			t.Fatalf("HTTP status = %d, want %d", resp.StatusCode, want)
		}
		if want == 200 && string(body) != "pinned-caddy-upstream" {
			t.Fatalf("unexpected upstream body: %q", body)
		}
	}
	resp, err := client.Get("http://127.0.0.1:" + strconv.Itoa(streamPort))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || string(body) != "pinned-caddy-upstream" {
		t.Fatalf("TCP stream failed: %d %q", resp.StatusCode, body)
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}
