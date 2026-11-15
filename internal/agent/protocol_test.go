package agent

import (
	"encoding/json"
	"testing"
)

func TestNewMessage_Roundtrip(t *testing.T) {
	payload := StatusPayload{
		ServerID:     5,
		CaddyVersion: "2.8.4",
		Uptime:       3600,
		ActiveRoutes: 12,
	}

	msg, err := NewMessage(KindStatus, payload)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if msg.Kind != KindStatus {
		t.Errorf("Kind = %q, want %q", msg.Kind, KindStatus)
	}

	var decoded StatusPayload
	if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
		t.Fatalf("Unmarshal payload: %v", err)
	}
	if decoded.ServerID != 5 {
		t.Errorf("ServerID = %d, want 5", decoded.ServerID)
	}
	if decoded.CaddyVersion != "2.8.4" {
		t.Errorf("CaddyVersion = %q, want 2.8.4", decoded.CaddyVersion)
	}
	if decoded.Uptime != 3600 {
		t.Errorf("Uptime = %d, want 3600", decoded.Uptime)
	}
	if decoded.ActiveRoutes != 12 {
		t.Errorf("ActiveRoutes = %d, want 12", decoded.ActiveRoutes)
	}
}

func TestNewMessage_ConfigFullSync(t *testing.T) {
	cfg := json.RawMessage(`{"apps":{"http":{}}}`)
	payload := ConfigFullSyncPayload{
		Config: cfg,
		Certs: []CertFile{
			{CertID: 1, CertPEM: []byte("cert"), KeyPEM: []byte("key")},
		},
	}

	msg, err := NewMessage(KindConfigFullSync, payload)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}
	if msg.Kind != KindConfigFullSync {
		t.Errorf("Kind = %q, want %q", msg.Kind, KindConfigFullSync)
	}

	var decoded ConfigFullSyncPayload
	if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(decoded.Certs) != 1 {
		t.Fatalf("Certs len = %d, want 1", len(decoded.Certs))
	}
	if decoded.Certs[0].CertID != 1 {
		t.Errorf("CertID = %d, want 1", decoded.Certs[0].CertID)
	}
}

func TestNewMessage_CommandResult(t *testing.T) {
	payload := CommandResultPayload{
		CommandKind: KindConfigPatch,
		Success:     true,
		Error:       "",
	}

	msg, err := NewMessage(KindCommandResult, payload)
	if err != nil {
		t.Fatalf("NewMessage: %v", err)
	}

	var decoded CommandResultPayload
	if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !decoded.Success {
		t.Error("expected Success=true")
	}
	if decoded.CommandKind != KindConfigPatch {
		t.Errorf("CommandKind = %q, want %q", decoded.CommandKind, KindConfigPatch)
	}
}
