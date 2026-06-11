package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/posternproxy/posternproxy/internal/models"
)

func newHistorySvc(t *testing.T) (*ConfigHistoryService, *UserService) {
	t.Helper()
	db := testDB(t)
	audit := NewAuditService(db)
	history := NewConfigHistoryService(db)
	users := NewUserService(db, audit)
	return history, users
}

func TestConfigHistoryService_SnapshotAndList(t *testing.T) {
	history, _ := newHistorySvc(t)

	type payload struct{ Name string }
	entity := payload{Name: "test"}

	if err := history.Snapshot(context.Background(), "proxy_host", 42, "create", entity, nil); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	entries, err := history.List(context.Background(), "proxy_host", 42)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("List returned %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.ResourceType != "proxy_host" {
		t.Errorf("resource_type = %q", e.ResourceType)
	}
	if e.ResourceID != 42 {
		t.Errorf("resource_id = %d", e.ResourceID)
	}
	if e.Action != "create" {
		t.Errorf("action = %q", e.Action)
	}

	var got payload
	if err := json.Unmarshal([]byte(e.Snapshot), &got); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("snapshot Name = %q", got.Name)
	}
}

func TestConfigHistoryService_Snapshot_NilEntity(t *testing.T) {
	history, _ := newHistorySvc(t)

	if err := history.Snapshot(context.Background(), "proxy_host", 1, "create", nil, nil); err != nil {
		t.Fatalf("Snapshot nil entity: %v", err)
	}

	entries, _ := history.List(context.Background(), "proxy_host", 1)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Snapshot != "null" {
		t.Errorf("snapshot = %q, want null", entries[0].Snapshot)
	}
}

func TestConfigHistoryService_ListAll(t *testing.T) {
	history, _ := newHistorySvc(t)

	history.Snapshot(context.Background(), "proxy_host", 1, "create", nil, nil)
	history.Snapshot(context.Background(), "redirection_host", 2, "update", map[string]any{"x": 1}, nil)

	all, err := history.ListAll(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) < 2 {
		t.Errorf("ListAll returned %d, want >= 2", len(all))
	}
}

func TestConfigHistoryService_List_IsolatedByResource(t *testing.T) {
	history, _ := newHistorySvc(t)

	history.Snapshot(context.Background(), "proxy_host", 1, "create", nil, nil)
	history.Snapshot(context.Background(), "proxy_host", 2, "create", nil, nil)
	history.Snapshot(context.Background(), "proxy_host", 1, "update", map[string]any{"v": 2}, nil)

	entries, _ := history.List(context.Background(), "proxy_host", 1)
	if len(entries) != 2 {
		t.Errorf("List for resource 1 = %d entries, want 2", len(entries))
	}
	entries2, _ := history.List(context.Background(), "proxy_host", 2)
	if len(entries2) != 1 {
		t.Errorf("List for resource 2 = %d entries, want 1", len(entries2))
	}
}

func TestConfigHistoryService_Rollback_NoHandler(t *testing.T) {
	history, _ := newHistorySvc(t)
	history.Snapshot(context.Background(), "proxy_host", 1, "update", map[string]any{"a": 1}, nil)
	entries, _ := history.List(context.Background(), "proxy_host", 1)

	err := history.Rollback(context.Background(), nil, entries[0].ID, "")
	if err == nil {
		t.Error("expected error: no rollback handler registered")
	}
}

func TestConfigHistoryService_Rollback_NullSnapshot(t *testing.T) {
	history, _ := newHistorySvc(t)
	history.Snapshot(context.Background(), "proxy_host", 1, "create", nil, nil)
	entries, _ := history.List(context.Background(), "proxy_host", 1)

	err := history.Rollback(context.Background(), nil, entries[0].ID, "")
	if err == nil {
		t.Error("expected error rolling back a create (null snapshot)")
	}
}

func TestConfigHistoryService_Rollback_NotFound(t *testing.T) {
	history, _ := newHistorySvc(t)
	err := history.Rollback(context.Background(), nil, 9999, "")
	if err == nil {
		t.Error("expected error for non-existent history entry")
	}
}

func TestConfigHistoryService_Rollback_WithHandler(t *testing.T) {
	history, _ := newHistorySvc(t)

	type fakeEntity struct{ Val int }
	history.Snapshot(context.Background(), "fake_resource", 10, "update", fakeEntity{Val: 42}, nil)
	entries, _ := history.List(context.Background(), "fake_resource", 10)

	var rolledBackID int64
	var rolledBackData []byte
	history.Register("fake_resource", func(ctx context.Context, callerID *int64, id int64, snapshot []byte, ip string) error {
		rolledBackID = id
		rolledBackData = snapshot
		return nil
	})

	if err := history.Rollback(context.Background(), nil, entries[0].ID, "127.0.0.1"); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if rolledBackID != 10 {
		t.Errorf("rolledBackID = %d, want 10", rolledBackID)
	}

	var got fakeEntity
	if err := json.Unmarshal(rolledBackData, &got); err != nil {
		t.Fatalf("unmarshal rollback data: %v", err)
	}
	if got.Val != 42 {
		t.Errorf("rolled back Val = %d, want 42", got.Val)
	}
}

func TestConfigHistoryService_SnapshotWithCallerID(t *testing.T) {
	history, users := newHistorySvc(t)

	// created_by is a FK to users(id), so we need a real user.
	user, err := users.Create(context.Background(), nil, models.CreateUserInput{Email: "snap@test.com", Name: "Snap", Password: "12345678"}, "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := history.Snapshot(context.Background(), "proxy_host", 5, "delete", map[string]any{"x": 1}, &user.ID); err != nil {
		t.Fatalf("Snapshot with callerID: %v", err)
	}

	entries, _ := history.List(context.Background(), "proxy_host", 5)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].CreatedBy == nil || *entries[0].CreatedBy != user.ID {
		t.Errorf("created_by = %v, want %d", entries[0].CreatedBy, user.ID)
	}
}
