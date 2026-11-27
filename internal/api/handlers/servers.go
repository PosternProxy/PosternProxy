package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

// ServerHandler handles server management endpoints.
type ServerHandler struct {
	svc  *services.ServerService
	hub  *agent.Hub
	sync *services.CaddySyncService
}

// NewServerHandler creates a ServerHandler backed by the given service, agent hub, and sync service.
func NewServerHandler(svc *services.ServerService, hub *agent.Hub, sync *services.CaddySyncService) *ServerHandler {
	return &ServerHandler{svc: svc, hub: hub, sync: sync}
}

// List returns all registered servers with their online status.
func (h *ServerHandler) List(w http.ResponseWriter, r *http.Request) {
	servers, err := h.svc.List(r.Context())
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}
	if servers == nil {
		respondJSON(w, http.StatusOK, []any{})
		return
	}
	respondJSON(w, http.StatusOK, servers)
}

// Get returns a single server.
func (h *ServerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	srv, err := h.svc.GetByID(r.Context(), id)
	if err != nil || srv == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	respondJSON(w, http.StatusOK, srv)
}

// Create registers a new server and starts provisioning.
// The response is the new server record immediately (status=provisioning).
// The caller should then open the SSE stream at /api/servers/{id}/provision-log
// to watch progress.
func (h *ServerHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	var input models.CreateServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	srv, err := h.svc.Create(r.Context(), &claims.UserID, input, r.RemoteAddr)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusCreated, srv)
}

// Update changes name / SSH credentials for an existing server.
func (h *ServerHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var input models.CreateServerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.UpdateCredentials(r.Context(), &claims.UserID, id, input, r.RemoteAddr); err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	srv, _ := h.svc.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, srv)
}

// Delete removes a server (not the local one).
func (h *ServerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.svc.Delete(r.Context(), &claims.UserID, id, r.RemoteAddr); err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ProvisionLog streams the provisioning log for a server as Server-Sent Events.
// It attaches to an existing in-memory ProvisionRun, or replays the stored log
// from the DB if no run is active. It never starts a new run.
func (h *ServerHandler) ProvisionLog(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	srv, err := h.svc.GetByID(r.Context(), id)
	if err != nil || srv == nil {
		respondError(w, http.StatusNotFound, "server not found")
		return
	}

	run := h.svc.GetProvisionRun(id)
	if run == nil || run.Done() {
		// No live run — replay the stored log and emit a done event so the
		// frontend stops spinning.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		for _, line := range splitLines(srv.ProvisionLog) {
			fmt.Fprintf(w, "data: %s\n\n", line)
		}
		status := srv.Status
		if status == "provisioning" {
			// Stale "provisioning" status with no in-memory run means the
			// controller restarted mid-run. Surface as error so the user can
			// manually reprovision instead of waiting forever.
			status = "error"
		}
		fmt.Fprintf(w, "event: done\ndata: %s\n\n", status) // #nosec G705 -- SSE text/event-stream, not HTML; status is a server-generated value
		return
	}

	streamRun(w, r, run)
}

// Reprovision triggers a fresh provisioning run for an existing server and
// streams the log over SSE. If a run is already in progress, the response is
// 409 Conflict — clients should fall back to GET /provision-log to watch the
// existing run.
func (h *ServerHandler) Reprovision(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.Reprovision(r.Context(), &claims.UserID, id, r.RemoteAddr); err != nil {
		if errors.Is(err, services.ErrProvisioningInProgress) {
			respondServiceError(w, http.StatusConflict, err)
			return
		}
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}

	run := h.svc.GetProvisionRun(id)
	if run == nil {
		// Should not happen — Reprovision just claimed a slot.
		respondError(w, http.StatusInternalServerError, "run not registered")
		return
	}
	streamRun(w, r, run)
}

// streamRun is the shared SSE streaming loop used by both ProvisionLog and
// Reprovision. It first replays the run's accumulated history, then streams
// live log lines until the run finishes, the client disconnects, or the
// overall timeout fires.
func streamRun(w http.ResponseWriter, r *http.Request, run *services.ProvisionRun) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, canFlush := w.(http.Flusher)

	history, live := run.Subscribe()
	for _, line := range history {
		fmt.Fprintf(w, "data: %s\n\n", line) // #nosec G705 -- SSE text/event-stream, not HTML; line is a server-generated log line
	}
	if canFlush {
		flusher.Flush()
	}

	timeout := time.After(20 * time.Minute)
	for {
		select {
		case line, ok := <-live:
			if !ok {
				// Run finished — derive a status from its Result.
				status := "error"
				if res, done := run.Result(); done && res.Success {
					status = "online"
				}
				fmt.Fprintf(w, "event: done\ndata: %s\n\n", status)
				if canFlush {
					flusher.Flush()
				}
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", line)
			if canFlush {
				flusher.Flush()
			}
		case <-r.Context().Done():
			return
		case <-timeout:
			fmt.Fprintf(w, "event: done\ndata: timeout\n\n")
			return
		}
	}
}

// Resync pushes the current Caddy config to a remote agent over its live WebSocket.
func (h *ServerHandler) Resync(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	h.svc.AuditResync(r.Context(), claims.UserID, id, r.RemoteAddr)
	if err := h.sync.SyncServer(r.Context(), id); err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}
	state, err := h.sync.State(r.Context(), id)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}
	status := http.StatusAccepted
	if state.Status == "applied" {
		status = http.StatusOK
	}
	respondJSON(w, status, state)
}

// SetHTTP2 toggles the server-wide HTTP/2 setting and re-syncs Caddy so it takes
// effect. Body: {"enabled": bool}. Works for the local server too.
func (h *ServerHandler) SetHTTP2(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.SetHTTP2(r.Context(), &claims.UserID, id, body.Enabled, r.RemoteAddr); err != nil {
		if errors.Is(err, services.ErrServerNotFound) {
			respondError(w, http.StatusNotFound, "server not found")
			return
		}
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.sync.SyncServer(r.Context(), id); err != nil {
		// The setting is saved; surface the apply failure so the user knows the
		// change hasn't reached Caddy yet.
		respondError(w, http.StatusBadGateway, "saved but failed to apply to Caddy: "+err.Error())
		return
	}
	srv, _ := h.svc.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, srv)
}

// Status returns the real-time connectivity status of one server.
func (h *ServerHandler) Status(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"server_id": id,
		"online":    h.hub.IsOnline(id),
	})
}

// ResetJoin clears agent_token_hash on the server row so a new join token can
// be minted. Refuses with 409 if the agent is currently connected, unless
// ?force=true is passed.
func (h *ServerHandler) ResetJoin(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	force := r.URL.Query().Get("force") == "true"
	if err := h.svc.ResetJoin(r.Context(), &claims.UserID, id, force, r.RemoteAddr); err != nil {
		switch {
		case errors.Is(err, services.ErrAgentConnected):
			respondJSON(w, http.StatusConflict, map[string]string{
				"error": err.Error(),
				"hint":  "retry with ?force=true to reset even though an agent is online",
			})
		case errors.Is(err, services.ErrServerNotFound):
			respondError(w, http.StatusNotFound, "server not found")
		default:
			respondServiceError(w, http.StatusBadRequest, err)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── WebSocket endpoint for agent connections ──────────────────────────────────

// AgentConnect upgrades the connection to WebSocket for a remote agent.
// The agent supplies server_id in the URL and its token in Authorization.
//
// All failure cases return an identical 401 to prevent server ID enumeration.
func (h *ServerHandler) AgentConnect(w http.ResponseWriter, r *http.Request) {
	serverID, authorize, ok := h.authenticatedAgent(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	h.hub.ServeWS(serverID, w, r, authorize)
}

func (h *ServerHandler) VerifyAgent(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := h.authenticatedAgent(r); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ServerHandler) authenticatedAgent(r *http.Request) (int64, func() bool, bool) {
	serverIDStr := r.URL.Query().Get("server_id")
	parts := strings.Fields(r.Header.Get("Authorization"))
	if r.URL.Query().Has("token") || len(r.Header.Values("Authorization")) != 1 || len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return 0, nil, false
	}
	token := parts[1]

	// ParseInt returning an error yields serverID == 0, which ValidateAgentToken
	// safely rejects (no row + dummy bcrypt). No need for a separate branch.
	serverID, _ := strconv.ParseInt(serverIDStr, 10, 64)
	authorize, valid := h.svc.AgentAuthorization(r.Context(), serverID, token)
	if !valid {
		return 0, nil, false
	}

	return serverID, authorize, true
}

// ── helpers ───────────────────────────────────────────────────────────────────

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func (h *ServerHandler) AgentCredentials(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, err := parseID(r)
	if err != nil {
		respondError(w, 400, "invalid id")
		return
	}
	config, err := h.svc.AgentCredentials(r.Context(), &claims.UserID, id, r.RemoteAddr)
	if err != nil {
		respondServiceError(w, 400, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, 200, config)
}
