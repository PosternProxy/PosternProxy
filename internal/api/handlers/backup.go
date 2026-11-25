package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/posternproxy/posternproxy/internal/auth"
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

	h.audit(r, "export", map[string]any{"format": "1.0"})
	w.Header().Set("Cache-Control", "no-store")
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
	var committed *services.RestoreApplyError
	if err != nil && !errors.As(err, &committed) {
		respondServiceError(w, 400, err)
		return
	}
	h.restoreResult(w, r, restored, err)
}

func (h *BackupHandler) ExportPortable(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Passphrase string `json:"passphrase"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, 400, "invalid request")
		return
	}
	data, err := h.backup.ExportPortable(r.Context(), input.Passphrase)
	if err != nil {
		respondServiceError(w, 400, err)
		return
	}
	h.audit(r, "export", map[string]any{"format": "2.0"})
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="posternproxy-encrypted-backup.json"`)
	w.Write(data)
}
func (h *BackupHandler) RestorePortable(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Archive json.RawMessage `json:"archive"`
		services.PortableRestoreOptions
	}
	r.Body = http.MaxBytesReader(w, r.Body, 51<<20)
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, 400, "invalid restore request")
		return
	}
	restored, err := h.backup.RestorePortable(r.Context(), input.Archive, input.PortableRestoreOptions)
	var committed *services.RestoreApplyError
	if err != nil && !errors.As(err, &committed) {
		respondServiceError(w, 400, err)
		return
	}
	h.restoreResult(w, r, restored, err)
}
func (h *BackupHandler) restoreResult(w http.ResponseWriter, r *http.Request, restored int, applyErr error) {
	states, err := h.backup.ConfigurationStates(r.Context())
	synced := err == nil && applyErr == nil && len(states) > 0
	for _, state := range states {
		if state.Status != "applied" || state.AppliedRevision != state.DesiredRevision {
			synced = false
		}
	}
	h.audit(r, "restore", map[string]any{"restored": restored, "synced": synced})
	w.Header().Set("Cache-Control", "no-store")
	respondJSON(w, 200, map[string]any{"restored": restored, "committed": true, "synced": synced, "configuration": states})
}
func (h *BackupHandler) audit(r *http.Request, action string, details map[string]any) {
	claims := requireBackupActor(r)
	if claims != nil {
		h.backup.Audit(r.Context(), claims, action, details, r.RemoteAddr)
	}
}
func requireBackupActor(r *http.Request) *int64 {
	claims := auth.UserFromContext(r.Context())
	if claims == nil {
		return nil
	}
	return &claims.UserID
}
