package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/posternproxy/posternproxy/internal/security"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/posternproxy/posternproxy/internal/certstore"
)

func TestPortableRecoveryAcrossKeysAndExplicitServers(t *testing.T) {
	ctx := context.Background()
	sourceDB := testDB(t)
	sourceStore := certstore.Store{Root: filepath.Join(t.TempDir(), "source"), Key: [32]byte{1}}
	source := NewBackupService(sourceDB, nil)
	source.ConfigureCertificates(sourceStore)
	cert, key := acceptanceCertificatePair(t, "app.example.com", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	c, err := NewCertificateService(sourceDB, sourceStore.Root, NewAuditService(sourceDB), sourceStore.Key).UploadCustom(ctx, nil, "app", json.RawMessage(`["app.example.com"]`), bytes.NewReader(cert), bytes.NewReader(key), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sourceDB.Exec(`INSERT INTO servers(id,name,hostname,agent_token_hash,ssh_credentials_encrypted) VALUES(2,'source','remote','SOURCE_TOKEN','SOURCE_SSH'); INSERT INTO proxy_hosts(server_id,domain_names,forward_host,forward_port,certificate_id) VALUES(2,'["app.example.com"]','10.0.0.1',8080,?)`, c.ID); err != nil {
		t.Fatal(err)
	}
	archive, err := source.ExportPortable(ctx, "disposable-backup-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range [][]byte{cert, key, []byte("SOURCE_TOKEN"), []byte("SOURCE_SSH"), []byte("disposable-backup-passphrase")} {
		if bytes.Contains(archive, secret) {
			t.Fatal("archive contains plaintext secret")
		}
	}
	for _, mode := range []string{"mapping", "unenrolled", "missing", "wrong", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			db := testDB(t)
			store := certstore.Store{Root: filepath.Join(t.TempDir(), "destination"), Key: [32]byte{2}}
			svc := NewBackupService(db, nil)
			svc.ConfigureCertificates(store)
			if _, err := db.Exec(`INSERT INTO servers(id,name,hostname,agent_token_hash) VALUES(52,'target','target','TARGET_TOKEN')`); err != nil {
				t.Fatal(err)
			}
			before, _ := svc.Export(ctx)
			beforeJSON, _ := json.Marshal(before.Tables)
			opts := PortableRestoreOptions{Passphrase: "disposable-backup-passphrase"}
			data := append([]byte(nil), archive...)
			switch mode {
			case "mapping":
				opts.ServerMapping = map[int64]int64{2: 52}
			case "unenrolled":
				opts.CreateUnenrolled = true
			case "wrong":
				opts.Passphrase = "incorrect-passphrase"
			case "corrupt":
				data[len(data)/2] ^= 1
			}
			n, err := svc.RestorePortable(ctx, data, opts)
			if mode == "missing" || mode == "wrong" || mode == "corrupt" {
				if err == nil || n != 0 {
					t.Fatal("invalid restore accepted")
				}
				after, _ := svc.Export(ctx)
				afterJSON, _ := json.Marshal(after.Tables)
				if !bytes.Equal(beforeJSON, afterJSON) {
					t.Fatal("rejected restore changed destination")
				}
				return
			}
			if err != nil || n == 0 {
				t.Fatalf("restore: %d %v", n, err)
			}
			var serverID int64
			var certPath, keyPath string
			if err := db.QueryRow(`SELECT server_id FROM proxy_hosts`).Scan(&serverID); err != nil {
				t.Fatal(err)
			}
			if mode == "mapping" && serverID != 52 {
				t.Fatal("mapping ignored")
			}
			if mode == "unenrolled" {
				var deployment, token, credentials string
				if err := db.QueryRow(`SELECT deployment_type,agent_token_hash,ssh_credentials_encrypted FROM servers WHERE id=?`, serverID).Scan(&deployment, &token, &credentials); err != nil {
					t.Fatal(err)
				}
				if deployment != "manual" || token != "" || credentials != "" {
					t.Fatal("restored server is enrolled")
				}
			}
			if err := db.QueryRow(`SELECT cert_path,key_path FROM certificates WHERE id=?`, c.ID).Scan(&certPath, &keyPath); err != nil {
				t.Fatal(err)
			}
			pair, err := store.Load(certPath, keyPath)
			if err != nil || !bytes.Equal(pair.Key, key) {
				t.Fatalf("certificate recovery: %v", err)
			}
		})
	}
}

func TestLegacyBackupRejectsUnknownVersionAndTables(t *testing.T) {
	for _, payload := range []BackupPayload{{Version: "999.0"}, {Version: "1.0", Tables: map[string][]map[string]any{"unexpected": {{"id": 1}}}}} {
		db := testDB(t)
		svc := NewBackupService(db, nil)
		if _, err := svc.Import(context.Background(), &payload); err == nil {
			t.Fatal("invalid backup accepted")
		}
	}
}

func TestPortableRecoveryAuthenticatedMalformedPayloadCannotMutateDestination(t *testing.T) {
	ctx := context.Background()
	passphrase := "disposable-recovery-passphrase"
	for _, mode := range []string{"bad certificate", "unknown column", "missing certificate relation", "duplicate IDs"} {
		t.Run(mode, func(t *testing.T) {
			source := NewBackupService(testDB(t), nil)
			archive, err := source.ExportPortable(ctx, passphrase)
			if err != nil {
				t.Fatal(err)
			}
			var envelope portableEnvelope
			if err := json.Unmarshal(archive, &envelope); err != nil {
				t.Fatal(err)
			}
			salt, _ := base64.StdEncoding.DecodeString(envelope.Salt)
			key, err := portableKey(passphrase, salt)
			if err != nil {
				t.Fatal(err)
			}
			plaintext, err := security.Decrypt(key, envelope.Ciphertext)
			if err != nil {
				t.Fatal(err)
			}
			var payload portablePayload
			if err := json.Unmarshal([]byte(plaintext), &payload); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "bad certificate":
				payload.Config.Tables["certificates"] = []map[string]any{{"id": 1, "name": "bad", "provider": "custom", "domain_names": `["app.example.com"]`}}
				payload.Certificates = map[int64]certstore.Pair{1: {Certificate: []byte("invalid"), Key: []byte("invalid")}}
			case "unknown column":
				payload.Config.Tables["access_lists"] = []map[string]any{{"name": "bad", "unknown": "value"}}
			case "missing certificate relation":
				payload.Config.Tables["proxy_hosts"] = []map[string]any{{"server_id": 1, "domain_names": `["app.example.com"]`, "forward_host": "127.0.0.1", "forward_port": 8080, "certificate_id": 999}}
			case "duplicate IDs":
				payload.Config.Tables["access_lists"] = []map[string]any{{"id": 1, "name": "first"}, {"id": 1, "name": "second"}}
			}
			modified, _ := json.Marshal(payload)
			envelope.Ciphertext, err = security.Encrypt(key, string(modified))
			if err != nil {
				t.Fatal(err)
			}
			modified, _ = json.Marshal(envelope)
			db := testDB(t)
			root := filepath.Join(t.TempDir(), "certificates")
			target := NewBackupService(db, nil)
			target.ConfigureCertificates(certstore.Store{Root: root, Key: [32]byte{9}})
			if _, err := db.Exec(`INSERT INTO access_lists(name) VALUES('preserve')`); err != nil {
				t.Fatal(err)
			}
			before, _ := target.Export(ctx)
			beforeJSON, _ := json.Marshal(before.Tables)
			if n, err := target.RestorePortable(ctx, modified, PortableRestoreOptions{Passphrase: passphrase}); err == nil || n != 0 {
				t.Fatal("malformed authenticated payload accepted")
			}
			after, _ := target.Export(ctx)
			afterJSON, _ := json.Marshal(after.Tables)
			if !bytes.Equal(beforeJSON, afterJSON) {
				t.Fatal("rejected restore changed database")
			}
			if _, err := os.Stat(root); !os.IsNotExist(err) {
				t.Fatal("invalid restore staged certificate files")
			}
		})
	}
}
