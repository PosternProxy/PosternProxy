package services

import (
	"context"
	"testing"
)

func TestCaddyTrustDoesNotLeakAcrossServers(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO servers(id,name,hostname) VALUES(2,'remote','remote.test')`); err != nil {
		t.Fatal(err)
	}
	svc := NewCaddySyncService(db, nil, nil, "test")
	svc.ConfigureTrustedProxies(map[int64][]string{1: {"192.0.2.1/32"}})
	local, err := svc.buildInput(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	remote, err := svc.buildInput(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(local.TrustedProxies) != 1 || len(remote.TrustedProxies) != 0 {
		t.Fatal("server trust policy was inherited by a different server")
	}
}
