package services

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"testing"

	"github.com/posternproxy/posternproxy/internal/database"
)

// A second SQLite connection commits between two export reads. Keeping this at
// the driver boundary exercises the public export with real WAL isolation.
type snapshotConnector struct {
	driver driver.Driver
	dsn    string
	before func(string)
}

func (c snapshotConnector) Driver() driver.Driver { return c.driver }
func (c snapshotConnector) Connect(context.Context) (driver.Conn, error) {
	conn, err := c.driver.Open(c.dsn)
	if err != nil {
		return nil, err
	}
	return &snapshotConn{Conn: conn, before: c.before}, nil
}

type snapshotConn struct {
	driver.Conn
	before func(string)
}

func (c *snapshotConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.before(query)
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}
func (c *snapshotConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
}

func TestPortableExportUsesOneSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.db")
	writer, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	changed := false
	reader := sql.OpenDB(snapshotConnector{driver: writer.Driver(), dsn: path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", before: func(query string) {
		if changed || query != "SELECT * FROM access_lists" {
			return
		}
		changed = true
		tx, err := writer.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		result, err := tx.Exec(`INSERT INTO certificates(name,provider,domain_names) VALUES ('concurrent','auto','["snapshot.test"]')`)
		if err != nil {
			t.Fatal(err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO proxy_hosts(server_id,domain_names,forward_host,forward_port,certificate_id) VALUES (1,'["snapshot.test"]','127.0.0.1',8080,?)`, id); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}})
	defer reader.Close()
	archive, err := NewBackupService(reader, nil).ExportPortable(context.Background(), "snapshot-test-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("concurrent write was not exercised")
	}
	var count int
	if err := writer.QueryRow(`SELECT count(*) FROM proxy_hosts`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("source write: %d %v", count, err)
	}
	destination := testDB(t)
	if _, err := NewBackupService(destination, nil).RestorePortable(context.Background(), archive, PortableRestoreOptions{Passphrase: "snapshot-test-passphrase"}); err != nil {
		t.Fatalf("snapshot cannot be restored: %v", err)
	}
	if err := destination.QueryRow(`SELECT count(*) FROM proxy_hosts`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("export mixed snapshots: %d %v", count, err)
	}
}
