package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/models"
)

func TestAccessListsDoNotHoldTheOnlyConnection(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO access_lists (name) VALUES ('empty'),('assigned'); INSERT INTO access_list_clients (access_list_id,address,directive) VALUES (2,'192.0.2.1','allow')`); err != nil {
		t.Fatal(err)
	}
	s := NewCaddySyncService(db, nil, nil, "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	input, err := s.buildInput(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.AccessLists) != 2 || len(input.AccessLists[0].Clients) != 0 || len(input.AccessLists[1].Clients) != 1 {
		t.Fatalf("incorrect hydration: %+v", input.AccessLists)
	}
	cancel()
	if _, err := s.buildInput(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

type desiredHostMutator interface {
	SetEnabled(context.Context, *int64, int64, bool, string) error
	Delete(context.Context, *int64, int64, string) error
}

func TestDisableAndDeleteReportApplyFailuresForEveryHostType(t *testing.T) {
	cases := []struct {
		name, seed, enabled, count string
		service                    func(*sql.DB, *AuditService, *caddy.Client, *CaddySyncService) desiredHostMutator
	}{
		{"proxy", `INSERT INTO proxy_hosts(id,caddy_id,server_id,domain_names,forward_host,forward_port) VALUES (1,'p',1,'["p.test"]','127.0.0.1',8080)`, `SELECT enabled FROM proxy_hosts WHERE id=1`, `SELECT count(*) FROM proxy_hosts`, func(db *sql.DB, a *AuditService, c *caddy.Client, s *CaddySyncService) desiredHostMutator {
			return NewProxyHostService(db, c, a, nil, s)
		}},
		{"redirect", `INSERT INTO redirection_hosts(id,caddy_id,server_id,domain_names,forward_scheme,forward_domain_name) VALUES (1,'r',1,'["r.test"]','http','target.test')`, `SELECT enabled FROM redirection_hosts WHERE id=1`, `SELECT count(*) FROM redirection_hosts`, func(db *sql.DB, a *AuditService, c *caddy.Client, s *CaddySyncService) desiredHostMutator {
			return NewRedirectionHostService(db, c, a, s)
		}},
		{"dead", `INSERT INTO dead_hosts(id,caddy_id,server_id,domain_names) VALUES (1,'d',1,'["d.test"]')`, `SELECT enabled FROM dead_hosts WHERE id=1`, `SELECT count(*) FROM dead_hosts`, func(db *sql.DB, a *AuditService, c *caddy.Client, s *CaddySyncService) desiredHostMutator {
			return NewDeadHostService(db, c, a, s)
		}},
		{"stream", `INSERT INTO stream_hosts(id,caddy_id,server_id,incoming_port,forward_host,forward_port) VALUES (1,'s',1,18080,'127.0.0.1',8080)`, `SELECT enabled FROM stream_hosts WHERE id=1`, `SELECT count(*) FROM stream_hosts`, func(db *sql.DB, a *AuditService, c *caddy.Client, s *CaddySyncService) desiredHostMutator {
			return NewStreamHostService(db, c, a, s)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testDB(t)
			ctx := context.Background()
			if _, err := db.Exec(tc.seed); err != nil {
				t.Fatal(err)
			}
			var reject atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if reject.Load() {
					w.WriteHeader(400)
				} else {
					w.WriteHeader(200)
				}
			}))
			defer server.Close()
			client := caddy.NewClient(server.URL)
			s := NewCaddySyncService(db, client, nil, "")
			mutator := tc.service(db, NewAuditService(db), client, s)
			if err := s.SyncServer(ctx, 1); err != nil {
				t.Fatal(err)
			}
			before, err := s.State(ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			reject.Store(true)
			var apply *ConfigApplyError
			if err := mutator.SetEnabled(ctx, nil, 1, false, "test"); !errors.As(err, &apply) {
				t.Fatalf("disable false success: %v", err)
			}
			var enabled int
			if err := db.QueryRow(tc.enabled).Scan(&enabled); err != nil || enabled != 0 {
				t.Fatalf("desired disable missing: %d %v", enabled, err)
			}
			failed, err := s.State(ctx, 1)
			if err != nil || failed.Status != "failed" || failed.AppliedRevision != before.AppliedRevision {
				t.Fatalf("disable state: %+v %v", failed, err)
			}
			reject.Store(false)
			if err := s.SyncDirty(ctx); err != nil {
				t.Fatal(err)
			}
			if err := mutator.SetEnabled(ctx, nil, 1, true, "test"); err != nil {
				t.Fatal(err)
			}
			reject.Store(true)
			if err := mutator.Delete(ctx, nil, 1, "test"); !errors.As(err, &apply) {
				t.Fatalf("delete false success: %v", err)
			}
			var count int
			if err := db.QueryRow(tc.count).Scan(&count); err != nil || count != 0 {
				t.Fatalf("desired deletion missing: %d %v", count, err)
			}
		})
	}
}

func TestRollbackReportsSavedButUnapplied(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	audit := NewAuditService(db)
	history := NewConfigHistoryService(db, [32]byte{1})
	storage := NewProxyHostService(db, nil, audit, nil, nil)
	input := models.CreateProxyHostInput{DomainNames: json.RawMessage(`["original.test"]`), ForwardScheme: "http", ForwardHost: "127.0.0.1", ForwardPort: 8080}
	host, err := storage.Create(ctx, nil, input, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := history.Snapshot(ctx, "proxy_host", host.ID, "update", host, nil); err != nil {
		t.Fatal(err)
	}
	input.DomainNames = json.RawMessage(`["changed.test"]`)
	if _, err := storage.Update(ctx, nil, host.ID, input, "test"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400) }))
	defer server.Close()
	client := caddy.NewClient(server.URL)
	s := NewCaddySyncService(db, client, nil, "")
	hosts := NewProxyHostService(db, client, audit, nil, s)
	history.Register("proxy_host", func(ctx context.Context, actor *int64, id int64, snapshot []byte, ip string) error {
		var saved models.ProxyHost
		if err := json.Unmarshal(snapshot, &saved); err != nil {
			return err
		}
		_, err := hosts.Update(ctx, actor, id, models.CreateProxyHostInput{ServerID: saved.ServerID, DomainNames: saved.DomainNames, ForwardScheme: saved.ForwardScheme, ForwardHost: saved.ForwardHost, ForwardPort: saved.ForwardPort}, ip)
		return err
	})
	entries, err := history.List(ctx, "proxy_host", host.ID)
	if err != nil {
		t.Fatal(err)
	}
	var apply *ConfigApplyError
	if err := history.Rollback(ctx, nil, entries[0].ID, "test"); !errors.As(err, &apply) {
		t.Fatalf("rollback false success: %v", err)
	}
	restored, err := hosts.GetByID(ctx, host.ID)
	if err != nil || string(restored.DomainNames) != `["original.test"]` {
		t.Fatalf("rollback desired state missing: %+v %v", restored, err)
	}
	state, err := s.State(ctx, 1)
	if err != nil || state.Status != "failed" || state.AppliedRevision != 0 {
		t.Fatalf("rollback apply state: %+v %v", state, err)
	}
}

func TestMoveSynchronizesOldAndNewServers(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO servers(id,name,hostname) VALUES (2,'remote','remote.test')`); err != nil {
		t.Fatal(err)
	}
	var lastConfig atomic.Value
	lastConfig.Store("")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		lastConfig.Store(string(body))
		w.WriteHeader(200)
	}))
	defer server.Close()
	client := caddy.NewClient(server.URL)
	syncSvc := NewCaddySyncService(db, client, nil, "")
	hosts := NewProxyHostService(db, client, NewAuditService(db), nil, syncSvc)
	input := models.CreateProxyHostInput{ServerID: 1, DomainNames: json.RawMessage(`["moved.test"]`), ForwardScheme: "http", ForwardHost: "127.0.0.1", ForwardPort: 8080}
	host, err := hosts.Create(ctx, nil, input, "test")
	if err != nil {
		t.Fatal(err)
	}
	input.ServerID = 2
	host, err = hosts.Update(ctx, nil, host.ID, input, "test")
	if err != nil {
		t.Fatal(err)
	}
	if host.ServerID != 2 || strings.Contains(lastConfig.Load().(string), "moved.test") {
		t.Fatal("old server retained moved route")
	}
	oldState, err := syncSvc.State(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	newState, err := syncSvc.State(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if oldState.Status != "applied" || newState.Status != "queued" {
		t.Fatalf("move state: %+v %+v", oldState, newState)
	}
}

func TestDirtyRevisionRollsBackWithTheMutation(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	s := NewCaddySyncService(db, nil, nil, "")
	before, err := s.State(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO proxy_hosts(caddy_id,server_id,domain_names,forward_host,forward_port) VALUES ('rolled-back',1,'["rolled-back.test"]','127.0.0.1',8080)`); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	after, err := s.State(ctx, 1)
	if err != nil || after.DesiredRevision != before.DesiredRevision {
		t.Fatalf("revision escaped transaction: %+v %v", after, err)
	}
}

func TestRestoreReportsSavedButFailedApplication(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	hosts := NewProxyHostService(db, nil, NewAuditService(db), nil, nil)
	if _, err := hosts.Create(ctx, nil, models.CreateProxyHostInput{DomainNames: json.RawMessage(`["restore.test"]`), ForwardScheme: "http", ForwardHost: "127.0.0.1", ForwardPort: 8080}, "test"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(400) }))
	defer server.Close()
	s := NewCaddySyncService(db, caddy.NewClient(server.URL), nil, "")
	backup := NewBackupService(db, s)
	payload, err := backup.Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	count, err := backup.Import(ctx, payload)
	var apply *ConfigApplyError
	if count == 0 || !errors.As(err, &apply) {
		t.Fatalf("restore false success: %d %v", count, err)
	}
	state, err := s.State(ctx, 1)
	if err != nil || state.Status != "failed" || state.AppliedRevision != 0 {
		t.Fatalf("restore state: %+v %v", state, err)
	}
}

func TestHostAuditOmitsCredentialBearingFields(t *testing.T) {
	db := testDB(t)
	audit := NewAuditService(db)
	before := models.ProxyHost{ID: 1, ServerID: 1, Enabled: true, CustomHeaders: json.RawMessage(`[{"name":"Authorization","value":"audit-secret-sentinel"}]`)}
	after := before
	after.Enabled = false
	audit.HostChange(context.Background(), nil, "disable", "proxy_host", 1, &before, &after, "192.0.2.1")
	var details string
	if err := db.QueryRow(`SELECT details FROM audit_log WHERE resource_type='proxy_host'`).Scan(&details); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(details, "audit-secret-sentinel") || !strings.Contains(details, `"before"`) || !strings.Contains(details, `"after"`) {
		t.Fatalf("incorrect audit snapshot: %s", details)
	}
}

func TestRejectedConfigurationIsSavedButNotApplied(t *testing.T) {
	db := testDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "SECRET-REJECTION-BODY", 400) }))
	defer server.Close()
	client := caddy.NewClient(server.URL)
	s := NewCaddySyncService(db, client, nil, "")
	hosts := NewProxyHostService(db, client, NewAuditService(db), nil, s)
	host, err := hosts.Create(context.Background(), nil, models.CreateProxyHostInput{DomainNames: json.RawMessage(`["saved.test"]`), ForwardScheme: "http", ForwardHost: "127.0.0.1", ForwardPort: 8080}, "test")
	var apply *ConfigApplyError
	if host == nil || !errors.As(err, &apply) {
		t.Fatalf("false success: host=%+v error=%v", host, err)
	}
	state, err := s.State(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "failed" || state.AppliedRevision != 0 || state.DesiredRevision < 2 || strings.Contains(state.Error, "SECRET") {
		t.Fatalf("incorrect state: %+v", state)
	}
	if err := hosts.Delete(context.Background(), nil, host.ID, "test"); !errors.As(err, &apply) {
		t.Fatalf("delete false success: %v", err)
	}
	if remaining, err := hosts.GetByID(context.Background(), host.ID); err != nil || remaining != nil {
		t.Fatalf("desired deletion lost: %v %v", remaining, err)
	}
	state, err = s.State(context.Background(), 1)
	if err != nil || state.Status != "failed" {
		t.Fatalf("deletion status: %+v %v", state, err)
	}
}

func TestConfigurationAcknowledgmentsAreCorrelated(t *testing.T) {
	db := testDB(t)
	s := NewCaddySyncService(db, nil, nil, "")
	if _, err := db.Exec(`INSERT INTO servers (id,name,hostname) VALUES (2,'remote','remote.test')`); err != nil {
		t.Fatal(err)
	}
	s.pending[2] = configAttempt{ID: "command-a", Revision: 1, Deadline: time.Now().Add(time.Minute)}
	for _, result := range []agent.CommandResultPayload{
		{CommandID: "wrong", Revision: 1, CommandKind: agent.KindConfigFullSync, Success: true},
		{CommandID: "command-a", Revision: 2, CommandKind: agent.KindConfigFullSync, Success: true},
		{CommandID: "command-a", Revision: 1, CommandKind: agent.KindConfigPatch, Success: true},
	} {
		s.commandResult(2, result)
		state, err := s.State(context.Background(), 2)
		if err != nil || state.AppliedRevision != 0 {
			t.Fatalf("uncorrelated result accepted: %+v %v", state, err)
		}
	}
	if _, err := db.Exec(`UPDATE server_config_state SET desired_revision=2 WHERE server_id=2`); err != nil {
		t.Fatal(err)
	}
	correct := agent.CommandResultPayload{CommandID: "command-a", Revision: 1, CommandKind: agent.KindConfigFullSync, Success: true}
	s.commandResult(2, correct)
	state, err := s.State(context.Background(), 2)
	if err != nil || state.AppliedRevision != 1 || state.DesiredRevision != 2 || state.Status != "queued" {
		t.Fatalf("newer desired revision misreported: %+v %v", state, err)
	}
	s.pending[2] = configAttempt{ID: "command-b", Revision: 2, Deadline: time.Now().Add(time.Minute)}
	s.commandResult(2, correct)
	state, err = s.State(context.Background(), 2)
	if err != nil || state.AppliedRevision != 1 {
		t.Fatal("duplicate result advanced state")
	}
	s.commandResult(2, agent.CommandResultPayload{CommandID: "command-b", Revision: 2, CommandKind: agent.KindConfigFullSync, Success: false, Error: "PRIVATE KEY SENTINEL"})
	state, err = s.State(context.Background(), 2)
	if err != nil || state.Status != "failed" || state.AppliedRevision != 1 || strings.Contains(state.Error, "PRIVATE") {
		t.Fatalf("agent failure: %+v %v", state, err)
	}
}

func TestDirtyRevisionSurvivesServiceRestartAndMove(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO servers (id,name,hostname) VALUES (2,'remote','remote.test'); INSERT INTO proxy_hosts (caddy_id,server_id,domain_names,forward_host,forward_port) VALUES ('fixture',1,'["move.test"]','127.0.0.1',8080)`); err != nil {
		t.Fatal(err)
	}
	first := NewCaddySyncService(db, nil, nil, "")
	before, err := first.State(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE proxy_hosts SET server_id=2 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	restarted := NewCaddySyncService(db, nil, nil, "")
	oldState, err := restarted.State(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	newState, err := restarted.State(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if oldState.DesiredRevision <= before.DesiredRevision || newState.DesiredRevision <= 1 || newState.Status != "queued" {
		t.Fatalf("move lost: old=%+v new=%+v", oldState, newState)
	}
	if err := restarted.SyncServer(ctx, 2); err != nil {
		t.Fatal(err)
	}
	state, err := restarted.State(ctx, 2)
	if err != nil || state.Status != "queued" || state.AppliedRevision != 0 {
		t.Fatalf("offline send claimed applied: %+v %v", state, err)
	}
}

func TestAcknowledgmentTimeoutIsDurable(t *testing.T) {
	db := testDB(t)
	s := NewCaddySyncService(db, nil, nil, "")
	if _, err := db.Exec(`INSERT INTO servers (id,name,hostname) VALUES (2,'remote','remote.test'); UPDATE server_config_state SET status='applied',applied_revision=desired_revision WHERE server_id=1`); err != nil {
		t.Fatal(err)
	}
	s.pending[2] = configAttempt{ID: "lost", Revision: 1, Deadline: time.Now().Add(-time.Second)}
	s.reconcile(context.Background())
	state, err := s.State(context.Background(), 2)
	if err != nil || state.Status != "failed" || state.AppliedRevision != 0 || s.attemptActive(2) {
		t.Fatalf("timeout lost: %+v %v", state, err)
	}
	if _, err := db.Exec(`UPDATE server_config_state SET updated_at=datetime('now','-31 seconds') WHERE server_id=2`); err != nil {
		t.Fatal(err)
	}
	s.reconcile(context.Background())
	state, err = s.State(context.Background(), 2)
	if err != nil || state.Status != "queued" || state.AppliedRevision != 0 {
		t.Fatalf("offline retry was not queued: %+v %v", state, err)
	}
}

func TestReconnectRebuildsDurableOfflineWorkAfterControllerRestart(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO servers (id,name,hostname) VALUES (2,'remote','remote.test'); INSERT INTO proxy_hosts (caddy_id,server_id,domain_names,forward_host,forward_port) VALUES ('fixture',2,'["offline-change.test"]','127.0.0.1',8080)`); err != nil {
		t.Fatal(err)
	}
	old := NewCaddySyncService(db, nil, agent.NewHub(nil, nil), "")
	if err := old.SyncServer(ctx, 2); err != nil {
		t.Fatal(err)
	}
	var current *CaddySyncService
	hub := agent.NewHub(nil, func(id int64) {
		if err := current.Reconnect(ctx, id); err != nil {
			t.Errorf("reconnect: %v", err)
		}
	})
	current = NewCaddySyncService(db, nil, hub, "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hub.ServeWS(2, w, r, nil) }))
	defer server.Close()
	previousID := ""
	for range 2 {
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
		if err != nil {
			t.Fatal(err)
		}
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		var command agent.Message
		if err := conn.ReadJSON(&command); err != nil {
			conn.Close()
			t.Fatal(err)
		}
		if command.ID == "" || command.ID == previousID || command.Revision < 2 || !strings.Contains(string(command.Payload), "offline-change.test") {
			conn.Close()
			t.Fatalf("incorrect reconnect command: %+v", command)
		}
		previousID = command.ID
		ack, err := agent.NewMessage(agent.KindCommandResult, agent.CommandResultPayload{CommandID: command.ID, Revision: command.Revision, CommandKind: command.Kind, Success: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.WriteJSON(ack); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(3 * time.Second)
		for {
			state, err := current.State(ctx, 2)
			if err != nil {
				t.Fatal(err)
			}
			if state.Status == "applied" && state.AppliedRevision == state.DesiredRevision {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("acknowledgment not persisted: %+v", state)
			}
			time.Sleep(10 * time.Millisecond)
		}
		conn.Close()
	}
}
