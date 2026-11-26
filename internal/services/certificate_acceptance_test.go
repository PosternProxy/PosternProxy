package services

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"
)

func acceptanceCertificatePair(t *testing.T, name string, notBefore, notAfter time.Time) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: name}, DNSNames: []string{name},
		NotBefore: notBefore, NotAfter: notAfter, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func TestCertificateAcceptanceRejectsInvalidUploadWithoutMutation(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	validCert, validKey := acceptanceCertificatePair(t, "app.example.com", now.Add(-time.Hour), now.Add(time.Hour))
	_, otherKey := acceptanceCertificatePair(t, "app.example.com", now.Add(-time.Hour), now.Add(time.Hour))
	expiredCert, expiredKey := acceptanceCertificatePair(t, "app.example.com", now.Add(-2*time.Hour), now.Add(-time.Hour))
	futureCert, futureKey := acceptanceCertificatePair(t, "app.example.com", now.Add(time.Hour), now.Add(2*time.Hour))
	for _, tc := range []struct {
		name, domain string
		cert, key    []byte
	}{
		{"invalid PEM", "app.example.com", []byte("not a certificate"), validKey},
		{"invalid key", "app.example.com", validCert, []byte("not a key")},
		{"mismatched key", "app.example.com", validCert, otherKey},
		{"expired certificate", "app.example.com", expiredCert, expiredKey},
		{"future certificate", "app.example.com", futureCert, futureKey},
		{"uncovered domain", "other.example.com", validCert, validKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, dir := testDB(t), t.TempDir()
			var key [32]byte
			key[0] = 1
			svc := NewCertificateService(db, dir, NewAuditService(db), key)
			domains, err := json.Marshal([]string{tc.domain})
			if err != nil {
				t.Fatal(err)
			}
			cert, err := svc.UploadCustom(context.Background(), nil, "test", domains, bytes.NewReader(tc.cert), bytes.NewReader(tc.key), "127.0.0.1")
			if err == nil || cert != nil {
				t.Errorf("invalid upload accepted: returnedCertificate=%v, error=%v", cert != nil, err)
			}
			var count int
			if err := db.QueryRow(`SELECT count(*) FROM certificates`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Errorf("invalid upload inserted %d certificate rows", count)
			}
			files, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(files) != 0 {
				t.Errorf("invalid upload left %d storage entries", len(files))
			}
		})
	}
}

func TestCertificateAcceptancePersistsValidatedExpiry(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	expires := now.Add(24 * time.Hour)
	certPEM, keyPEM := acceptanceCertificatePair(t, "app.example.com", now.Add(-time.Hour), expires)
	db := testDB(t)
	var key [32]byte
	key[0] = 1
	svc := NewCertificateService(db, t.TempDir(), NewAuditService(db), key)
	cert, err := svc.UploadCustom(context.Background(), nil, "test", json.RawMessage(`["app.example.com"]`), bytes.NewReader(certPEM), bytes.NewReader(keyPEM), "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if cert.ExpiresAt == nil {
		t.Fatal("validated certificate has no expiry metadata")
	}
	got, err := time.Parse(time.RFC3339, *cert.ExpiresAt)
	if err != nil || !got.Equal(expires) {
		t.Fatalf("expiry = %q, error=%v; want %s", *cert.ExpiresAt, err, expires.Format(time.RFC3339))
	}
}

func TestCertificateAcceptanceSelectsOnlyTargetTLSReferences(t *testing.T) {
	db := testDB(t)
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO servers(id,name,hostname) VALUES(2,'A','a.example.com'),(3,'B','b.example.com')`)
	for id := 1; id <= 9; id++ {
		exec(`INSERT INTO certificates(id,name,provider,domain_names,status) VALUES(?,?,'custom','["app.example.com"]','active')`, id, "certificate")
	}
	for _, host := range []struct {
		server, cert, enabled, httpOnly, passthrough int
	}{
		{2, 1, 1, 0, 0},
		{2, 1, 1, 0, 0},
		{3, 2, 1, 0, 0},
		{2, 3, 0, 0, 0},
		{2, 4, 1, 1, 0},
		{2, 5, 1, 0, 1},
		{1, 8, 1, 0, 0},
	} {
		exec(`INSERT INTO proxy_hosts(server_id,domain_names,forward_host,forward_port,certificate_id,enabled,http_only,tls_passthrough,caddy_id) VALUES(?,'["app.example.com"]','127.0.0.1',8080,?,?,?,?,'test-proxy')`, host.server, host.cert, host.enabled, host.httpOnly, host.passthrough)
	}
	exec(`INSERT INTO redirection_hosts(server_id,domain_names,forward_domain_name,certificate_id,caddy_id) VALUES(2,'["redirect.example.com"]','target.example.com',6,'test-redirect')`)
	exec(`INSERT INTO dead_hosts(server_id,domain_names,certificate_id,caddy_id) VALUES(2,'["parked.example.com"]',7,'test-dead')`)
	svc := NewCaddySyncService(db, nullCaddy(), nil, "test")
	for _, tc := range []struct {
		server int64
		want   []int64
	}{
		{1, []int64{8}},
		{2, []int64{1, 6, 7}},
		{3, []int64{2}},
	} {
		input, err := svc.buildInput(context.Background(), tc.server)
		if err != nil {
			t.Fatal(err)
		}
		var got []int64
		for _, cert := range input.Certificates {
			got = append(got, cert.ID)
		}
		sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("server %d certificate IDs = %v; want %v", tc.server, got, tc.want)
		}
	}
}

func TestCertificateAcceptanceMissingReferenceFails(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO proxy_hosts(domain_names,forward_host,forward_port,certificate_id,caddy_id) VALUES('["app.example.com"]','127.0.0.1',8080,999,'test-missing')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatal(err)
	}
	svc := NewCaddySyncService(db, nullCaddy(), nil, "test")
	if _, err := svc.buildInput(context.Background(), 1); err == nil {
		t.Fatal("missing referenced custom certificate silently omitted")
	}
}
