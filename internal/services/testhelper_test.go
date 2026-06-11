package services

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/database"
)

// testDB opens a fresh in-memory SQLite database with all migrations applied.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// nullCaddy returns a Caddy client that will fail all HTTP calls gracefully.
// Services treat Caddy push failures as non-fatal warnings, so tests still
// exercise the full DB path.
func nullCaddy() *caddy.Client {
	return caddy.NewClient("http://localhost:0")
}
