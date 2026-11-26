package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/posternproxy/posternproxy/internal/auth"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

// ConfigHistoryHandler provides HTTP handler methods for the ConfigHistory resource.
type ConfigHistoryHandler struct {
	history *services.ConfigHistoryService
}

// NewConfigHistoryHandler creates a ConfigHistoryHandler backed by the given service.
func NewConfigHistoryHandler(history *services.ConfigHistoryService) *ConfigHistoryHandler {
	return &ConfigHistoryHandler{history: history}
}

// List writes all ConfigHistory records for the given resource_type and resource_id to the response.
func (h *ConfigHistoryHandler) List(w http.ResponseWriter, r *http.Request) {
	resourceType := r.URL.Query().Get("resource_type")
	resourceIDStr := r.URL.Query().Get("resource_id")

	if resourceType == "" || resourceIDStr == "" {
		http.Error(w, "resource_type and resource_id are required", http.StatusBadRequest)
		return
	}
	resourceID, err := strconv.ParseInt(resourceIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid resource_id", http.StatusBadRequest)
		return
	}

	entries, err := h.history.List(r.Context(), resourceType, resourceID)
	if err != nil {
		http.Error(w, "query failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if entries == nil {
		respondJSON(w, http.StatusOK, []models.ConfigHistory{})
		return
	}
	respondJSON(w, http.StatusOK, entries)
}

// Rollback restores the resource configuration snapshot identified by URL parameter "id".
func (h *ConfigHistoryHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	historyID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	claims := auth.UserFromContext(r.Context())
	var callerID *int64
	if claims != nil {
		uid := claims.UserID
		callerID = &uid
	}

	if err := h.history.Rollback(r.Context(), callerID, historyID, r.RemoteAddr); err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
