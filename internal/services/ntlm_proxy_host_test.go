package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/posternproxy/posternproxy/internal/models"
)

func TestNTLMCreatePersistsFlagAndEmptyContainers(t *testing.T) {
	db := testDB(t)
	svc := NewProxyHostService(db, nullCaddy(), NewAuditService(db), NewConfigHistoryService(db, [32]byte{1}), nil)
	input := baseProxyHostInput()
	input.NTLMEnabled = true
	input.HCEnabled = true
	input.HCActivePath = "/health"
	input.CustomHeaders = json.RawMessage(`[{"direction":"request","operation":"set","name":"X-Request-Id","value":"audit-secret-sentinel"}]`)
	host, err := svc.Create(context.Background(), nil, input, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	var flag int
	var locations, extra, headers, pages string
	if err := db.QueryRow(`SELECT ntlm_enabled, locations, extra_upstreams, custom_headers, error_pages FROM proxy_hosts WHERE id=?`, host.ID).
		Scan(&flag, &locations, &extra, &headers, &pages); err != nil {
		t.Fatal(err)
	}
	if flag != 1 || locations != "[]" || extra != "[]" || pages != "{}" || !strings.Contains(headers, "X-Request-Id") {
		t.Fatalf("stored flag=%d locations=%s extra=%s pages=%s headers=%s", flag, locations, extra, pages, headers)
	}
	got, err := svc.GetByID(context.Background(), host.ID)
	if err != nil || got == nil || !got.NTLMEnabled || !got.HCEnabled {
		t.Fatalf("read back: %+v %v", got, err)
	}
	var details string
	if err := db.QueryRow(`SELECT details FROM audit_log WHERE resource_type='proxy_host' AND action='create'`).Scan(&details); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(details, `"ntlm_enabled":true`) || strings.Contains(details, "audit-secret-sentinel") {
		t.Fatalf("audit snapshot: %s", details)
	}
}

func TestNTLMRejectedUpdateDoesNotMutateOrRecordHistory(t *testing.T) {
	db := testDB(t)
	history := NewConfigHistoryService(db, [32]byte{1})
	svc := NewProxyHostService(db, nullCaddy(), NewAuditService(db), history, nil)
	host, err := svc.Create(context.Background(), nil, baseProxyHostInput(), "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	revision := configRevision(t, db)
	historyCount := historyRows(t, db, host.ID)

	input := baseProxyHostInput()
	input.NTLMEnabled = true
	input.Locations = json.RawMessage(`[{"path":"/kept","forward_scheme":"http","forward_host":"10.0.0.2","forward_port":9}]`)
	updated, err := svc.Update(context.Background(), nil, host.ID, input, "127.0.0.1")
	if updated != nil || err == nil || err.Error() != models.NTLMErrLocations {
		t.Fatalf("updated=%v err=%v", updated, err)
	}
	if configRevision(t, db) != revision || historyRows(t, db, host.ID) != historyCount {
		t.Fatalf("rejected update changed revision %d->%d or history %d->%d", revision, configRevision(t, db), historyCount, historyRows(t, db, host.ID))
	}
	got, err := svc.GetByID(context.Background(), host.ID)
	if err != nil || got == nil || got.NTLMEnabled || string(got.Locations) != "[]" {
		t.Fatalf("row changed: %+v %v", got, err)
	}

	entries, err := history.List(context.Background(), "proxy_host", host.ID)
	if err != nil || len(entries) != 1 || entries[0].Action != "create" || entries[0].Snapshot != "{}" {
		t.Fatalf("history list: %+v %v", entries, err)
	}
}

func TestNTLMRejectedCreateWritesNothing(t *testing.T) {
	db := testDB(t)
	svc := NewProxyHostService(db, nullCaddy(), NewAuditService(db), NewConfigHistoryService(db, [32]byte{1}), nil)
	before := configRevision(t, db)
	input := baseProxyHostInput()
	input.NTLMEnabled = true
	input.AdvancedConfig = " { } "
	host, err := svc.Create(context.Background(), nil, input, "127.0.0.1")
	if host != nil || err == nil || err.Error() != models.NTLMErrAdvanced {
		t.Fatalf("host=%v err=%v", host, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM proxy_hosts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 || configRevision(t, db) != before || historyRows(t, db, 0) != 0 {
		t.Fatalf("rejected create persisted state count=%d revision=%d", count, configRevision(t, db))
	}
}

func TestOrdinaryExtraUpstreamStillCreates(t *testing.T) {
	svc := newProxyHostSvc(t)
	input := baseProxyHostInput()
	input.ExtraUpstreams = json.RawMessage(`[{"host":"10.0.0.2","port":9}]`)
	input.LBPolicy = "random"
	host, err := svc.Create(context.Background(), nil, input, "127.0.0.1")
	if err != nil || host.NTLMEnabled || !strings.Contains(string(host.ExtraUpstreams), "10.0.0.2") {
		t.Fatalf("host=%+v err=%v", host, err)
	}
}

func configRevision(t *testing.T, db *sql.DB) int {
	t.Helper()
	var revision int
	if err := db.QueryRow(`SELECT desired_revision FROM server_config_state WHERE server_id=1`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	return revision
}

func historyRows(t *testing.T, db *sql.DB, id int64) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM config_history WHERE resource_type='proxy_host' AND resource_id=?`, id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
