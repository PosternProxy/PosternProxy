package app

import (
	"context"
	"github.com/posternproxy/posternproxy/internal/config"
	"os"
	"path/filepath"
	"testing"
)

func TestReservedManagementPortFailsBeforeDatabaseCreation(t *testing.T) {
	database := filepath.Join(t.TempDir(), "must-not-exist.db")
	app, err := New(context.Background(), &config.Config{ListenAddr: ":8443", DBPath: database})
	if err == nil {
		app.Close()
		t.Fatal("runtime accepted reserved bridge port")
	}
	if _, err := os.Stat(database); !os.IsNotExist(err) {
		t.Fatalf("startup touched database: %v", err)
	}
}
