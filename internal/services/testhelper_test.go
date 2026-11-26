package services

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/database"
)

// testDB opens an isolated SQLite database with all migrations applied.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// Storage tests omit synchronization; application tests supply an active sync
// service and verify Caddy failures separately.
func nullCaddy() *caddy.Client {
	return caddy.NewClient("http://localhost:0")
}
