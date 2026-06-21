package services

import (
	"context"
	"encoding/json"
	"testing"
)

func TestAuditLog_ListAndMarshal(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	svc := NewAuditService(db)

	// One entry with nil details (stored as "null"), one with a real object.
	id := int64(0)
	if err := svc.Log(ctx, nil, "delete", "proxy_host", &id, nil, "1.2.3.4"); err != nil {
		t.Fatalf("log nil details: %v", err)
	}
	if err := svc.Log(ctx, nil, "create", "proxy_host", &id, map[string]any{"domains": "app.test"}, "1.2.3.4"); err != nil {
		t.Fatalf("log map details: %v", err)
	}

	entries, total, err := svc.List(ctx, "", nil, 1, 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 || len(entries) != 2 {
		t.Fatalf("expected 2 entries, got total=%d len=%d", total, len(entries))
	}

	// Marshaling the handler-style payload must not fail (json.RawMessage details
	// must be valid JSON).
	payload := map[string]any{"data": entries, "total": total, "page": 1, "limit": 50}
	if _, err := json.Marshal(payload); err != nil {
		t.Fatalf("marshal audit payload: %v", err)
	}
}
