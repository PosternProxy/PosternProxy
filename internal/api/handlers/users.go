package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

// UserHandler provides HTTP handler methods for the User resource.
type UserHandler struct {
	users *services.UserService
}

// NewUserHandler creates a UserHandler backed by the given service.
func NewUserHandler(users *services.UserService) *UserHandler {
	return &UserHandler{users: users}
}

// List writes all User records to the response.
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	respondJSON(w, http.StatusOK, users)
}

// Get writes a single User record looked up by URL parameter "id".
func (h *UserHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	user, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	if user == nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}
	respondJSON(w, http.StatusOK, user)
}

// Create decodes the request body and inserts a new User record.
func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input models.CreateUserInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	callerID := &claims.UserID
	user, err := h.users.Create(r.Context(), callerID, input, r.RemoteAddr)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, user)
}

// Update decodes the request body and replaces an existing User record.
func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var input models.UpdateUserInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	callerID := &claims.UserID
	user, err := h.users.Update(r.Context(), callerID, id, input, r.RemoteAddr)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// Delete removes a User record by URL parameter "id".
func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	if claims.UserID == id {
		respondError(w, http.StatusBadRequest, "cannot delete yourself")
		return
	}

	callerID := &claims.UserID
	if err := h.users.Delete(r.Context(), callerID, id, r.RemoteAddr); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ChangePassword decodes the request body and updates the password for a User record by URL parameter "id".
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var input models.ChangePasswordInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	claims := requireClaims(w, r)
	if claims == nil {
		return
	}
	callerID := &claims.UserID
	if err := h.users.ChangePassword(r.Context(), callerID, id, input.Password, r.RemoteAddr); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "password changed"})
}
