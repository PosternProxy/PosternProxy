package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/models"
)

func TestProvisionReadinessRequiresAuthenticatedAppliedRevision(t *testing.T) {
	db := testDB(t)
	hub := agent.NewHub(nil, nil)
	if _, err := db.Exec(`INSERT INTO servers(id,name,hostname) VALUES(2,'test','host')`); err != nil {
		t.Fatal(err)
	}
	svc := NewServerService(db, NewAuditService(db), hub, NewAPITokenService(db), nil, [32]byte{}, "https://controller.test", "")
	hub.MarkOnlineForTest(2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := svc.waitProvisionReady(ctx, 2); err == nil {
		t.Fatal("connection alone counted as applied")
	}
	if _, err := db.Exec(`UPDATE server_config_state SET status='applied',applied_revision=desired_revision WHERE server_id=2`); err != nil {
		t.Fatal(err)
	}
	if err := svc.waitProvisionReady(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	hub.Disconnect(2)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2()
	if err := svc.waitProvisionReady(ctx2, 2); err == nil {
		t.Fatal("applied state without connection counted as ready")
	}
}
func TestManualEnrollmentReturnsCredentialOnlyOnceAndRotationRevokes(t *testing.T) {
	db := testDB(t)
	hub := agent.NewHub(nil, nil)
	svc := NewServerService(db, NewAuditService(db), hub, NewAPITokenService(db), nil, [32]byte{}, "https://controller.test", "")
	srv, err := svc.Create(context.Background(), nil, models.CreateServerInput{Name: "manual", Hostname: "remote.test", DeploymentType: "manual"}, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if srv.Enrollment == nil || svc.GetProvisionRun(srv.ID) != nil {
		t.Fatal("manual enrollment started provisioning or lost credential")
	}
	token := srv.Enrollment.Token
	if !svc.ValidateAgentToken(context.Background(), srv.ID, token) {
		t.Fatal("manual credential cannot authenticate")
	}
	listed, err := svc.GetByID(context.Background(), srv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if listed.Enrollment != nil {
		t.Fatal("credential appeared on read")
	}
	hub.MarkOnlineForTest(srv.ID)
	next, err := svc.AgentCredentials(context.Background(), nil, srv.ID, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if hub.IsOnline(srv.ID) || svc.ValidateAgentToken(context.Background(), srv.ID, token) || !svc.ValidateAgentToken(context.Background(), srv.ID, next.Token) {
		t.Fatal("rotation did not revoke and disconnect previous identity")
	}
}
func TestRuntimeSettingsReportActualValuesAndRefuseChanges(t *testing.T) {
	db := testDB(t)
	svc := NewSettingsService(db)
	svc.ConfigureRuntime("http://127.0.0.1:2019", "/private/certs")
	if err := svc.SetBulk(context.Background(), map[string]string{SettingCaddyAPIURL: "http://other:2019"}); err == nil {
		t.Fatal("runtime change silently accepted")
	}
	actual, err := svc.GetAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if actual[SettingCertStoragePath] != "/private/certs" {
		t.Fatal("settings differ from active configuration")
	}
}
func TestStreamConflictsRejectBeforeSaving(t *testing.T) {
	db := testDB(t)
	svc := NewStreamHostService(db, nil, NewAuditService(db), nil)
	input := CreateStreamHostInput{IncomingPort: 12000, ForwardHost: "127.0.0.1", ForwardPort: 8080, Protocol: "tcp"}
	if _, err := svc.Create(context.Background(), nil, input, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(context.Background(), nil, input, ""); err == nil {
		t.Fatal("duplicate listener accepted")
	}
	input.Protocol = "udp"
	if _, err := svc.Create(context.Background(), nil, input, ""); err != nil {
		t.Fatal("separate UDP listener rejected", err)
	}
	input.Protocol = "tcp+udp"
	if _, err := svc.Create(context.Background(), nil, input, ""); err == nil {
		t.Fatal("overlapping listener accepted")
	}
	input.IncomingPort = 443
	if _, err := svc.Create(context.Background(), nil, input, ""); err == nil {
		t.Fatal("reserved listener accepted")
	}
}
func TestDeadHostPersistsCertificateAndAccessList(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO certificates(id,name,provider,domain_names) VALUES(7,'cert','auto','["park.test"]');INSERT INTO access_lists(id,name) VALUES(9,'private')`); err != nil {
		t.Fatal(err)
	}
	certID, listID := int64(7), int64(9)
	svc := NewDeadHostService(db, nil, NewAuditService(db), nil)
	host, err := svc.Create(context.Background(), nil, CreateDeadHostInput{DomainNames: json.RawMessage(`["park.test"]`), CertificateID: &certID, AccessListID: &listID}, "")
	if err != nil {
		t.Fatal(err)
	}
	if host.CertificateID == nil || *host.CertificateID != 7 || host.AccessListID == nil || *host.AccessListID != 9 {
		t.Fatal("security selections were not persisted")
	}
}
