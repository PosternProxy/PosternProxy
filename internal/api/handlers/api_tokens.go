package handlers

import (
	"net/http"

	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

// APITokenHandler handles /api/tokens endpoints.
type APITokenHandler struct {
	svc *services.APITokenService
}

// NewAPITokenHandler creates an APITokenHandler backed by the given service.
func NewAPITokenHandler(svc *services.APITokenService) *APITokenHandler {
	return &APITokenHandler{svc: svc}
}

// List returns all tokens visible to the caller.
// Admins see every token; regular users see only their own.
func (h *APITokenHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	tokens, err := h.svc.List(r.Context(), claims.UserID, claims.Role == "admin")
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, tokens)
}

// Create generates a new API token and returns the raw secret exactly once.
func (h *APITokenHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}

	var input models.CreateAPITokenInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	tok, err := h.svc.Create(r.Context(), claims.UserID, claims.Role, input)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.svc.Audit(r.Context(), claims.UserID, "create", &tok.ID, r.RemoteAddr)
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, http.StatusCreated, tok)
}

// Revoke soft-deletes a token.
func (h *APITokenHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}

	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.svc.Revoke(r.Context(), id, claims.UserID, claims.Role == "admin"); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.svc.Audit(r.Context(), claims.UserID, "revoke", &id, r.RemoteAddr)
	w.WriteHeader(http.StatusNoContent)
}
