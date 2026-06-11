package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/posternproxy/posternproxy/internal/services"
)

// SettingsHandler handles /api/settings endpoints.
type SettingsHandler struct {
	svc *services.SettingsService
}

// NewSettingsHandler creates a SettingsHandler backed by the given service.
func NewSettingsHandler(svc *services.SettingsService) *SettingsHandler {
	return &SettingsHandler{svc: svc}
}

// List returns all settings as a JSON object { "key": "value", ... }.
func (h *SettingsHandler) List(w http.ResponseWriter, r *http.Request) {
	settings, err := h.svc.GetAll(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, settings)
}

// Update accepts a JSON object of key→value pairs and upserts them.
// Only allowed keys are accepted; unknown keys return 400.
func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var updates map[string]string
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(updates) == 0 {
		respondError(w, http.StatusBadRequest, "no settings provided")
		return
	}
	if err := h.svc.SetBulk(r.Context(), updates); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Return the fresh full settings map.
	settings, err := h.svc.GetAll(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, settings)
}
