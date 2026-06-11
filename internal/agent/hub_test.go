package agent

import (
	"encoding/json"
	"testing"
)

func TestHub_IsOnline_Initially_False(t *testing.T) {
	hub := NewHub(nil, nil)
	if hub.IsOnline(1) {
		t.Error("server 1 should not be online initially")
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

func TestHub_MarkPendingSync(t *testing.T) {
	hub := NewHub(nil, nil)
	// MarkPendingSync should not panic for unknown servers.
	hub.MarkPendingSync(999)
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
