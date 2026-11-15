package handlers

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/posternproxy/posternproxy/internal/services"
)

// BackupHandler provides HTTP handler methods for backup export and import.
type BackupHandler struct {
	backup *services.BackupService
}

// NewBackupHandler creates a BackupHandler backed by the given service.
func NewBackupHandler(backup *services.BackupService) *BackupHandler {
	return &BackupHandler{backup: backup}
}

// Export writes a JSON backup of all configuration tables as a file download.
func (h *BackupHandler) Export(w http.ResponseWriter, r *http.Request) {
	payload, err := h.backup.Export(r.Context())
	if err != nil {
		http.Error(w, "export failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := services.MarshalPayload(payload)
	if err != nil {
		http.Error(w, "marshal failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("posternproxy-backup-%s.json", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.WriteHeader(http.StatusOK)
	// A write error here means the client disconnected mid-response;
	// nothing actionable for us to do.
	_, _ = w.Write(data)
}

// Import accepts a JSON backup file in the request body and restores all configuration tables.
func (h *BackupHandler) Import(w http.ResponseWriter, r *http.Request) {
	const maxSize = 50 << 20 // 50 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}

	payload, err := services.UnmarshalPayload(data)
	if err != nil {
		http.Error(w, "invalid backup file: "+err.Error(), http.StatusBadRequest)
		return
	}

	restored, err := h.backup.Import(r.Context(), payload)
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}
	states, err := h.backup.ConfigurationStates(r.Context())
	if err != nil {
		respondServiceError(w, http.StatusInternalServerError, err)
		return
	}
	synced := len(states) > 0
	for _, state := range states {
		if state.Status != "applied" || state.AppliedRevision != state.DesiredRevision {
			synced = false
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"restored":      restored,
		"synced":        synced,
		"configuration": states,
	})
}
