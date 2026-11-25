package agent

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4 * 1024 * 1024 // 4 MB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Token auth happens before upgrade, so all origins are acceptable here.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// AgentConn represents one connected agent.
type AgentConn struct {
	serverID int64
	conn     *websocket.Conn
	send     chan Message
}

// Hub is the controller-side registry of connected agents.
// It is safe for concurrent use.
type Hub struct {
	mu     sync.RWMutex
	agents map[int64]*AgentConn // serverID → connection

	// onStatus is called when a status message arrives from an agent.
	onStatus func(serverID int64, payload StatusPayload)

	// Every authenticated connection reconciles against durable desired state.
	onConnect       func(serverID int64)
	onCommandResult func(serverID int64, result CommandResultPayload)
}

func (h *Hub) SetCommandResultHandler(handler func(int64, CommandResultPayload)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onCommandResult = handler
}

// NewHub creates an empty Hub.
// onStatus is invoked on every heartbeat/status message from any agent.
// onConnect is invoked on every connection.
func NewHub(onStatus func(int64, StatusPayload), onConnect func(int64)) *Hub {
	if onStatus == nil {
		onStatus = func(int64, StatusPayload) {}
	}
	if onConnect == nil {
		onConnect = func(int64) {}
	}
	return &Hub{
		agents:    make(map[int64]*AgentConn),
		onStatus:  onStatus,
		onConnect: onConnect,
	}
}

// ServeWS upgrades an HTTP connection to a WebSocket and registers the agent.
// serverID must have been validated by the caller (token check in handler).
func (h *Hub) ServeWS(serverID int64, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Int64("server_id", serverID).Msg("ws upgrade failed")
		return
	}

	ac := &AgentConn{
		serverID: serverID,
		conn:     conn,
		send:     make(chan Message, 64),
	}

	h.mu.Lock()
	// Close any existing connection for this server before registering the new one.
	if old, ok := h.agents[serverID]; ok {
		old.conn.Close()
	}
	h.agents[serverID] = ac
	h.mu.Unlock()

	log.Info().Int64("server_id", serverID).Msg("agent connected")

	go h.onConnect(serverID)

	go ac.writePump()
	ac.readPump(h)
}

// Send dispatches a message to the agent for the given server.
// The caller retains durable pending work when dispatch fails.
func (h *Hub) Send(serverID int64, msg Message) bool {
	h.mu.RLock()
	ac, ok := h.agents[serverID]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	select {
	case ac.send <- msg:
		return true
	default:
		// The reconciliation worker retries a saturated connection.
		return false
	}
}

// IsOnline reports whether an agent with serverID is currently connected.
func (h *Hub) IsOnline(serverID int64) bool {
	h.mu.RLock()
	_, ok := h.agents[serverID]
	h.mu.RUnlock()
	return ok
}

func (h *Hub) Disconnect(serverID int64) {
	if h == nil {
		return
	}
	h.mu.Lock()
	ac := h.agents[serverID]
	delete(h.agents, serverID)
	h.mu.Unlock()
	if ac != nil && ac.conn != nil {
		ac.conn.Close()
	}
}

// MarkOnlineForTest registers a placeholder AgentConn for the given server so
// IsOnline returns true without requiring a real WebSocket. Tests only.
func (h *Hub) MarkOnlineForTest(serverID int64) {
	h.mu.Lock()
	h.agents[serverID] = &AgentConn{serverID: serverID}
	h.mu.Unlock()
}

// ── per-connection pumps ──────────────────────────────────────────────────────

func (ac *AgentConn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		ac.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-ac.send:
			ac.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				ac.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := ac.conn.WriteJSON(msg); err != nil {
				return
			}
		case <-ticker.C:
			ac.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ac.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (ac *AgentConn) readPump(h *Hub) {
	defer func() {
		h.mu.Lock()
		if h.agents[ac.serverID] == ac {
			delete(h.agents, ac.serverID)
		}
		h.mu.Unlock()
		ac.conn.Close()
		log.Info().Int64("server_id", ac.serverID).Msg("agent disconnected")
	}()

	ac.conn.SetReadLimit(maxMessageSize)
	ac.conn.SetReadDeadline(time.Now().Add(pongWait))
	ac.conn.SetPongHandler(func(string) error {
		ac.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		var msg Message
		if err := ac.conn.ReadJSON(&msg); err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Warn().Err(err).Int64("server_id", ac.serverID).Msg("agent read error")
			}
			return
		}
		h.handleAgentMessage(ac, msg)
	}
}

func (h *Hub) handleAgentMessage(ac *AgentConn, msg Message) {
	switch msg.Kind {
	case KindStatus:
		var p StatusPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			h.onStatus(ac.serverID, p)
		}
	case KindCommandResult:
		var p CommandResultPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			h.mu.RLock()
			current := h.agents[ac.serverID] == ac
			handler := h.onCommandResult
			h.mu.RUnlock()
			if current && handler != nil {
				handler(ac.serverID, p)
			}
		}
	case KindHealthReport:
		// Could persist metrics; for now just log.
		log.Debug().Int64("server_id", ac.serverID).Msg("health report received")
	}
}
