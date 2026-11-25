package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/posternproxy/posternproxy/internal/api"
	"github.com/posternproxy/posternproxy/internal/auth"
	"github.com/posternproxy/posternproxy/internal/database"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

type authFixture struct {
	router   http.Handler
	users    *services.UserService
	tokens   *services.APITokenService
	sessions *services.SessionService
	admin    *models.User
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	audit := services.NewAuditService(db)
	users := services.NewUserService(db, audit)
	admin, err := users.Create(context.Background(), nil, models.CreateUserInput{Email: "admin@test.invalid", Name: "Admin", Password: "test-password", Role: "admin"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	tokens := services.NewAPITokenService(db)
	sessions := services.NewSessionService(db, "test-secret")
	router := api.NewRouter(api.RouterConfig{DB: db, JWTSecret: "test-secret", UserService: users, AuditService: audit, APITokenService: tokens, SessionService: sessions})
	return &authFixture{router: router, users: users, tokens: tokens, sessions: sessions, admin: admin}
}

func (f *authFixture) request(method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.1:1234"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", "test-csrf")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test-csrf"})
	req.Header.Set("X-Token-Scope", "read,write,admin")
	rr := httptest.NewRecorder()
	f.router.ServeHTTP(rr, req)
	return rr
}

func TestReadTokenCannotMutateThroughRouter(t *testing.T) {
	f := newAuthFixture(t)
	token, err := f.tokens.Create(context.Background(), f.admin.ID, "admin", models.CreateAPITokenInput{Name: "read-only", Scopes: []string{"read"}})
	if err != nil {
		t.Fatal(err)
	}
	if rr := f.request("GET", "/api/tokens", token.Token, ""); rr.Code != 200 {
		t.Fatalf("read: %d %s", rr.Code, rr.Body)
	}
	paths := []struct{ method, path string }{
		{"POST", "/api/tokens"}, {"POST", "/api/proxy-hosts"}, {"PUT", "/api/proxy-hosts/1"}, {"DELETE", "/api/proxy-hosts/1"},
		{"POST", "/api/proxy-hosts/1/disable"}, {"POST", "/api/redirection-hosts"}, {"POST", "/api/dead-hosts"}, {"POST", "/api/stream-hosts"},
		{"PUT", "/api/access-lists/1"}, {"POST", "/api/certificates/upload"}, {"POST", "/api/servers/2/provision"}, {"POST", "/api/servers/2/resync"},
		{"POST", "/api/servers/2/join-token"}, {"POST", "/api/servers/2/reset-join"}, {"POST", "/api/config-history/1/rollback"},
		{"POST", "/api/backup/import"}, {"POST", "/api/backup/export"}, {"POST", "/api/backup/restore"}, {"POST", "/api/servers/2/agent-credentials"}, {"POST", "/api/import/nginx/preview"}, {"POST", "/api/import/nginx/confirm"}, {"PUT", "/api/settings"}, {"POST", "/api/users"},
	}
	for _, tc := range paths {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			rr := f.request(tc.method, tc.path, token.Token, `{"name":"stronger","scopes":["read","write"]}`)
			if rr.Code != 403 || !strings.Contains(rr.Body.String(), "insufficient token scope") {
				t.Fatalf("%d %s", rr.Code, rr.Body)
			}
		})
	}
	write, err := f.tokens.Create(context.Background(), f.admin.ID, "admin", models.CreateAPITokenInput{Name: "write", Scopes: []string{"read", "write"}})
	if err != nil {
		t.Fatal(err)
	}
	if rr := f.request("POST", "/api/tokens", write.Token, `{"name":"child"}`); rr.Code != 403 {
		t.Fatalf("token minting: %d %s", rr.Code, rr.Body)
	}
}

func TestAuthorityChangesRevokeExistingCredentials(t *testing.T) {
	for _, change := range []string{"demote", "disable", "delete", "password"} {
		t.Run(change, func(t *testing.T) {
			f := newAuthFixture(t)
			token, err := f.tokens.Create(context.Background(), f.admin.ID, "admin", models.CreateAPITokenInput{Name: "old"})
			if err != nil {
				t.Fatal(err)
			}
			pair, err := f.sessions.Issue(context.Background(), f.admin)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "demote":
				role := "viewer"
				_, err = f.users.Update(context.Background(), nil, f.admin.ID, models.UpdateUserInput{Role: &role}, "test")
			case "disable":
				active := false
				_, err = f.users.Update(context.Background(), nil, f.admin.ID, models.UpdateUserInput{IsActive: &active}, "test")
			case "delete":
				err = f.users.Delete(context.Background(), nil, f.admin.ID, "test")
			case "password":
				err = f.users.ChangePassword(context.Background(), nil, f.admin.ID, "replacement-password", "test")
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, raw := range []string{token.Token, pair.AccessToken} {
				if rr := f.request("GET", "/api/tokens", raw, ""); rr.Code != 401 {
					t.Fatalf("stale authority accepted: %d %s", rr.Code, rr.Body)
				}
			}
			if _, err = f.sessions.Refresh(context.Background(), pair.RefreshToken); err == nil {
				t.Fatal("refresh survived account change")
			}
		})
	}
}

func TestSessionRotationAndLogoutThroughRouter(t *testing.T) {
	f := newAuthFixture(t)
	rr := f.request("POST", "/api/auth/login", "", `{"email":"admin@test.invalid","password":"test-password"}`)
	if rr.Code != 201 {
		t.Fatalf("login: %d %s", rr.Code, rr.Body)
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	var cookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "refresh_token" {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly {
		t.Fatal("missing HttpOnly refresh cookie")
	}
	refresh := func(raw string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/auth/refresh", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: raw})
		res := httptest.NewRecorder()
		f.router.ServeHTTP(res, req)
		return res
	}
	rotated := refresh(cookie.Value)
	if rotated.Code != 201 {
		t.Fatalf("refresh: %d %s", rotated.Code, rotated.Body)
	}
	if replay := refresh(cookie.Value); replay.Code != 401 {
		t.Fatalf("replayed refresh: %d", replay.Code)
	}
	if logout := f.request("POST", "/api/auth/logout", body.AccessToken, ""); logout.Code != 200 {
		t.Fatalf("logout: %d %s", logout.Code, logout.Body)
	}
	if res := f.request("GET", "/api/auth/me", body.AccessToken, ""); res.Code != 401 {
		t.Fatalf("access survived logout: %d", res.Code)
	}
	for _, c := range rotated.Result().Cookies() {
		if c.Name == "refresh_token" && refresh(c.Value).Code != 401 {
			t.Fatal("refresh survived logout")
		}
	}
	legacy, err := auth.GenerateTokenPair("test-secret", f.admin.ID, f.admin.Email, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if res := f.request("GET", "/api/auth/me", legacy.AccessToken, ""); res.Code != 401 {
		t.Fatal("sessionless JWT accepted")
	}
}

func TestAuditLogRejectsNonAdmin(t *testing.T) {
	f := newAuthFixture(t)
	viewer, err := f.users.Create(context.Background(), nil, models.CreateUserInput{Email: "viewer@test.invalid", Name: "Viewer", Password: "test-password", Role: "viewer"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	pair, err := f.sessions.Issue(context.Background(), viewer)
	if err != nil {
		t.Fatal(err)
	}
	if response := f.request("GET", "/api/audit-log", pair.AccessToken, ""); response.Code != 403 {
		t.Fatalf("viewer audit access: %d", response.Code)
	}
	admin, err := f.sessions.Issue(context.Background(), f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if response := f.request("GET", "/api/audit-log", admin.AccessToken, ""); response.Code != 200 {
		t.Fatalf("admin audit access: %d %s", response.Code, response.Body)
	}
}
