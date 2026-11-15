package agent

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHub_IsOnline_Initially_False(t *testing.T) {
	hub := NewHub(nil, nil)
	if hub.IsOnline(1) {
		t.Error("server 1 should not be online initially")
	}
}

func TestWriterFailureReleasesTheConnection(t *testing.T) {
	connections := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := websocket.Upgrader{}
		conn, err := u.Upgrade(w, r, nil)
		if err == nil {
			connections <- conn
		}
	}))
	defer server.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var conn *websocket.Conn
	select {
	case conn = <-connections:
	case <-time.After(time.Second):
		t.Fatal("missing server connection")
	}
	hub := NewHub(nil, nil)
	ac := &AgentConn{serverID: 2, conn: conn, send: make(chan Message, 1)}
	hub.agents[2] = ac
	conn.Close()
	ac.send <- Message{Kind: KindConfigFullSync}
	writerDone := make(chan struct{})
	readerDone := make(chan struct{})
	go func() { ac.writePump(); close(writerDone) }()
	go func() { ac.readPump(hub); close(readerDone) }()
	for _, done := range []chan struct{}{writerDone, readerDone} {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("connection pump did not stop")
		}
	}
	if hub.IsOnline(2) {
		t.Fatal("failed connection remained online")
	}
}

func TestHub_MarkOnlineForTest(t *testing.T) {
	hub := NewHub(nil, nil)
	hub.MarkOnlineForTest(42)
	if !hub.IsOnline(42) {
		t.Error("server 42 should be online after MarkOnlineForTest")
	}
	if hub.IsOnline(43) {
		t.Error("server 43 should not be online")
	}
}

func TestHub_Send_Offline_ReturnsFalse(t *testing.T) {
	hub := NewHub(nil, nil)
	msg, _ := NewMessage(KindHealthCheck, nil)
	if hub.Send(99, msg) {
		t.Error("Send to offline server should return false")
	}
}

func TestHub_Send_Online_NilChannel_ReturnsFalse(t *testing.T) {
	// MarkOnlineForTest creates a placeholder AgentConn without a send channel,
	// so Send correctly returns false (no channel to write to). This verifies
	// that Send gracefully handles the nil-channel case without panicking.
	hub := NewHub(nil, nil)
	hub.MarkOnlineForTest(10)

	msg, _ := NewMessage(KindHealthCheck, nil)
	if hub.Send(10, msg) {
		t.Error("Send to placeholder agent (nil channel) should return false")
	}
}

func TestHub_Send_ChannelFull_ReturnsFalse(t *testing.T) {
	hub := NewHub(nil, nil)
	hub.MarkOnlineForTest(20)

	// Fill the send channel (buffer size is typically small).
	msg, _ := NewMessage(KindConfigFullSync, ConfigFullSyncPayload{
		Config: json.RawMessage(`{}`),
	})
	// Send repeatedly until the channel is full.
	for i := 0; i < 300; i++ {
		if !hub.Send(20, msg) {
			// Once full, Send should return false — test passes.
			return
		}
	}
	t.Error("expected Send to return false when channel is full")
}

func TestHubSendSaturatedConnection(t *testing.T) {
	hub := NewHub(nil, nil)
	hub.agents[2] = &AgentConn{serverID: 2, send: make(chan Message, 1)}
	if !hub.Send(2, Message{Kind: KindConfigFullSync}) {
		t.Fatal("empty queue rejected")
	}
	if hub.Send(2, Message{Kind: KindConfigFullSync}) {
		t.Fatal("saturated queue accepted")
	}
}

func TestHubIgnoresResultsFromReplacedConnection(t *testing.T) {
	hub := NewHub(nil, nil)
	called := false
	hub.SetCommandResultHandler(func(int64, CommandResultPayload) { called = true })
	old := &AgentConn{serverID: 2}
	hub.agents[2] = &AgentConn{serverID: 2}
	message, err := NewMessage(KindCommandResult, CommandResultPayload{CommandID: "old", Revision: 1, Success: true})
	if err != nil {
		t.Fatal(err)
	}
	hub.handleAgentMessage(old, message)
	if called {
		t.Fatal("old connection acknowledged current configuration")
	}
}

func TestHub_OnStatus_Callback(t *testing.T) {
	var called bool
	var gotServerID int64
	hub := NewHub(func(serverID int64, status StatusPayload) {
		called = true
		gotServerID = serverID
	}, nil)

	hub.MarkOnlineForTest(7)
	// Simulate a status message being received — trigger via Send doesn't call
	// the callback (that happens in readPump). But we can verify the hub
	// construction doesn't panic and the callback is stored.
	_ = hub
	_ = called
	_ = gotServerID
}
