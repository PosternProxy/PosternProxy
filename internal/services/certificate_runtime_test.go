package services

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/certstore"
)

func TestCertificateSourcesEncryptedAndApplyFailurePreservesRuntime(t *testing.T) {
	db, root, runtimeDir := testDB(t), t.TempDir(), t.TempDir()
	var key [32]byte
	key[0] = 42
	certPEM, keyPEM := acceptanceCertificatePair(t, "app.example.com", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	svc := NewCertificateService(db, root, NewAuditService(db), key)
	cert, err := svc.UploadCustom(context.Background(), nil, "app", json.RawMessage(`["app.example.com"]`), bytes.NewReader(certPEM), bytes.NewReader(keyPEM), "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{*cert.CertPath, *cert.KeyPath} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(raw, []byte("enc:v1:")) || bytes.Contains(raw, []byte("-----BEGIN")) {
			t.Fatal("stored source is not encrypted")
		}
	}
	if _, err := db.Exec(`INSERT INTO proxy_hosts(domain_names,forward_host,forward_port,certificate_id,caddy_id) VALUES('["app.example.com"]','127.0.0.1',8080,?,'test')`, cert.ID); err != nil {
		t.Fatal(err)
	}
	reject := false
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if reject {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var cfg json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			t.Error(err)
		}
		received = append([]byte(nil), cfg...)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	sync := NewCaddySyncService(db, caddy.NewClient(server.URL), nil, "test")
	sync.ConfigureCertificates(certstore.Store{Root: root, Key: key}, runtimeDir)
	if err := sync.FullSync(context.Background()); err != nil {
		t.Fatal(err)
	}
	oldDir := filepath.Join(runtimeDir, certstore.Name(cert.ID, certstore.Pair{Certificate: certPEM, Key: keyPEM}))
	if _, err := os.Stat(filepath.Join(oldDir, "key.pem")); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(received, keyPEM) || bytes.Contains(received, []byte(root)) {
		t.Fatal("Caddy config exposes source storage or raw private material")
	}
	if _, err := db.Exec(`UPDATE proxy_hosts SET certificate_id=NULL`); err != nil {
		t.Fatal(err)
	}
	reject = true
	if err := sync.FullSync(context.Background()); err == nil {
		t.Fatal("rejected apply succeeded")
	}
	if _, err := os.Stat(filepath.Join(oldDir, "key.pem")); err != nil {
		t.Fatal("failed apply retired previously active material")
	}
	reject = false
	if err := sync.FullSync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("successful removal retained inactive runtime directory: %v", err)
	}
}

func TestLegacyCertificateEncryptionCanResume(t *testing.T) {
	root := t.TempDir()
	certPEM, keyPEM := acceptanceCertificatePair(t, "app.example.com", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	certPath, keyPath := filepath.Join(root, "cert.pem"), filepath.Join(root, "key.pem")
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	store := certstore.Store{Root: root, Key: [32]byte{1}}
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO certificates(id,name,provider,domain_names,cert_path,key_path,status) VALUES(1,'legacy','custom','["app.example.com"]',?,?,'active')`, certPath, keyPath); err != nil {
		t.Fatal(err)
	}
	svc := NewCertificateService(db, root, NewAuditService(db), store.Key)
	for n := 0; n < 2; n++ {
		if err := svc.ProtectExisting(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	var protectedCert, protectedKey string
	if err := db.QueryRow(`SELECT cert_path,key_path FROM certificates WHERE id=1`).Scan(&protectedCert, &protectedKey); err != nil {
		t.Fatal(err)
	}
	if protectedCert == certPath || protectedKey == keyPath {
		t.Fatal("legacy paths were not replaced with immutable protected copies")
	}
	for n, path := range []string{certPath, keyPath} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		want := certPEM
		if n == 1 {
			want = keyPEM
		}
		if !bytes.Equal(raw, want) {
			t.Fatal("migration changed material still used by previous Caddy config")
		}
	}
	got, err := store.Load(protectedCert, protectedKey)
	if err != nil || !bytes.Equal(got.Certificate, certPEM) || !bytes.Equal(got.Key, keyPEM) {
		t.Fatalf("migration changed pair: %v", err)
	}
}

func TestRuntimeManifestRejectsUnreferencedAndUnsafeMaterial(t *testing.T) {
	certPEM, keyPEM := acceptanceCertificatePair(t, "app.example.com", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	pair := certstore.Pair{Certificate: certPEM, Key: keyPEM}
	root := t.TempDir()
	if _, _, err := certstore.StageConfig(json.RawMessage(`{"apps":{}}`), []certstore.Material{{ID: 1, Pair: pair}}, root); err == nil {
		t.Fatal("unreferenced private material accepted")
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatal("rejected manifest wrote files")
	}
	if _, _, err := certstore.StageConfig(json.RawMessage(`{"apps":{"tls":{"certificates":{"load_files":[{"certificate":"/etc/shadow","key":"/etc/shadow"}]}}}}`), nil, root); err == nil {
		t.Fatal("arbitrary certificate file path accepted")
	}
	name := certstore.Name(1, pair)
	raw, _ := json.Marshal(map[string]any{"apps": map[string]any{"tls": map[string]any{"certificates": map[string]any{"load_files": []map[string]any{{"certificate": "/other/" + name + "/cert.pem", "key": "/other/" + name + "/key.pem"}}}}}})
	config, keep, err := certstore.StageConfig(raw, []certstore.Material{{ID: 1, Pair: pair}}, root)
	if err != nil || !keep[name] || bytes.Contains(config, []byte("/other/")) {
		t.Fatalf("manifest did not stage and rewrite managed paths: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, name, "key.pem")); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeRefusesSymlinkAndRepairsTraversalMode(t *testing.T) {
	certPEM, keyPEM := acceptanceCertificatePair(t, "app.example.com", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	pair := certstore.Pair{Certificate: certPEM, Key: keyPEM}
	root := filepath.Join(t.TempDir(), "runtime")
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "key.pem"), []byte("outside sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, root); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := certstore.StageRuntime(root, 1, pair); err == nil {
		t.Fatal("runtime accepted symlink root")
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := certstore.StageRuntime(root, 1, pair); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(root)
		if err != nil || info.Mode().Perm() != 0750 {
			t.Fatalf("runtime parent is not group traversable: %v", err)
		}
	}
	name := certstore.Name(2, pair)
	if err := os.Symlink(outside, filepath.Join(root, name)); err != nil {
		t.Fatal(err)
	}
	if err := certstore.PruneRuntime(root, map[string]bool{}); err == nil {
		t.Fatal("cleanup followed unsafe entry")
	}
	got, err := os.ReadFile(filepath.Join(outside, "key.pem"))
	if err != nil || string(got) != "outside sentinel" {
		t.Fatal("touched material outside managed root")
	}
}

func TestLegacyProtectionPreservesExpiredAndReportsMissing(t *testing.T) {
	db, root := testDB(t), t.TempDir()
	certPEM, keyPEM := acceptanceCertificatePair(t, "expired.example.com", time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	var key [32]byte
	key[0] = 7
	store := certstore.Store{Root: root, Key: key}
	certPath, keyPath, err := store.Save(1, certstore.Pair{Certificate: certPEM, Key: keyPEM})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO certificates(id,name,provider,domain_names,cert_path,key_path,status) VALUES(1,'expired','custom','["expired.example.com"]',?,?,'active')`, certPath, keyPath); err != nil {
		t.Fatal(err)
	}
	svc := NewCertificateService(db, root, NewAuditService(db), key)
	if err := svc.ProtectExisting(context.Background()); err != nil {
		t.Fatalf("expired unused certificate prevented migration: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO certificates(id,name,provider,domain_names,cert_path,key_path,status) VALUES(2,'missing','custom','["missing.example.com"]',?,?,'active')`, filepath.Join(root, "missing", "cert.pem"), filepath.Join(root, "missing", "key.pem")); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProtectExisting(context.Background()); err == nil {
		t.Fatal("missing legacy material silently accepted")
	}
}

func TestCertificateRejectsExtraPrivateMaterial(t *testing.T) {
	certPEM, keyPEM := acceptanceCertificatePair(t, "app.example.com", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	_, extra := acceptanceCertificatePair(t, "other.example.com", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	for _, pair := range []certstore.Pair{
		{Certificate: certPEM, Key: append(append([]byte(nil), keyPEM...), extra...)},
		{Certificate: append(append([]byte(nil), certPEM...), extra...), Key: keyPEM},
	} {
		if _, err := certstore.Validate(pair, nil); err == nil {
			t.Fatal("unrelated private key accepted in certificate material")
		}
	}
}
