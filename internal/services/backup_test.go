package services

import (
	"context"
	"testing"
)

// TestBackupExportImportRoundTrip guards against the regression where backup
// export/restore referenced the dropped access_list_auth table, which made both
// operations fail with "no such table".
func TestBackupExportImportRoundTrip(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `
		INSERT INTO proxy_hosts (server_id, domain_names, forward_scheme, forward_host, forward_port)
		VALUES (1, '["app.test"]', 'http', '10.0.0.1', 8080)`); err != nil {
		t.Fatalf("seed proxy host: %v", err)
	}

	svc := NewBackupService(db, nil) // nil sync → no Caddy call on import

	payload, err := svc.Export(ctx)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(payload.Tables["proxy_hosts"]) != 1 {
		t.Fatalf("expected 1 proxy host in export, got %d", len(payload.Tables["proxy_hosts"]))
	}
	if _, ok := payload.Tables["access_list_auth"]; ok {
		t.Error("export should not contain the dropped access_list_auth table")
	}

	restored, err := svc.Import(ctx, payload)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if restored == 0 {
		t.Error("expected at least one restored row")
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM proxy_hosts WHERE forward_host = '10.0.0.1'`).Scan(&count); err != nil {
		t.Fatalf("count after restore: %v", err)
	}
	if count != 1 {
		t.Errorf("proxy host not preserved across round-trip: count = %d", count)
	}
}
