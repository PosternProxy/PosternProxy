package services

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/posternproxy/posternproxy/internal/models"
)

func TestNTLMBackupRoundTripPreservesBothFlags(t *testing.T) {
	ctx := context.Background()
	source := testDB(t)
	hosts := NewProxyHostService(source, nil, NewAuditService(source), nil, nil)
	on := baseProxyHostInput()
	on.DomainNames = []byte(`["ntlm-on.test"]`)
	on.NTLMEnabled = true
	off := baseProxyHostInput()
	off.DomainNames = []byte(`["ntlm-off.test"]`)
	off.ForwardPort = 8081
	off.Locations = []byte(`[{"path":"/legacy"}]`)
	if _, err := hosts.Create(ctx, nil, on, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := hosts.Create(ctx, nil, off, "test"); err != nil {
		t.Fatal(err)
	}
	payload, err := NewBackupService(source, nil).Export(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dest := testDB(t)
	if _, err := NewBackupService(dest, nil).Import(ctx, payload); err != nil {
		t.Fatal(err)
	}
	assertNTLMFlag(t, dest, "ntlm-on.test", 1)
	assertNTLMFlag(t, dest, "ntlm-off.test", 0)
}

func TestPortableNTLMRoundTripAndConflictLeavesDestination(t *testing.T) {
	ctx := context.Background()
	const passphrase = "portable-ntlm-pass"
	source := testDB(t)
	if _, err := source.Exec(`INSERT INTO proxy_hosts (domain_names, forward_host, forward_port, ntlm_enabled) VALUES
		('["portable-on.test"]','127.0.0.1',8080,1),
		('["portable-off.test"]','127.0.0.1',8081,0)`); err != nil {
		t.Fatal(err)
	}
	archive, err := NewBackupService(source, nil).ExportPortable(ctx, passphrase)
	if err != nil {
		t.Fatal(err)
	}
	dest := testDB(t)
	if _, err := NewBackupService(dest, nil).RestorePortable(ctx, archive, PortableRestoreOptions{Passphrase: passphrase}); err != nil {
		t.Fatal(err)
	}
	assertNTLMFlag(t, dest, "portable-on.test", 1)
	assertNTLMFlag(t, dest, "portable-off.test", 0)

	conflict := testDB(t)
	if _, err := conflict.Exec(`INSERT INTO proxy_hosts (domain_names, forward_host, forward_port, ntlm_enabled, locations) VALUES ('["portable-conflict.test"]','127.0.0.1',8080,1,'[{"path":"/x"}]')`); err != nil {
		t.Fatal(err)
	}
	bad, err := NewBackupService(conflict, nil).ExportPortable(ctx, passphrase)
	if err != nil {
		t.Fatal(err)
	}
	kept := testDB(t)
	if _, err := kept.Exec(`INSERT INTO proxy_hosts (domain_names, forward_host, forward_port) VALUES ('["marker.test"]','10.0.0.9',9)`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewBackupService(kept, nil).RestorePortable(ctx, bad, PortableRestoreOptions{Passphrase: passphrase}); err == nil || !strings.Contains(err.Error(), models.NTLMErrLocations) {
		t.Fatal(err)
	}
	assertNTLMFlag(t, kept, "marker.test", 0)
	var conflictCount int
	if err := kept.QueryRow(`SELECT COUNT(*) FROM proxy_hosts WHERE domain_names='["portable-conflict.test"]'`).Scan(&conflictCount); err != nil || conflictCount != 0 {
		t.Fatalf("conflict host landed: %d %v", conflictCount, err)
	}
}

func TestBackupRejectsIllegalNTLMFlagsWithoutReplacingHosts(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		value any
	}{
		{name: "string one", value: "1"},
		{name: "string true", value: "true"},
		{name: "fraction", value: 1.5},
		{name: "two", value: float64(2)},
		{name: "negative", value: int64(-1)},
		{name: "nil", value: nil},
		{name: "plain int", value: int(1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testDB(t)
			if _, err := db.Exec(`INSERT INTO proxy_hosts (domain_names, forward_host, forward_port) VALUES ('["marker.test"]','10.0.0.9',9)`); err != nil {
				t.Fatal(err)
			}
			payload := &BackupPayload{Version: backupVersion, Tables: map[string][]map[string]any{
				"proxy_hosts": {{
					"forward_host": "127.0.0.1",
					"forward_port": int64(8080),
					"domain_names": `["imported.test"]`,
					"ntlm_enabled": tc.value,
				}},
			}}
			if _, err := NewBackupService(db, nil).Import(ctx, payload); err == nil || !strings.Contains(err.Error(), "invalid ntlm_enabled value") {
				t.Fatal(err)
			}
			assertNTLMFlag(t, db, "marker.test", 0)
			var imported int
			if err := db.QueryRow(`SELECT COUNT(*) FROM proxy_hosts WHERE domain_names='["imported.test"]'`).Scan(&imported); err != nil || imported != 0 {
				t.Fatalf("illegal flag was imported: %d %v", imported, err)
			}
		})
	}
}

func TestOldBackupOmitsNTLMAndStillRestoresLocations(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	payload := &BackupPayload{Version: backupVersion, Tables: map[string][]map[string]any{
		"proxy_hosts": {{
			"domain_names": `["legacy.test"]`,
			"forward_host": "127.0.0.1",
			"forward_port": int64(8080),
			"locations":    `[{"path":"/legacy"}]`,
		}},
	}}
	if _, err := NewBackupService(db, nil).Import(ctx, payload); err != nil {
		t.Fatal(err)
	}
	assertNTLMFlag(t, db, "legacy.test", 0)
	var locations string
	if err := db.QueryRow(`SELECT locations FROM proxy_hosts WHERE domain_names='["legacy.test"]'`).Scan(&locations); err != nil || locations != `[{"path":"/legacy"}]` {
		t.Fatalf("locations=%s err=%v", locations, err)
	}
}

func TestBackupAcceptsZeroOneNTLMEncodings(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		value any
		want  int
	}{
		{name: "bool", value: true, want: 1},
		{name: "float one", value: float64(1), want: 1},
		{name: "int64 zero", value: int64(0), want: 0},
		{name: "float zero", value: float64(0), want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testDB(t)
			payload := &BackupPayload{Version: backupVersion, Tables: map[string][]map[string]any{
				"proxy_hosts": {{
					"domain_names": `["encoded.test"]`,
					"forward_host": "127.0.0.1",
					"forward_port": int64(8080),
					"ntlm_enabled": tc.value,
				}},
			}}
			if _, err := NewBackupService(db, nil).Import(ctx, payload); err != nil {
				t.Fatal(err)
			}
			assertNTLMFlag(t, db, "encoded.test", tc.want)
		})
	}
}

func assertNTLMFlag(t *testing.T, db *sql.DB, domain string, want int) {
	t.Helper()
	var flag int
	err := db.QueryRow(`SELECT ntlm_enabled FROM proxy_hosts WHERE domain_names=?`, `["`+domain+`"]`).Scan(&flag)
	if err != nil || flag != want {
		t.Fatalf("%s ntlm_enabled=%d err=%v", domain, flag, err)
	}
}
