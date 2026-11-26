package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/posternproxy/posternproxy/internal/services"
)

// JoinTokenHandler exposes the admin endpoints for minting and revoking
// single-use join tokens used by remote agents on first connect.
type JoinTokenHandler struct {
	svc      *services.JoinTokenService
	servers  *services.ServerService
	settings *services.SettingsService
	audit    *services.AuditService
}

// NewJoinTokenHandler constructs a JoinTokenHandler.
func NewJoinTokenHandler(svc *services.JoinTokenService, servers *services.ServerService, settings *services.SettingsService, audit *services.AuditService) *JoinTokenHandler {
	return &JoinTokenHandler{svc: svc, servers: servers, settings: settings, audit: audit}
}

type issueJoinTokenRequest struct {
	TTLSeconds int `json:"ttl_seconds"`
}

type issueJoinTokenResponse struct {
	Token          string `json:"token"`
	ExpiresAt      string `json:"expires_at"`
	InstallCommand string `json:"install_command,omitempty"`
}

// Issue handles POST /api/servers/:id/join-token.
//
//	201: { token, expires_at, install_command }
//	409: server already joined — caller must POST /reset-join first.
func (h *JoinTokenHandler) Issue(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	serverID, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req issueJoinTokenRequest
	// Body is optional. Empty / missing body → defaults.
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	ttl := time.Duration(req.TTLSeconds) * time.Second
	raw, expiresAt, err := h.svc.Issue(r.Context(), &claims.UserID, serverID, ttl)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrServerNotFound):
			respondError(w, http.StatusNotFound, "server not found")
		case errors.Is(err, services.ErrServerAlreadyJoined):
			respondJSON(w, http.StatusConflict, map[string]string{
				"error": "server already joined",
				"hint":  "POST /api/servers/{id}/reset-join first to invalidate the existing agent's credentials",
			})
		default:
			respondError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	h.audit.Log(r.Context(), &claims.UserID, "join_token.issue", "server", &serverID, nil, r.RemoteAddr)

	respondJSON(w, http.StatusCreated, issueJoinTokenResponse{
		Token:     raw,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	})
}

// Revoke handles DELETE /api/servers/:id/join-token. It deletes all
// outstanding (unconsumed) join tokens for the server. 204 on success.
func (h *JoinTokenHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	serverID, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	n, err := h.svc.RevokeForServer(r.Context(), serverID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit.Log(r.Context(), &claims.UserID, "join_token.revoke", "server", &serverID, map[string]any{"count": n}, r.RemoteAddr)
	w.WriteHeader(http.StatusNoContent)
}
