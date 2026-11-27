package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/posternproxy/posternproxy/internal/database"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

func TestNTLMRollbackRestoresFlagAndRefusesConflicts(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	history := services.NewConfigHistoryService(db, [32]byte{9})
	hosts := services.NewProxyHostService(db, nil, services.NewAuditService(db), history, nil)
	registerRollbackHandlers(history, hosts)

	input := models.CreateProxyHostInput{
		DomainNames:   []byte(`["roll.test"]`),
		ForwardScheme: "http",
		ForwardHost:   "192.168.1.10",
		ForwardPort:   8080,
		NTLMEnabled:   true,
	}
	host, err := hosts.Create(ctx, nil, input, "test")
	if err != nil || !host.NTLMEnabled {
		t.Fatalf("create: %+v %v", host, err)
	}
	input.NTLMEnabled = false
	updated, err := hosts.Update(ctx, nil, host.ID, input, "test")
	if err != nil || updated.NTLMEnabled {
		t.Fatalf("update: %+v %v", updated, err)
	}
	entries, err := history.List(ctx, "proxy_host", host.ID)
	if err != nil || len(entries) < 2 {
		t.Fatalf("history: %+v %v", entries, err)
	}
	var updateID int64
	for _, entry := range entries {
		if entry.Snapshot != "{}" {
			t.Fatalf("list exposed snapshot: %+v", entry)
		}
		if entry.Action == "update" && (updateID == 0 || entry.ID > updateID) {
			updateID = entry.ID
		}
	}
	if updateID == 0 {
		t.Fatal("missing update snapshot")
	}
	if err := history.Rollback(ctx, nil, updateID, "test"); err != nil {
		t.Fatal(err)
	}
	restored, err := hosts.GetByID(ctx, host.ID)
	if err != nil || restored == nil || !restored.NTLMEnabled {
		t.Fatalf("rollback did not restore the flag: %+v %v", restored, err)
	}

	var revision int
	if err := db.QueryRow(`SELECT desired_revision FROM server_config_state WHERE server_id=1`).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	conflict := `{"domain_names":["roll.test"],"forward_scheme":"http","forward_host":"192.168.1.10","forward_port":8080,"ntlm_enabled":true,"locations":[{"path":"/x"}]}`
	result, err := db.Exec(`INSERT INTO config_history (resource_type, resource_id, action, snapshot) VALUES ('proxy_host', ?, 'update', ?)`, host.ID, conflict)
	if err != nil {
		t.Fatal(err)
	}
	conflictID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if err := history.Rollback(ctx, nil, conflictID, "test"); err == nil || !strings.Contains(err.Error(), models.NTLMErrLocations) {
		t.Fatal(err)
	}
	var after int
	if err := db.QueryRow(`SELECT desired_revision FROM server_config_state WHERE server_id=1`).Scan(&after); err != nil || after != revision {
		t.Fatalf("conflict rollback changed revision %d -> %d (%v)", revision, after, err)
	}
	still, err := hosts.GetByID(ctx, host.ID)
	if err != nil || still == nil || !still.NTLMEnabled {
		t.Fatalf("conflict rollback mutated the host: %+v %v", still, err)
	}

	bare := `{"domain_names":["roll.test"],"forward_scheme":"http","forward_host":"192.168.1.10","forward_port":8080}`
	result, err = db.Exec(`INSERT INTO config_history (resource_type, resource_id, action, snapshot) VALUES ('proxy_host', ?, 'update', ?)`, host.ID, bare)
	if err != nil {
		t.Fatal(err)
	}
	bareID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if err := history.Rollback(ctx, nil, bareID, "test"); err != nil {
		t.Fatal(err)
	}
	off, err := hosts.GetByID(ctx, host.ID)
	if err != nil || off == nil || off.NTLMEnabled {
		t.Fatalf("snapshot without the field stayed on: %+v %v", off, err)
	}
}
