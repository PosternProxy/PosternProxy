//go:build integration

package services_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/api"
	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/database"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/security"
	"github.com/posternproxy/posternproxy/internal/services"
)

type acceptanceController struct {
	db          *sql.DB
	hub         *agent.Hub
	sync        *services.CaddySyncService
	hosts       *services.ProxyHostService
	accessLists *services.AccessListService
	server      *httptest.Server
	stop        func()
}

func startAcceptanceController(t *testing.T, path, address, localAPI string) *acceptanceController {
	t.Helper()
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{2, 3} {
		hash, err := security.BcryptHash("disposable-agent-secret")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT OR IGNORE INTO servers(id,name,hostname,agent_token_hash) VALUES (?,?,?,?)`, id, "test node", "127.0.0.1", hash); err != nil {
			t.Fatal(err)
		}
	}
	var syncSvc *services.CaddySyncService
	hub := agent.NewHub(nil, func(id int64) {
		if err := syncSvc.Reconnect(context.Background(), id); err != nil {
			t.Errorf("reconnect failed: %v", err)
		}
	})
	client := caddy.NewClient(localAPI)
	syncSvc = services.NewCaddySyncService(db, client, hub, "")
	audit := services.NewAuditService(db)
	tokens := services.NewAPITokenService(db)
	serverSvc := services.NewServerService(db, audit, hub, tokens, client, [32]byte{}, "", "")
	router := api.NewRouter(api.RouterConfig{DB: db, Hub: hub, ServerService: serverSvc, AuditService: audit, APITokenService: tokens, CaddySyncService: syncSvc})
	server := httptest.NewUnstartedServer(router)
	if address != "" {
		server.Listener.Close()
		listener, err := net.Listen("tcp", address)
		if err != nil {
			t.Fatal(err)
		}
		server.Listener = listener
	}
	server.Start()
	if err := syncSvc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	var stopOnce sync.Once
	stop := func() { stopOnce.Do(func() { syncSvc.Close(); server.Close(); db.Close() }) }
	t.Cleanup(stop)
	return &acceptanceController{db: db, hub: hub, sync: syncSvc, hosts: services.NewProxyHostService(db, client, audit, nil, syncSvc), accessLists: services.NewAccessListService(db, audit, syncSvc), server: server, stop: stop}
}

func startAcceptanceProcess(t *testing.T, binary string, args, environment []string) func() {
	t.Helper()
	logFile, err := os.Create(filepath.Join(t.TempDir(), "process.log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { logFile.Close() })
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), environment...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			if t.Failed() {
				data, err := os.ReadFile(logFile.Name())
				if err == nil {
					t.Log(string(data))
				}
			}
		})
	}
	t.Cleanup(stop)
	return stop
}

func acceptancePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func eventuallyAcceptance(t *testing.T, description string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal(description)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func acceptanceCaddyNode(t *testing.T, binary string) (int, string, func()) {
	t.Helper()
	dir := t.TempDir()
	adminAddress := "127.0.0.1:" + strconv.Itoa(acceptancePort(t))
	httpPort := acceptancePort(t)
	bootstrap, err := json.Marshal(caddy.Config{Admin: &caddy.AdminConfig{Listen: adminAddress}, Apps: map[string]json.RawMessage{}})
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "bootstrap.json")
	if err := os.WriteFile(configPath, bootstrap, 0600); err != nil {
		t.Fatal(err)
	}
	startCaddy := func() func() {
		return startAcceptanceProcess(t, binary, []string{"run", "--config", configPath}, []string{"XDG_DATA_HOME=" + dir, "XDG_CONFIG_HOME=" + dir})
	}
	stopCaddy := startCaddy()
	client := &http.Client{Timeout: time.Second}
	eventuallyAcceptance(t, "Caddy did not start", func() bool {
		response, err := client.Get("http://" + adminAddress + "/config/")
		if err != nil {
			return false
		}
		response.Body.Close()
		return response.StatusCode == 200
	})
	// Each node gets isolated listener ports. All generated routes/handlers are
	// retained and accepted by the real pinned Caddy process behind this shim.
	shim := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body io.Reader = r.Body
		if r.Method == "POST" && r.URL.Path == "/load" {
			var cfg caddy.Config
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, "invalid fixture config", 500)
				return
			}
			var app caddy.HTTPApp
			if err := json.Unmarshal(cfg.Apps["http"], &app); err != nil {
				http.Error(w, "invalid fixture app", 500)
				return
			}
			app.Servers["http"].Listen = []string{"127.0.0.1:" + strconv.Itoa(httpPort)}
			app.Servers["http"].AutomaticHTTPS = &caddy.AutoHTTPS{Disable: true}
			delete(app.Servers, "https")
			httpJSON, err := json.Marshal(app)
			if err != nil {
				http.Error(w, "marshal fixture app", 500)
				return
			}
			cfg.Apps["http"] = httpJSON
			cfg.Admin.Listen = adminAddress
			encoded, err := json.Marshal(cfg)
			if err != nil {
				http.Error(w, "marshal fixture config", 500)
				return
			}
			body = bytes.NewReader(encoded)
		}
		request, err := http.NewRequestWithContext(r.Context(), r.Method, "http://"+adminAddress+r.URL.RequestURI(), body)
		if err != nil {
			http.Error(w, "fixture request", 500)
			return
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			http.Error(w, "fixture Caddy unavailable", 502)
			return
		}
		defer response.Body.Close()
		for key, values := range response.Header {
			w.Header()[key] = values
		}
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	t.Cleanup(shim.Close)
	return httpPort, shim.URL, func() { stopCaddy(); stopCaddy = startCaddy() }
}

func TestTwoAgentsConvergeAfterControllerRestart(t *testing.T) {
	caddyBinary, agentBinary := os.Getenv("POSTERNPROXY_TEST_CADDY"), os.Getenv("POSTERNPROXY_TEST_AGENT")
	if caddyBinary == "" || agentBinary == "" {
		t.Fatal("set POSTERNPROXY_TEST_CADDY and POSTERNPROXY_TEST_AGENT to native test binaries")
	}
	portA, apiA, restartA := acceptanceCaddyNode(t, caddyBinary)
	portB, apiB, _ := acceptanceCaddyNode(t, caddyBinary)
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer local.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "upstream:"+r.URL.Path) }))
	defer upstream.Close()
	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	upstreamPort, err := strconv.Atoi(upstreamURL.Port())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "controller.db")
	controller := startAcceptanceController(t, path, "", local.URL)
	startAgent := func(id int64, apiURL string) func() {
		return startAcceptanceProcess(t, agentBinary, nil, []string{"POSTERNPROXY_CONTROLLER_URL=" + controller.server.URL, "POSTERNPROXY_AGENT_TOKEN=disposable-agent-secret", "POSTERNPROXY_SERVER_ID=" + strconv.FormatInt(id, 10), "POSTERNPROXY_CADDY_API=" + apiURL})
	}
	stopA, stopB := startAgent(2, apiA), startAgent(3, apiB)
	accessList, err := controller.accessLists.Create(context.Background(), nil, services.CreateAccessListInput{Name: "node A", Clients: []services.AccessListClientInput{{Address: "127.0.0.1/32", Directive: "allow"}}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	waitApplied := func(id int64) {
		eventuallyAcceptance(t, "agent did not apply latest revision", func() bool {
			state, err := controller.sync.State(context.Background(), id)
			return err == nil && state.Status == "applied" && state.AppliedRevision == state.DesiredRevision
		})
	}
	input := func(id int64, domain string) models.CreateProxyHostInput {
		host := models.CreateProxyHostInput{ServerID: id, HTTPOnly: true, DomainNames: json.RawMessage(`["` + domain + `"]`), ForwardScheme: "http", ForwardHost: upstreamURL.Hostname(), ForwardPort: upstreamPort}
		if id == 2 {
			host.AccessListID = &accessList.ID
		}
		return host
	}
	a, err := controller.hosts.Create(context.Background(), nil, input(2, "node-a.test"), "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = controller.hosts.Create(context.Background(), nil, input(3, "node-b.test"), "test")
	if err != nil {
		t.Fatal(err)
	}
	waitApplied(2)
	waitApplied(3)
	traffic := func(port int, domain string, want int) {
		t.Helper()
		client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		request, err := http.NewRequest("GET", "http://127.0.0.1:"+strconv.Itoa(port)+"/check", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Host = domain
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want || (want == 200 && string(body) != "upstream:/check") {
			t.Fatalf("traffic: %d %q", response.StatusCode, body)
		}
	}
	traffic(portA, "node-a.test", 200)
	traffic(portB, "node-b.test", 200)
	restartA()
	if _, err := controller.db.Exec(`UPDATE server_config_state SET updated_at=datetime('now','-61 seconds') WHERE server_id=2`); err != nil {
		t.Fatal(err)
	}
	eventuallyAcceptance(t, "Caddy restart was not reconciled", func() bool {
		client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		request, err := http.NewRequest("GET", "http://127.0.0.1:"+strconv.Itoa(portA)+"/check", nil)
		if err != nil {
			return false
		}
		request.Host = "node-a.test"
		response, err := client.Do(request)
		if err != nil {
			return false
		}
		response.Body.Close()
		return response.StatusCode == 200
	})
	waitApplied(2)
	bState, err := controller.sync.State(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range []struct {
		directive string
		status    int
	}{{"deny", 403}, {"allow", 200}} {
		if _, err := controller.accessLists.Update(context.Background(), nil, accessList.ID, services.CreateAccessListInput{Name: "node A", Clients: []services.AccessListClientInput{{Address: "127.0.0.1/32", Directive: rule.directive}}}, "test"); err != nil {
			t.Fatal(err)
		}
		waitApplied(2)
		traffic(portA, "node-a.test", rule.status)
		unchanged, err := controller.sync.State(context.Background(), 3)
		if err != nil || unchanged.DesiredRevision != bState.DesiredRevision {
			t.Fatalf("unrelated server dirtied: %+v %v", unchanged, err)
		}
	}
	stopA()
	stopB()
	eventuallyAcceptance(t, "agents did not disconnect", func() bool { return !controller.hub.IsOnline(2) && !controller.hub.IsOnline(3) })
	if _, err := controller.hosts.Update(context.Background(), nil, a.ID, input(2, "changed-offline.test"), "test"); err != nil {
		t.Fatal(err)
	}
	state, err := controller.sync.State(context.Background(), 2)
	if err != nil || state.Status != "queued" {
		t.Fatalf("offline change not queued: %+v %v", state, err)
	}
	address := controller.server.Listener.Addr().String()
	controller.stop()
	controller = startAcceptanceController(t, path, address, local.URL)
	startAgent(2, apiA)
	startAgent(3, apiB)
	waitApplied(2)
	waitApplied(3)
	traffic(portA, "changed-offline.test", 200)
	traffic(portA, "node-a.test", 301)
	traffic(portB, "node-b.test", 200)
	if err := controller.hosts.Delete(context.Background(), nil, a.ID, "test"); err != nil {
		t.Fatal(err)
	}
	waitApplied(2)
	traffic(portA, "changed-offline.test", 301)
	traffic(portB, "node-b.test", 200)
}
