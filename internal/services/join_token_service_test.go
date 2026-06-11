package services

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/posternproxy/posternproxy/internal/agent"
)

// insertTestServer inserts a server row that has no agent_token_hash and
// returns the new ID. The local server (id=1) seeded by the migration is
// unsuitable because we want a "pending" join state.
func insertTestServer(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO servers (name, hostname, status) VALUES (?, ?, 'pending')`, "test", "test.example")
	if err != nil {
		t.Fatalf("insert server: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// setAgentTokenHash marks the server as already-joined by writing a
// non-empty agent_token_hash.
func setAgentTokenHash(t *testing.T, db *sql.DB, serverID int64) {
	t.Helper()
	if _, err := db.Exec(`UPDATE servers SET agent_token_hash = 'sentinel' WHERE id = ?`, serverID); err != nil {
		t.Fatalf("set agent token hash: %v", err)
	}
}

func TestJoinTokenService_IssueConsume_RoundTrip(t *testing.T) {
	db := testDB(t)
	svc := NewJoinTokenService(db)
	serverID := insertTestServer(t, db)

	raw, exp, err := svc.Issue(context.Background(), nil, serverID, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if raw == "" {
		t.Fatal("expected non-empty raw token")
	}
	if !exp.After(time.Now()) {
		t.Errorf("expires_at not in the future: %v", exp)
	}

	got, err := svc.Consume(context.Background(), raw, "1.2.3.4")
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if got != serverID {
		t.Errorf("server id = %d, want %d", got, serverID)
	}
}

func TestJoinTokenService_Consume_UnknownToken(t *testing.T) {
	db := testDB(t)
	svc := NewJoinTokenService(db)
	insertTestServer(t, db)

	_, err := svc.Consume(context.Background(), "not-a-real-token", "")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("err = %v, want ErrInvalidToken", err)
	}
}

func TestJoinTokenService_Consume_Expired(t *testing.T) {
	db := testDB(t)
	svc := NewJoinTokenService(db)
	serverID := insertTestServer(t, db)

	raw, _, err := svc.Issue(context.Background(), nil, serverID, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	// Backdate the row so it appears expired.
	past := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	if _, err := db.Exec(`UPDATE join_tokens SET expires_at = ?`, past); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	_, err = svc.Consume(context.Background(), raw, "")
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("err = %v, want ErrTokenExpired", err)
	}
}

func TestJoinTokenService_Consume_AlreadyUsed(t *testing.T) {
	db := testDB(t)
	svc := NewJoinTokenService(db)
	serverID := insertTestServer(t, db)

	raw, _, err := svc.Issue(context.Background(), nil, serverID, 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := svc.Consume(context.Background(), raw, ""); err != nil {
		t.Fatalf("first Consume: %v", err)
	}
	_, err = svc.Consume(context.Background(), raw, "")
	if !errors.Is(err, ErrTokenAlreadyUsed) {
		t.Errorf("second Consume err = %v, want ErrTokenAlreadyUsed", err)
	}
}

func TestJoinTokenService_Issue_AlreadyJoined(t *testing.T) {
	db := testDB(t)
	svc := NewJoinTokenService(db)
	serverID := insertTestServer(t, db)
	setAgentTokenHash(t, db, serverID)

	_, _, err := svc.Issue(context.Background(), nil, serverID, 0)
	if !errors.Is(err, ErrServerAlreadyJoined) {
		t.Errorf("err = %v, want ErrServerAlreadyJoined", err)
	}
}

func TestJoinTokenService_PurgeExpired(t *testing.T) {
	db := testDB(t)
	svc := NewJoinTokenService(db)
	serverID := insertTestServer(t, db)

	// Active row — survives purge.
	if _, _, err := svc.Issue(context.Background(), nil, serverID, 0); err != nil {
		t.Fatalf("Issue active: %v", err)
	}

	// Old expired row — should be deleted.
	if _, _, err := svc.Issue(context.Background(), nil, serverID, 0); err != nil {
		t.Fatalf("Issue stale: %v", err)
	}
	old := time.Now().UTC().Add(-8 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := db.Exec(`UPDATE join_tokens SET expires_at = ? WHERE id = (SELECT MAX(id) FROM join_tokens)`, old); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	if err := svc.PurgeExpired(context.Background()); err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM join_tokens`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("rows remaining = %d, want 1", count)
	}
}

// ── ResetJoin (lives on ServerService but covered here for cohesion) ──────────

func newServerSvcWithHub(t *testing.T, db *sql.DB) (*ServerService, *agent.Hub) {
	t.Helper()
	audit := NewAuditService(db)
	tokens := NewAPITokenService(db)
	hub := agent.NewHub(nil, nil)
	var key [32]byte
	return NewServerService(db, audit, hub, tokens, key, "", ""), hub
}

func TestServerService_ResetJoin_AgentOffline(t *testing.T) {
	db := testDB(t)
	svc, _ := newServerSvcWithHub(t, db)
	serverID := insertTestServer(t, db)
	setAgentTokenHash(t, db, serverID)

	if err := svc.ResetJoin(context.Background(), nil, serverID, false, ""); err != nil {
		t.Fatalf("ResetJoin: %v", err)
	}
	var hash string
	if err := db.QueryRow(`SELECT agent_token_hash FROM servers WHERE id = ?`, serverID).Scan(&hash); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if hash != "" {
		t.Errorf("agent_token_hash = %q, want empty", hash)
	}
}

func TestServerService_ResetJoin_AgentOnline_NoForce(t *testing.T) {
	db := testDB(t)
	svc, hub := newServerSvcWithHub(t, db)
	serverID := insertTestServer(t, db)
	setAgentTokenHash(t, db, serverID)
	hub.MarkOnlineForTest(serverID)

	err := svc.ResetJoin(context.Background(), nil, serverID, false, "")
	if !errors.Is(err, ErrAgentConnected) {
		t.Errorf("err = %v, want ErrAgentConnected", err)
	}
}

func TestServerService_ResetJoin_AgentOnline_Force(t *testing.T) {
	db := testDB(t)
	svc, hub := newServerSvcWithHub(t, db)
	serverID := insertTestServer(t, db)
	setAgentTokenHash(t, db, serverID)
	hub.MarkOnlineForTest(serverID)

	if err := svc.ResetJoin(context.Background(), nil, serverID, true, ""); err != nil {
		t.Fatalf("ResetJoin force=true: %v", err)
	}
	var hash string
	if err := db.QueryRow(`SELECT agent_token_hash FROM servers WHERE id = ?`, serverID).Scan(&hash); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if hash != "" {
		t.Errorf("agent_token_hash = %q, want empty", hash)
	}
}

func TestServerService_ResetJoin_ThenIssueAgain(t *testing.T) {
	db := testDB(t)
	svc, _ := newServerSvcWithHub(t, db)
	joinSvc := NewJoinTokenService(db)
	serverID := insertTestServer(t, db)
	setAgentTokenHash(t, db, serverID)

	// Before reset, Issue should fail.
	if _, _, err := joinSvc.Issue(context.Background(), nil, serverID, 0); !errors.Is(err, ErrServerAlreadyJoined) {
		t.Fatalf("Issue before reset: err = %v, want ErrServerAlreadyJoined", err)
	}

	if err := svc.ResetJoin(context.Background(), nil, serverID, false, ""); err != nil {
		t.Fatalf("ResetJoin: %v", err)
	}

	// After reset, Issue should succeed.
	if _, _, err := joinSvc.Issue(context.Background(), nil, serverID, 0); err != nil {
		t.Errorf("Issue after reset: %v", err)
	}
}
