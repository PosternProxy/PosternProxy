package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

// PortForwardHandler handles CRUD and enable/disable for port forwarding rules.
type PortForwardHandler struct {
	svc *services.PortForwardService
}

// NewPortForwardHandler creates a PortForwardHandler backed by the given service.
func NewPortForwardHandler(svc *services.PortForwardService) *PortForwardHandler {
	return &PortForwardHandler{svc: svc}
}

// List writes all PortForward records to the response.
// Optional ?server_id= query param filters results to a single server.
func (h *PortForwardHandler) List(w http.ResponseWriter, r *http.Request) {
	items, ok := listWithServerFilter(w, r,
		func(ctx context.Context) (any, error) { return h.svc.List(ctx) },
		func(ctx context.Context, sid int64) (any, error) { return h.svc.ListByServer(ctx, sid) },
		"port forwards",
	)
	if !ok {
		return
	}
	if items == nil {
		items = []models.PortForward{}
	}
	respondJSON(w, http.StatusOK, items)
}

// Create decodes the request body and inserts a new PortForward record.
func (h *PortForwardHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	var input services.CreatePortForwardInput
	// Note: port_forwards historically used json.NewDecoder directly with a
	// generic "invalid request body" message rather than decodeOrFail (which
	// surfaces the decoder's own error). Preserve the existing UX.
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	pf, err := h.svc.Create(r.Context(), &claims.UserID, input, r.RemoteAddr)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, pf)
}

// Get writes a single PortForward record looked up by URL parameter "id".
func (h *PortForwardHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	pf, err := h.svc.GetByID(r.Context(), id)
	if err != nil || pf == nil {
		respondError(w, http.StatusNotFound, "not found")
		return
	}
	respondJSON(w, http.StatusOK, pf)
}

// Update decodes the request body and replaces an existing PortForward record.
func (h *PortForwardHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var input services.CreatePortForwardInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	pf, err := h.svc.Update(r.Context(), &claims.UserID, id, input, r.RemoteAddr)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, pf)
}

// Delete removes a PortForward record by URL parameter "id".
func (h *PortForwardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), &claims.UserID, id, r.RemoteAddr); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Enable activates a PortForward record by URL parameter "id".
func (h *PortForwardHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, true)
}

// Disable deactivates a PortForward record by URL parameter "id".
func (h *PortForwardHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, false)
}

func (h *PortForwardHandler) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	pf, err := h.svc.SetEnabled(r.Context(), &claims.UserID, id, enabled, r.RemoteAddr)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, pf)
}
