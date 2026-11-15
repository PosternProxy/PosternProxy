package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/posternproxy/posternproxy/internal/agent"
)

func TestConfigurationResultFollowsCaddyApplication(t *testing.T) {
	for _, status := range []int{200, 400} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			caddy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release; w.WriteHeader(status) }))
			defer caddy.Close()
			results := make(chan agent.CommandResultPayload, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upgrader := websocket.Upgrader{}
				conn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					return
				}
				defer conn.Close()
				var message agent.Message
				if err := conn.ReadJSON(&message); err != nil {
					return
				}
				var result agent.CommandResultPayload
				if err := json.Unmarshal(message.Payload, &result); err != nil {
					return
				}
				results <- result
			}))
			defer server.Close()
			conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			message, err := agent.NewMessage(agent.KindConfigFullSync, agent.ConfigFullSyncPayload{Config: json.RawMessage(`{}`)})
			if err != nil {
				t.Fatal(err)
			}
			message.ID = "correlation"
			message.Revision = 17
			client := &agentClient{caddyAPI: caddy.URL}
			go client.handleMessage(conn, message)
			select {
			case <-started:
			case <-time.After(3 * time.Second):
				close(release)
				t.Fatal("Caddy was not called")
			}
			select {
			case <-results:
				close(release)
				t.Fatal("acknowledged before Caddy returned")
			default:
			}
			close(release)
			select {
			case result := <-results:
				if result.CommandID != "correlation" || result.Revision != 17 || result.CommandKind != agent.KindConfigFullSync || result.Success != (status == 200) {
					t.Fatalf("incorrect acknowledgment: %+v", result)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("missing acknowledgment")
			}
		})
	}
}
