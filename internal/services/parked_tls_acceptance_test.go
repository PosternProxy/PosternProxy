package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParkedHTTPOnlyPersistsAndSkipsCertificates(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO certificates(id,name,provider,domain_names,cert_path,key_path) VALUES(7,'ignored','custom','["park.test"]','missing-cert.pem','missing-key.pem')`); err != nil {
		t.Fatal(err)
	}
	svc := NewDeadHostService(db, nil, NewAuditService(db), nil)
	certID := int64(7)
	host, err := svc.Create(context.Background(), nil, CreateDeadHostInput{DomainNames: json.RawMessage(`["park.test"]`), HTTPOnly: true, CertificateID: &certID}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !host.HTTPOnly {
		t.Fatal("HTTP-only choice was lost")
	}
	sync := NewCaddySyncService(db, nil, nil, "")
	input, err := sync.buildInput(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Certificates) != 0 {
		t.Fatal("HTTP-only parked host distributed a certificate")
	}
	if len(input.DeadHosts) != 1 || !input.DeadHosts[0].HTTPOnly {
		t.Fatal("HTTP-only choice did not reach configuration builder")
	}
	backup := NewBackupService(db, nil)
	payload, err := backup.Export(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := backup.Import(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	restored, err := svc.GetByID(context.Background(), host.ID)
	if err != nil || !restored.HTTPOnly {
		t.Fatal("backup lost HTTP-only mode", err)
	}
}

func TestTLSChecksCoverHostTypesWithoutIDCollisions(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO proxy_hosts(id,domain_names,forward_host,forward_port,ssl_forced) VALUES(1,'["example.com","unmatched.example.test"]','127.0.0.1',8080,1);
 INSERT INTO redirection_hosts(id,domain_names,forward_domain_name,ssl_forced) VALUES(1,'["example.com"]','target.example',1);
 INSERT INTO dead_hosts(id,domain_names) VALUES(1,'["example.com"]');
 INSERT INTO dead_hosts(id,domain_names,http_only) VALUES(2,'["http-only.example"]',1)`); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()
	svc := NewTLSProbeService(db, server.Listener.Addr().String())
	svc.poll()
	statuses := svc.Statuses()
	if len(statuses) != 3 {
		t.Fatalf("TLS results: got %d, want all three host types", len(statuses))
	}
	found := map[string]TLSProbeStatus{}
	for _, status := range statuses {
		found[status.ResourceType] = status
	}
	if len(found) != 3 {
		t.Fatal("host IDs collided across resource types")
	}
	if len(found["proxy_host"].Domains) != 2 || found["proxy_host"].Status == TLSProbeValid {
		t.Fatal("unmatched alias was hidden by valid primary domain")
	}
	if found["redirection_host"].Status != TLSProbeValid || found["dead_host"].Status != TLSProbeValid {
		t.Fatal("redirect/parked HTTPS was not checked")
	}
}
