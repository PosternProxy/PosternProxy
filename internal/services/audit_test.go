package services

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
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

// A failed audit write must stay observable even though privileged-action
// callers discard the returned error.
func TestAuditLogFailureIsObservable(t *testing.T) {
	db := testDB(t)
	svc := NewAuditService(db)
	db.Close() // force subsequent writes to fail

	var buf bytes.Buffer
	orig := zlog.Logger
	zlog.Logger = zerolog.New(&buf)
	defer func() { zlog.Logger = orig }()

	id := int64(5)
	err := svc.Log(context.Background(), nil, "revoke", "api_token", &id, nil, "1.2.3.4")
	if err == nil {
		t.Fatal("expected audit write to fail on a closed database")
	}
	out := buf.String()
	if !strings.Contains(out, "failed to write audit record") {
		t.Fatalf("audit failure was not logged: %q", out)
	}
	if !strings.Contains(out, "api_token") {
		t.Fatalf("audit failure log omitted the resource type: %q", out)
	}
}

// Settings audit records must carry a before/after snapshot with an explicit
// outcome, and must redact values for any secret-bearing key.
func TestSettingsChangeRedactsSensitiveValues(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	svc := NewAuditService(db)

	before := map[string]string{"caddy_api_url": "http://old", "api_secret": "old-secret"}
	after := map[string]string{"caddy_api_url": "http://new", "api_secret": "new-secret"}
	svc.SettingsChange(ctx, nil, true, before, after, "1.2.3.4")

	entries, _, err := svc.List(ctx, "settings", nil, 1, 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 settings entry, got %d", len(entries))
	}

	var details struct {
		Success bool              `json:"success"`
		Before  map[string]string `json:"before"`
		After   map[string]string `json:"after"`
	}
	if err := json.Unmarshal(entries[0].Details, &details); err != nil {
		t.Fatalf("unmarshal details: %v", err)
	}
	if !details.Success {
		t.Fatal("expected success outcome to be recorded")
	}
	if details.Before["caddy_api_url"] != "http://old" || details.After["caddy_api_url"] != "http://new" {
		t.Fatalf("non-sensitive values should pass through: before=%q after=%q",
			details.Before["caddy_api_url"], details.After["caddy_api_url"])
	}
	if details.Before["api_secret"] != "[redacted]" || details.After["api_secret"] != "[redacted]" {
		t.Fatalf("sensitive values must be redacted: before=%q after=%q",
			details.Before["api_secret"], details.After["api_secret"])
	}
}

// A rejected settings change carries unvalidated input (unknown keys like
// api_key). Its values must never reach the audit record verbatim.
func TestSettingsChangeRejectionMasksAttemptedValues(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	svc := NewAuditService(db)

	before := map[string]string{"caddy_api_url": "http://current"}
	attempted := map[string]string{"api_key": "super-secret-sentinel", "unknown_flag": "whatever"}
	svc.SettingsChange(ctx, nil, false, before, attempted, "1.2.3.4")

	entries, _, err := svc.List(ctx, "settings", nil, 1, 50)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 settings entry, got %d", len(entries))
	}
	if raw := string(entries[0].Details); strings.Contains(raw, "super-secret-sentinel") {
		t.Fatalf("rejected attempt leaked a value into the audit record: %s", raw)
	}

	var details struct {
		Success bool              `json:"success"`
		After   map[string]string `json:"after"`
	}
	if err := json.Unmarshal(entries[0].Details, &details); err != nil {
		t.Fatalf("unmarshal details: %v", err)
	}
	if details.Success {
		t.Fatal("rejected change must record success=false")
	}
	if details.After["api_key"] != "[redacted]" || details.After["unknown_flag"] != "[redacted]" {
		t.Fatalf("rejected attempt values must be masked, got %v", details.After)
	}
}
