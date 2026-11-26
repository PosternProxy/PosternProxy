package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/posternproxy/posternproxy/internal/models"
)

func newProxyHostSvc(t *testing.T) *ProxyHostService {
	t.Helper()
	db := testDB(t)
	audit := NewAuditService(db)
	history := NewConfigHistoryService(db, [32]byte{1})
	return NewProxyHostService(db, nullCaddy(), audit, history, nil)
}

func baseProxyHostInput() models.CreateProxyHostInput {
	return models.CreateProxyHostInput{
		DomainNames:   json.RawMessage(`["example.com"]`),
		ForwardScheme: "http",
		ForwardHost:   "192.168.1.10",
		ForwardPort:   8080,
	}
}

func TestProxyHostService_CreateAndGetByID(t *testing.T) {
	svc := newProxyHostSvc(t)

	input := baseProxyHostInput()
	host, err := svc.Create(context.Background(), nil, input, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if host.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if host.ForwardHost != "192.168.1.10" {
		t.Errorf("forward_host = %q, want 192.168.1.10", host.ForwardHost)
	}
	if host.ForwardPort != 8080 {
		t.Errorf("forward_port = %d, want 8080", host.ForwardPort)
	}
	if host.CaddyID == "" {
		t.Error("caddy_id should be set after create")
	}
	if !host.Enabled {
		t.Error("host should be enabled by default")
	}

	got, err := svc.GetByID(context.Background(), host.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}
	if got.ForwardHost != host.ForwardHost {
		t.Errorf("GetByID forward_host = %q", got.ForwardHost)
	}
}

func TestProxyHostService_GetByID_NotFound(t *testing.T) {
	svc := newProxyHostSvc(t)
	got, err := svc.GetByID(context.Background(), 9999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestProxyHostService_Create_HTTPOnly(t *testing.T) {
	svc := newProxyHostSvc(t)

	input := baseProxyHostInput()
	input.HTTPOnly = true
	host, err := svc.Create(context.Background(), nil, input, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !host.HTTPOnly {
		t.Error("http_only should be true")
	}

	got, _ := svc.GetByID(context.Background(), host.ID)
	if !got.HTTPOnly {
		t.Error("http_only not persisted")
	}
}

func TestProxyHostService_Create_TLSPassthrough(t *testing.T) {
	svc := newProxyHostSvc(t)

	input := baseProxyHostInput()
	input.TLSPassthrough = true
	host, err := svc.Create(context.Background(), nil, input, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !host.TLSPassthrough {
		t.Error("tls_passthrough should be true")
	}

	got, _ := svc.GetByID(context.Background(), host.ID)
	if !got.TLSPassthrough {
		t.Error("tls_passthrough not persisted")
	}
}

func TestProxyHostService_Create_ValidationErrors(t *testing.T) {
	svc := newProxyHostSvc(t)

	tests := []struct {
		name  string
		input models.CreateProxyHostInput
	}{
		{
			name: "empty domain names",
			input: models.CreateProxyHostInput{
				DomainNames: json.RawMessage(`[]`),
				ForwardHost: "10.0.0.1",
				ForwardPort: 80,
			},
		},
		{
			name: "missing forward host",
			input: models.CreateProxyHostInput{
				DomainNames: json.RawMessage(`["x.com"]`),
				ForwardPort: 80,
			},
		},
		{
			name: "invalid forward port",
			input: models.CreateProxyHostInput{
				DomainNames: json.RawMessage(`["x.com"]`),
				ForwardHost: "10.0.0.1",
				ForwardPort: 0,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), nil, tc.input, "")
			if err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestProxyHostService_List(t *testing.T) {
	svc := newProxyHostSvc(t)

	for i := 0; i < 3; i++ {
		_, err := svc.Create(context.Background(), nil, baseProxyHostInput(), "")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	hosts, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(hosts) < 3 {
		t.Errorf("List returned %d hosts, want >= 3", len(hosts))
	}
}

func TestProxyHostService_Update(t *testing.T) {
	svc := newProxyHostSvc(t)

	host, _ := svc.Create(context.Background(), nil, baseProxyHostInput(), "")

	updated, err := svc.Update(context.Background(), nil, host.ID, models.CreateProxyHostInput{
		DomainNames:   json.RawMessage(`["updated.com"]`),
		ForwardScheme: "https",
		ForwardHost:   "10.10.10.10",
		ForwardPort:   9090,
		HTTPOnly:      true,
	}, "")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.ForwardHost != "10.10.10.10" {
		t.Errorf("forward_host = %q", updated.ForwardHost)
	}
	if updated.ForwardPort != 9090 {
		t.Errorf("forward_port = %d", updated.ForwardPort)
	}
	if !updated.HTTPOnly {
		t.Error("http_only should be true after update")
	}
}

func TestProxyHostService_Update_NotFound(t *testing.T) {
	svc := newProxyHostSvc(t)
	_, err := svc.Update(context.Background(), nil, 9999, baseProxyHostInput(), "")
	if err == nil {
		t.Error("expected error updating non-existent host")
	}
}

func TestProxyHostService_Delete(t *testing.T) {
	svc := newProxyHostSvc(t)

	host, _ := svc.Create(context.Background(), nil, baseProxyHostInput(), "")
	if err := svc.Delete(context.Background(), nil, host.ID, ""); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, _ := svc.GetByID(context.Background(), host.ID)
	if got != nil {
		t.Error("host still exists after Delete")
	}
}

func TestProxyHostService_SetEnabled(t *testing.T) {
	svc := newProxyHostSvc(t)

	host, _ := svc.Create(context.Background(), nil, baseProxyHostInput(), "")
	if !host.Enabled {
		t.Fatal("host should start enabled")
	}

	if err := svc.SetEnabled(context.Background(), nil, host.ID, false, ""); err != nil {
		t.Fatalf("SetEnabled false: %v", err)
	}
	got, _ := svc.GetByID(context.Background(), host.ID)
	if got.Enabled {
		t.Error("host should be disabled")
	}

	if err := svc.SetEnabled(context.Background(), nil, host.ID, true, ""); err != nil {
		t.Fatalf("SetEnabled true: %v", err)
	}
	got, _ = svc.GetByID(context.Background(), host.ID)
	if !got.Enabled {
		t.Error("host should be re-enabled")
	}
}

func TestProxyHostService_DefaultsApplied(t *testing.T) {
	svc := newProxyHostSvc(t)

	input := baseProxyHostInput()
	// Leave LBPolicy, RateLimitWindow, Locations, ErrorPages, CustomHeaders empty
	host, err := svc.Create(context.Background(), nil, input, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if host.LBPolicy == "" {
		t.Error("lb_policy should have a default")
	}
	if host.RateLimitWindow == "" {
		t.Error("rate_limit_window should have a default")
	}
}
