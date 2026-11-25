//go:build integration

package caddy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/posternproxy/posternproxy/internal/models"
)

func TestPinnedCaddyClientIdentityAndStreaming(t *testing.T) {
	binary := os.Getenv("POSTERNPROXY_TEST_CADDY")
	if binary == "" {
		t.Fatal("set POSTERNPROXY_TEST_CADDY to the pinned native Caddy binary")
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/upload":
			n, err := io.Copy(io.Discard, r.Body)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			fmt.Fprint(w, n)
		case "/events":
			w.Header().Set("Content-Type", "text/event-stream")
			io.WriteString(w, "data: first\n\n")
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-time.After(3 * time.Second):
				io.WriteString(w, "data: last\n\n")
			}
		case "/ws":
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			kind, data, err := conn.ReadMessage()
			if err == nil {
				conn.WriteMessage(kind, data)
			}
		default:
			fmt.Fprintf(w, "%s|%s", r.Header.Get("X-Real-IP"), r.Header.Get("X-Forwarded-For"))
		}
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	port, _ := strconv.Atoi(u.Port())
	proxyPort, adminPort := freeTCPPort(t), freeTCPPort(t)
	endpoint := "http://127.0.0.1:" + strconv.Itoa(proxyPort)
	admin := "127.0.0.1:" + strconv.Itoa(adminPort)
	makeConfig := func(trust []string, acl bool, rate int) *Config {
		h := models.ProxyHost{ID: 1, CaddyID: "identity", Enabled: true, HTTPOnly: true,
			DomainNames: json.RawMessage(`["127.0.0.1"]`), ForwardScheme: "http", ForwardHost: u.Hostname(), ForwardPort: port,
			RateLimitEnabled: rate > 0, RateLimitRequests: rate, RateLimitWindow: "1m",
			Locations: json.RawMessage(fmt.Sprintf(`[{"path":"/location","forward_scheme":"http","forward_host":"%s","forward_port":%d}]`, u.Hostname(), port)),
		}
		in := &BuildInput{TrustedProxies: trust, ProxyHosts: []models.ProxyHost{h}}
		if acl {
			id := int64(7)
			in.ProxyHosts[0].AccessListID = &id
			in.AccessLists = []models.AccessList{{ID: id, Clients: []models.AccessListClient{{Address: "198.51.100.7", Directive: "allow"}}}}
		}
		cfg, err := BuildFullConfig(in)
		if err != nil {
			t.Fatal(err)
		}
		var app HTTPApp
		if err := json.Unmarshal(cfg.Apps["http"], &app); err != nil {
			t.Fatal(err)
		}
		delete(app.Servers, "https")
		app.Servers["http"].Listen = []string{"127.0.0.1:" + strconv.Itoa(proxyPort)}
		app.Servers["http"].AutomaticHTTPS = &AutoHTTPS{Disable: true}
		cfg.Apps["http"], err = json.Marshal(app)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Admin.Listen = admin
		return cfg
	}
	dir := t.TempDir()
	raw, err := json.Marshal(makeConfig(nil, false, 0))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "caddy.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	logFile, err := os.Create(filepath.Join(dir, "caddy.log"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "run", "--config", path)
	cmd.Env = append(os.Environ(), "XDG_DATA_HOME="+dir, "XDG_CONFIG_HOME="+dir)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		cmd.Wait()
		logFile.Close()
		if t.Failed() {
			data, _ := os.ReadFile(logFile.Name())
			t.Log(string(data))
		}
	})
	api := NewClient("http://" + admin)
	for i := 0; ; i++ {
		if err := api.Ping(); err == nil {
			break
		}
		if i == 100 {
			t.Fatal("Caddy failed to start")
		}
		time.Sleep(50 * time.Millisecond)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	request := func(t *testing.T, path, xff string, wantStatus int, wantBody string) {
		t.Helper()
		req, _ := http.NewRequestWithContext(ctx, "GET", endpoint+path, nil)
		req.Header.Set("X-Forwarded-For", xff)
		req.Header.Set("X-Real-IP", "192.0.2.99")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != wantStatus || (wantBody != "" && string(body) != wantBody) {
			t.Fatalf("XFF=%q: got %d %q (%v), want %d %q", xff, resp.StatusCode, body, err, wantStatus, wantBody)
		}
	}
	load := func(trust []string, acl bool, rate int) {
		t.Helper()
		if err := api.Load(makeConfig(trust, acl, rate)); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("direct ignores spoofed headers", func(t *testing.T) {
		request(t, "/", "198.51.100.7", 200, "127.0.0.1|127.0.0.1")
		load([]string{"192.0.2.1/32"}, true, 0)
		request(t, "/", "198.51.100.7", 403, "")
	})
	trust := []string{"127.0.0.1/32", "192.0.2.1/32"}
	t.Run("trusted chain and location forwarding", func(t *testing.T) {
		load(trust, false, 0)
		for _, path := range []string{"/", "/location/check"} {
			for _, xff := range []string{"198.51.100.7", "203.0.113.99, 198.51.100.7, 192.0.2.1", "bad, 198.51.100.7, 192.0.2.1"} {
				request(t, path, xff, 200, "198.51.100.7|198.51.100.7")
			}
		}
		request(t, "/", "2001:db8::7, 192.0.2.1", 200, "2001:db8::7|2001:db8::7")
		request(t, "/", "bad", 200, "127.0.0.1|127.0.0.1")
	})
	t.Run("ACL uses visitor", func(t *testing.T) {
		load(trust, true, 0)
		request(t, "/", "198.51.100.7, 192.0.2.1", 200, "198.51.100.7|198.51.100.7")
		request(t, "/", "198.51.100.7, 203.0.113.8", 403, "")
		request(t, "/", "malformed", 403, "")
	})
	t.Run("rate keys isolate visitors", func(t *testing.T) {
		load(trust, false, 1)
		request(t, "/", "198.51.100.11", 200, "")
		request(t, "/", "198.51.100.11", 429, "")
		request(t, "/", "198.51.100.12", 200, "")
	})
	load(nil, false, 0)
	t.Run("upload and header bounds", func(t *testing.T) {
		resp, err := client.Post(endpoint+"/upload", "application/octet-stream", strings.NewReader(strings.Repeat("a", 2<<20)))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || string(body) != "2097152" {
			t.Fatalf("upload: %d %q", resp.StatusCode, body)
		}
		req, _ := http.NewRequest("GET", endpoint, nil)
		req.Header.Set("X-Large", strings.Repeat("a", (1<<20)+8192))
		resp, err = client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 431 {
			t.Fatalf("oversized headers: %d", resp.StatusCode)
		}
	})
	t.Run("SSE flushes before response completes", func(t *testing.T) {
		started := time.Now()
		resp, err := client.Get(endpoint + "/events")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		line, err := bufio.NewReader(resp.Body).ReadString('\n')
		if err != nil || line != "data: first\n" || time.Since(started) >= 2*time.Second {
			t.Fatalf("SSE buffered: %q %v", line, err)
		}
	})
	t.Run("WebSocket echo", func(t *testing.T) {
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(endpoint, "http")+"/ws", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, []byte("echo")); err != nil {
			t.Fatal(err)
		}
		_, data, err := conn.ReadMessage()
		if err != nil || string(data) != "echo" {
			t.Fatalf("WebSocket: %q %v", data, err)
		}
	})
}
