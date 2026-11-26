package api_test

import (
	"context"
	"github.com/posternproxy/posternproxy/internal/models"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/api"
	"github.com/posternproxy/posternproxy/internal/database"
	"github.com/posternproxy/posternproxy/internal/security"
	"github.com/posternproxy/posternproxy/internal/services"
)

func TestAgentHeaderAuthentication(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "transport.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hash, err := security.BcryptHash("transport-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	res, err := db.Exec(`INSERT INTO servers (name,hostname,agent_token_hash) VALUES ('remote','remote.test',?)`, hash)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	hub := agent.NewHub(nil, nil)
	svc := services.NewServerService(db, services.NewAuditService(db), hub, services.NewAPITokenService(db), nil, [32]byte{}, "", "")
	server := httptest.NewServer(api.NewRouter(api.RouterConfig{DB: db, ServerService: svc, Hub: hub}))
	defer server.Close()
	endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/agent?server_id=" + strconv.FormatInt(id, 10)
	for _, tc := range []struct {
		name, query, token string
		want               bool
	}{
		{"header", "", "transport-test-secret", true}, {"missing", "", "", false}, {"invalid", "", "wrong", false},
		{"query only", "&token=transport-test-secret", "", false}, {"conflicting", "&token=transport-test-secret", "transport-test-secret", false},
		{"wrong server", "0", "transport-test-secret", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := http.Header{}
			if tc.token != "" {
				h.Set("Authorization", "Bearer "+tc.token)
			}
			verifyURL := server.URL + "/agent/verify?server_id=" + strconv.FormatInt(id, 10) + tc.query
			request, err := http.NewRequest(http.MethodGet, verifyURL, nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header = h.Clone()
			verification, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			verification.Body.Close()
			expectedStatus := http.StatusUnauthorized
			if tc.want {
				expectedStatus = http.StatusNoContent
			}
			if verification.StatusCode != expectedStatus {
				t.Fatalf("agent identity preflight: status %d, want %d", verification.StatusCode, expectedStatus)
			}
			conn, resp, err := websocket.DefaultDialer.Dial(endpoint+tc.query, h)
			if tc.want {
				if err != nil {
					t.Fatal(err)
				}
				conn.Close()
			} else {
				if conn != nil {
					conn.Close()
				}
				if err == nil || resp == nil || resp.StatusCode != 401 {
					t.Fatalf("unexpected handshake: %v %v", err, resp)
				}
			}
			if resp != nil {
				resp.Body.Close()
			}
		})
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE servers SET agent_token_hash='' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(endpoint, http.Header{"Authorization": {"Bearer transport-test-secret"}})
	if conn != nil {
		conn.Close()
	}
	if err == nil || resp == nil || resp.StatusCode != 401 {
		t.Fatal("revoked token accepted")
	}
	if resp != nil {
		resp.Body.Close()
	}
}

func TestAgentRevocationThroughAPI(t *testing.T) {
	for _, change := range []string{"revoke", "disable", "password", "role", "rotate"} {
		t.Run(change, func(t *testing.T) {
			ctx := context.Background()
			db, err := database.Open(filepath.Join(t.TempDir(), "revoke.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			audit := services.NewAuditService(db)
			users := services.NewUserService(db, audit)
			owner, err := users.Create(ctx, nil, models.CreateUserInput{Email: "owner@test.invalid", Name: "Owner", Password: "test-password", Role: "admin"}, "test")
			if err != nil {
				t.Fatal(err)
			}
			result, err := db.Exec(`INSERT INTO servers(name,hostname) VALUES ('remote','remote.test')`)
			if err != nil {
				t.Fatal(err)
			}
			id, err := result.LastInsertId()
			if err != nil {
				t.Fatal(err)
			}
			tokens := services.NewAPITokenService(db)
			raw, err := tokens.CreateForServer(ctx, id, &owner.ID)
			if err != nil {
				t.Fatal(err)
			}
			hub := agent.NewHub(nil, nil)
			svc := services.NewServerService(db, audit, hub, tokens, nil, [32]byte{}, "", "")
			router := api.NewRouter(api.RouterConfig{DB: db, JWTSecret: "test-secret", UserService: users, AuditService: audit, APITokenService: tokens, SessionService: services.NewSessionService(db, "test-secret"), ServerService: svc, Hub: hub})
			server := httptest.NewServer(router)
			defer server.Close()
			endpoint := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/agent?server_id=" + strconv.FormatInt(id, 10)
			headers := http.Header{"Authorization": {"Bearer " + raw}}
			conn, resp, err := websocket.DefaultDialer.Dial(endpoint, headers)
			if resp != nil {
				resp.Body.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			switch change {
			case "revoke":
				operator, err := tokens.Create(ctx, owner.ID, "admin", models.CreateAPITokenInput{Name: "operator", Scopes: []string{"write"}})
				if err != nil {
					t.Fatal(err)
				}
				var tokenID int64
				if err := db.QueryRow(`SELECT id FROM api_tokens WHERE server_id=? AND revoked_at IS NULL`, id).Scan(&tokenID); err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest("DELETE", "/api/tokens/"+strconv.FormatInt(tokenID, 10), nil)
				req.Header.Set("Authorization", "Bearer "+operator.Token)
				req.Header.Set("X-CSRF-Token", "test-csrf")
				req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "test-csrf"})
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)
				if rr.Code != http.StatusNoContent {
					t.Fatalf("revoke: %d %s", rr.Code, rr.Body)
				}
			case "disable":
				_, err = db.Exec(`UPDATE users SET is_active=0 WHERE id=?`, owner.ID)
			case "password":
				_, err = db.Exec(`UPDATE users SET password_hash='changed' WHERE id=?`, owner.ID)
			case "role":
				_, err = db.Exec(`UPDATE users SET role='viewer' WHERE id=?`, owner.ID)
			case "rotate":
				var replacement string
				replacement, err = tokens.CreateForServer(ctx, id, &owner.ID)
				if err == nil && !svc.ValidateAgentToken(ctx, id, replacement) {
					t.Fatal("rotation revoked replacement")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if svc.ValidateAgentToken(ctx, id, raw) {
				t.Fatal("revoked credential still valid")
			}
			request, err := http.NewRequest("GET", server.URL+"/agent/verify?server_id="+strconv.FormatInt(id, 10), nil)
			if err != nil {
				t.Fatal(err)
			}
			request.Header = headers
			verification, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			verification.Body.Close()
			if verification.StatusCode != http.StatusUnauthorized {
				t.Fatalf("revoked preflight: %d", verification.StatusCode)
			}
			reconnected, resp, err := websocket.DefaultDialer.Dial(endpoint, headers)
			if reconnected != nil {
				reconnected.Close()
			}
			if resp != nil {
				defer resp.Body.Close()
			}
			if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
				t.Fatal("revoked connection accepted")
			}
			conn.SetReadDeadline(time.Now().Add(4 * time.Second))
			_, _, err = conn.ReadMessage()
			if err == nil {
				t.Fatal("revoked connection still receiving")
			}
			if timeout, ok := err.(interface{ Timeout() bool }); ok && timeout.Timeout() {
				t.Fatal("revoked connection was not closed")
			}
		})
	}
}
