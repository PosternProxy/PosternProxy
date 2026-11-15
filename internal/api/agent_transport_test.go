package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
