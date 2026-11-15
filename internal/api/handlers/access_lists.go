package handlers

import (
	"net/http"

	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

// AccessListHandler provides HTTP handler methods for the AccessList resource.
type AccessListHandler struct {
	svc *services.AccessListService
}

// NewAccessListHandler creates an AccessListHandler backed by the given service.
func NewAccessListHandler(svc *services.AccessListService) *AccessListHandler {
	return &AccessListHandler{svc: svc}
}

// List writes all AccessList records to the response.
func (h *AccessListHandler) List(w http.ResponseWriter, r *http.Request) {
	lists, err := h.svc.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list access lists")
		return
	}
	if lists == nil {
		lists = []models.AccessList{}
	}
	respondJSON(w, http.StatusOK, lists)
}

// Create decodes the request body and inserts a new AccessList record.
func (h *AccessListHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	var input services.CreateAccessListInput
	if err := decodeJSON(r, &input); err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	al, err := h.svc.Create(r.Context(), &claims.UserID, input, r.RemoteAddr)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusCreated, al)
}

// Get writes a single AccessList record looked up by URL parameter "id".
func (h *AccessListHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	al, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get access list")
		return
	}
	if al == nil {
		respondError(w, http.StatusNotFound, "access list not found")
		return
	}
	respondJSON(w, http.StatusOK, al)
}

// Update decodes the request body and replaces an existing AccessList record.
func (h *AccessListHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var input services.CreateAccessListInput
	if err := decodeJSON(r, &input); err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	al, err := h.svc.Update(r.Context(), &claims.UserID, id, input, r.RemoteAddr)
	if err != nil {
		respondServiceError(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, http.StatusOK, al)
}

// Delete removes an AccessList record by URL parameter "id".
func (h *AccessListHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
