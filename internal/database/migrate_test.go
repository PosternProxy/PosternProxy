package database

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestInitialMigrationIsFrozen(t *testing.T) {
	baseline, err := os.ReadFile("testdata/beta-exit-2026-10-24.sql")
	if err != nil {
		t.Fatal(err)
	}
	initial, err := migrationFS.ReadFile("migrations/001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	normalize := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	if !bytes.Equal(normalize(baseline), normalize(initial)) {
		t.Fatal("001_initial.sql is frozen; add a forward migration instead of changing the baseline")
	}
}

func TestBaselineUpgradePreservesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.db")
	// Recreate the database from the frozen source, independently of future
	// migrations, to exercise startup from an already-applied baseline.
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	baseline, err := os.ReadFile("testdata/beta-exit-2026-10-24.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(baseline)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (datetime('now')));
		INSERT INTO schema_migrations (version) VALUES ('001_initial.sql');
		INSERT INTO proxy_hosts (server_id, domain_names, forward_scheme, forward_host, forward_port)
		VALUES (1, '["baseline.test"]', 'http', '127.0.0.1', 8080)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO dead_hosts(domain_names) VALUES('["old-park.test"]')`); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := Migrate(db); err != nil {
			t.Fatalf("upgrade/repeated startup: %v", err)
		}
	}
	var httpOnly int
	if err := db.QueryRow(`SELECT http_only FROM dead_hosts WHERE domain_names='["old-park.test"]'`).Scan(&httpOnly); err != nil || httpOnly != 0 {
		t.Fatalf("parked host migration changed existing HTTPS mode: %v", err)
	}
	var host string
	if err := db.QueryRow(`SELECT forward_host FROM proxy_hosts WHERE domain_names = '["baseline.test"]'`).Scan(&host); err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" {
		t.Fatalf("baseline host changed: %q", host)
	}
	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("upgrade left foreign-key violations")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
