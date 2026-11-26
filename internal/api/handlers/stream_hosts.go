package handlers

import (
	"context"
	"net/http"

	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

// StreamHostHandler provides HTTP handler methods for the StreamHost resource.
type StreamHostHandler struct {
	svc *services.StreamHostService
}

// NewStreamHostHandler creates a StreamHostHandler backed by the given service.
func NewStreamHostHandler(svc *services.StreamHostService) *StreamHostHandler {
	return &StreamHostHandler{svc: svc}
}

// List writes all StreamHost records to the response.
// Optional ?server_id= query param filters results to a single server.
func (h *StreamHostHandler) List(w http.ResponseWriter, r *http.Request) {
	items, ok := listWithServerFilter(w, r,
		func(ctx context.Context) (any, error) { return h.svc.List(ctx) },
		func(ctx context.Context, sid int64) (any, error) { return h.svc.ListByServer(ctx, sid) },
		"stream hosts",
	)
	if !ok {
		return
	}
	if items == nil {
		items = []models.StreamHost{}
	}
	respondJSON(w, http.StatusOK, items)
}

// Create decodes the request body and inserts a new StreamHost record.
func (h *StreamHostHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	var input services.CreateStreamHostInput
	if !decodeOrFail(w, r, &input) {
		return
	}
	host, err := h.svc.Create(r.Context(), &claims.UserID, input, r.RemoteAddr)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusCreated, host)
}

// Get writes a single StreamHost record looked up by URL parameter "id".
func (h *StreamHostHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	host, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get stream host")
		return
	}
	if host == nil {
		respondError(w, http.StatusNotFound, "stream host not found")
		return
	}
	respondJSON(w, http.StatusOK, host)
}

// Update decodes the request body and replaces an existing StreamHost record.
func (h *StreamHostHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var input services.CreateStreamHostInput
	if !decodeOrFail(w, r, &input) {
		return
	}
	host, err := h.svc.Update(r.Context(), &claims.UserID, id, input, r.RemoteAddr)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, host)
}

// Delete removes a StreamHost record by URL parameter "id".
func (h *StreamHostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), &claims.UserID, id, r.RemoteAddr); err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Enable activates a StreamHost record by URL parameter "id".
func (h *StreamHostHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, true)
}

// Disable deactivates a StreamHost record by URL parameter "id".
func (h *StreamHostHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, false)
}

func (h *StreamHostHandler) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	if err := h.svc.SetEnabled(r.Context(), &claims.UserID, id, enabled, r.RemoteAddr); err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	host, _ := h.svc.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, host)
}
