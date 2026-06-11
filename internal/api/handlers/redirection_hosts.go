package handlers

import (
	"context"
	"net/http"

	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

// RedirectionHostHandler provides HTTP handler methods for the RedirectionHost resource.
type RedirectionHostHandler struct {
	svc *services.RedirectionHostService
}

// NewRedirectionHostHandler creates a RedirectionHostHandler backed by the given service.
func NewRedirectionHostHandler(svc *services.RedirectionHostService) *RedirectionHostHandler {
	return &RedirectionHostHandler{svc: svc}
}

// List writes all RedirectionHost records to the response.
// Optional ?server_id= query param filters results to a single server.
func (h *RedirectionHostHandler) List(w http.ResponseWriter, r *http.Request) {
	items, ok := listWithServerFilter(w, r,
		func(ctx context.Context) (any, error) { return h.svc.List(ctx) },
		func(ctx context.Context, sid int64) (any, error) { return h.svc.ListByServer(ctx, sid) },
		"redirection hosts",
	)
	if !ok {
		return
	}
	if items == nil {
		items = []models.RedirectionHost{}
	}
	respondJSON(w, http.StatusOK, items)
}

// Create decodes the request body and inserts a new RedirectionHost record.
func (h *RedirectionHostHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	var input services.CreateRedirectionHostInput
	if !decodeOrFail(w, r, &input) {
		return
	}
	host, err := h.svc.Create(r.Context(), &claims.UserID, input, r.RemoteAddr)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, host)
}

// Get writes a single RedirectionHost record looked up by URL parameter "id".
func (h *RedirectionHostHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	host, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get redirection host")
		return
	}
	if host == nil {
		respondError(w, http.StatusNotFound, "redirection host not found")
		return
	}
	respondJSON(w, http.StatusOK, host)
}

// Update decodes the request body and replaces an existing RedirectionHost record.
func (h *RedirectionHostHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	var input services.CreateRedirectionHostInput
	if !decodeOrFail(w, r, &input) {
		return
	}
	host, err := h.svc.Update(r.Context(), &claims.UserID, id, input, r.RemoteAddr)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, host)
}

// Delete removes a RedirectionHost record by URL parameter "id".
func (h *RedirectionHostHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

// Enable activates a RedirectionHost record by URL parameter "id".
func (h *RedirectionHostHandler) Enable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, true)
}

// Disable deactivates a RedirectionHost record by URL parameter "id".
func (h *RedirectionHostHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.setEnabled(w, r, false)
}

func (h *RedirectionHostHandler) setEnabled(w http.ResponseWriter, r *http.Request, enabled bool) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, ok := requireID(w, r)
	if !ok {
		return
	}
	if err := h.svc.SetEnabled(r.Context(), &claims.UserID, id, enabled, r.RemoteAddr); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	host, _ := h.svc.GetByID(r.Context(), id)
	respondJSON(w, http.StatusOK, host)
}
