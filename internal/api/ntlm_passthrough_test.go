package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/posternproxy/posternproxy/internal/api"
	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/database"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

func TestNTLMCreateReportsSavedWhenModuleIsMissing(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("{\"error\":\"loading config: loading new config: loading module 'http_ntlm': unknown module: http.reverse_proxy.transport.http_ntlm\"}\n"))
	}))
	t.Cleanup(caddyServer.Close)
	audit := services.NewAuditService(db)
	users := services.NewUserService(db, audit)
	admin, err := users.Create(context.Background(), nil, models.CreateUserInput{Email: "admin@test.invalid", Name: "Admin", Password: "test-password", Role: "admin"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	tokens := services.NewAPITokenService(db)
	token, err := tokens.Create(context.Background(), admin.ID, "admin", models.CreateAPITokenInput{Name: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	client := caddy.NewClient(caddyServer.URL)
	sync := services.NewCaddySyncService(db, client, nil, "")
	hosts := services.NewProxyHostService(db, client, audit, nil, sync)
	router := api.NewRouter(api.RouterConfig{
		DB: db, JWTSecret: "test-secret", UserService: users, AuditService: audit,
		APITokenService: tokens, SessionService: services.NewSessionService(db, "test-secret"),
		ProxyHostService: hosts, CaddySyncService: sync,
	})
	body := `{"domain_names":["ntlm.example"],"forward_scheme":"http","forward_host":"127.0.0.1","forward_port":8080,"ntlm_enabled":true}`
	created := apiRequest(router, http.MethodPost, "/api/proxy-hosts", token.Token, body)
	if created.Code != http.StatusBadGateway {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	var saved struct {
		Error  string `json:"error"`
		Saved  bool   `json:"saved"`
		ID     int64  `json:"id"`
		Server int64  `json:"server_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.Saved || saved.ID == 0 || saved.Server != 1 || !strings.Contains(saved.Error, "Configuration saved, but server 1") || strings.Contains(saved.Error, "unknown module") {
		t.Fatalf("create body: %+v", saved)
	}
	got := apiRequest(router, http.MethodGet, "/api/proxy-hosts/"+strconv.FormatInt(saved.ID, 10), token.Token, "")
	if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"ntlm_enabled":true`) {
		t.Fatalf("get: %d %s", got.Code, got.Body)
	}
	states := apiRequest(router, http.MethodGet, "/api/configuration", token.Token, "")
	if states.Code != http.StatusOK || !strings.Contains(states.Body.String(), "Caddy rejected the configuration because the NTLM transport module is missing") || strings.Contains(states.Body.String(), "unknown module") {
		t.Fatalf("configuration: %d %s", states.Code, states.Body)
	}

	conflict := apiRequest(router, http.MethodPost, "/api/proxy-hosts", token.Token, `{"domain_names":["conflict.example"],"forward_scheme":"http","forward_host":"127.0.0.1","forward_port":8080,"ntlm_enabled":true,"locations":[{"path":"/x"}]}`)
	if conflict.Code != http.StatusBadRequest || strings.Contains(conflict.Body.String(), `"saved"`) || !strings.Contains(conflict.Body.String(), "NTLM passthrough cannot be combined with path locations") {
		t.Fatalf("conflict: %d %s", conflict.Code, conflict.Body)
	}
	listed := apiRequest(router, http.MethodGet, "/api/proxy-hosts", token.Token, "")
	if strings.Contains(listed.Body.String(), "conflict.example") {
		t.Fatalf("conflict host was stored: %s", listed.Body)
	}
}

func apiRequest(router http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "test-csrf")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test-csrf"})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}
