package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/caddy"
)

const (
	ntlmMissingSentence  = "Caddy rejected the configuration because the NTLM transport module is missing. Upgrade Caddy on this server and retry sync."
	localGenericSentence = "Configuration application failed; retry or check server connectivity."
	agentGenericSentence = "Agent rejected configuration. Check the server and retry synchronization."
	ntlmModulePhrase     = "loading config: loading new config: loading module 'http_ntlm': unknown module: http.reverse_proxy.transport.http_ntlm"
)

func TestApplyFailureSentenceClassifiesOnlyTheNTLMModule(t *testing.T) {
	moduleJSON := `{"error":"` + ntlmModulePhrase + `"}` + "\n"
	shorterJSON := `{"error":"loading config: loading new config: unknown module: http.reverse_proxy.transport.http_ntlm"}` + "\n"
	tests := []struct {
		name    string
		raw     string
		generic string
		want    string
	}{
		{name: "controller wrapper", raw: "caddy POST /load returned 400: " + moduleJSON, generic: localGenericSentence, want: ntlmMissingSentence},
		{name: "agent wrapper", raw: "caddy POST /load: status 400: " + moduleJSON, generic: agentGenericSentence, want: ntlmMissingSentence},
		{name: "agent status 500", raw: "caddy POST /load: status 500: " + shorterJSON, generic: agentGenericSentence, want: ntlmMissingSentence},
		{name: "other module", raw: "caddy POST /load returned 400: {\"error\":\"unknown module: http.handlers.rate_limit\"}\n", generic: localGenericSentence, want: localGenericSentence},
		{name: "module not registered", raw: "caddy POST /load returned 400: {\"error\":\"module not registered\"}\n", generic: localGenericSentence, want: localGenericSentence},
		{name: "registered phrase without unknown module", raw: "caddy POST /load returned 400: {\"error\":\"module not registered: http.reverse_proxy.transport.http_ntlm\"}\n", generic: localGenericSentence, want: localGenericSentence},
		{name: "generated protocol only", raw: `{"apps":{"http":{"routes":[{"handle":[{"transport":{"protocol":"http_ntlm"}}]}]}}}`, generic: localGenericSentence, want: localGenericSentence},
		{name: "module id outside error", raw: `{"note":"unknown module: http.reverse_proxy.transport.http_ntlm"}`, generic: localGenericSentence, want: localGenericSentence},
		{name: "secret body", raw: "SECRET-REJECTION-BODY", generic: localGenericSentence, want: localGenericSentence},
		{name: "private key sentinel", raw: "PRIVATE KEY SENTINEL", generic: agentGenericSentence, want: agentGenericSentence},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := applyFailureSentence(tc.raw, tc.generic)
			if got != tc.want || strings.Contains(got, "SECRET") || strings.Contains(got, "PRIVATE") || strings.Contains(got, "unknown module:") {
				t.Fatalf("got %q", got)
			}
		})
	}
}

func TestSyncServerStoresNTLMModuleSentenceWithoutApplying(t *testing.T) {
	db := testDB(t)
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"` + ntlmModulePhrase + `"}` + "\n"))
	}))
	defer server.Close()
	hosts := NewProxyHostService(db, nil, NewAuditService(db), nil, nil)
	input := baseProxyHostInput()
	input.NTLMEnabled = true
	host, err := hosts.Create(context.Background(), nil, input, "test")
	if err != nil {
		t.Fatal(err)
	}
	sync := NewCaddySyncService(db, caddy.NewClient(server.URL), nil, "")
	err = sync.SyncServer(context.Background(), 1)
	var apply *ConfigApplyError
	if !errors.As(err, &apply) || strings.Contains(err.Error(), "unknown module") || strings.Contains(err.Error(), "http_ntlm") {
		t.Fatalf("apply error: %v", err)
	}
	state, err := sync.State(context.Background(), 1)
	if err != nil || state.Status != "failed" || state.AppliedRevision != 0 || state.Error != ntlmMissingSentence || hits == 0 {
		t.Fatalf("state=%+v hits=%d err=%v", state, hits, err)
	}
	got, err := hosts.GetByID(context.Background(), host.ID)
	if err != nil || got == nil || !got.NTLMEnabled {
		t.Fatalf("host lost: %+v %v", got, err)
	}
}

func TestPoisonedNTLMRowBlocksApplyAndKeepsGenericSentence(t *testing.T) {
	db := testDB(t)
	hits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if _, err := db.Exec(`INSERT INTO proxy_hosts (domain_names, forward_host, forward_port, ntlm_enabled) VALUES ('["good.test"]','127.0.0.1',8080,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO proxy_hosts (domain_names, forward_host, forward_port, ntlm_enabled, locations) VALUES ('["poison.test"]','127.0.0.1',8081,1,'[{"path":"/x"}]')`); err != nil {
		t.Fatal(err)
	}
	sync := NewCaddySyncService(db, caddy.NewClient(server.URL), nil, "")
	err := sync.SyncServer(context.Background(), 1)
	var apply *ConfigApplyError
	if !errors.As(err, &apply) || err.Error() != "Configuration saved, but server 1 has not applied it. Check configuration status and retry synchronization." {
		t.Fatalf("apply error: %v", err)
	}
	if strings.Contains(err.Error(), "path locations") {
		t.Fatal(err)
	}
	state, err := sync.State(context.Background(), 1)
	if err != nil || state.Status != "failed" || state.AppliedRevision != 0 || state.Error != localGenericSentence || hits != 0 {
		t.Fatalf("state=%+v hits=%d err=%v", state, hits, err)
	}
	var locations string
	var flag int
	if err := db.QueryRow(`SELECT ntlm_enabled, locations FROM proxy_hosts WHERE domain_names='["poison.test"]'`).Scan(&flag, &locations); err != nil {
		t.Fatal(err)
	}
	if flag != 1 || locations != `[{"path":"/x"}]` {
		t.Fatalf("poisoned row rewritten: %d %s", flag, locations)
	}
}

func TestAgentNTLMRejectionStoresSentenceOnly(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO servers (id,name,hostname) VALUES (2,'remote','remote.test')`); err != nil {
		t.Fatal(err)
	}
	sync := NewCaddySyncService(db, nil, nil, "")
	sync.pending[2] = configAttempt{ID: "command-ntlm", Revision: 1, Deadline: time.Now().Add(time.Minute)}
	sync.commandResult(2, agent.CommandResultPayload{
		CommandID:   "command-ntlm",
		Revision:    1,
		CommandKind: agent.KindConfigFullSync,
		Success:     false,
		Error:       "caddy POST /load: status 400: {\"error\":\"" + ntlmModulePhrase + "\"}\n",
	})
	state, err := sync.State(context.Background(), 2)
	if err != nil || state.Status != "failed" || state.AppliedRevision != 0 || state.Error != ntlmMissingSentence || strings.Contains(state.Error, "unknown module") {
		t.Fatalf("agent state: %+v %v", state, err)
	}
}
