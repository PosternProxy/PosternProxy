package handlers

import (
	"net/http"

	"github.com/posternproxy/posternproxy/internal/services"
)

// HealthHandler provides HTTP handler methods for upstream health status.
type HealthHandler struct {
	health *services.HealthService
}

// NewHealthHandler creates a HealthHandler backed by the given service.
func NewHealthHandler(health *services.HealthService) *HealthHandler {
	return &HealthHandler{health: health}
}

// Upstreams returns the latest cached upstream health statuses.
// GET /api/health/upstreams
func (h *HealthHandler) Upstreams(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.health.Statuses()
	if err != nil {
		http.Error(w, "caddy unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	respondJSON(w, http.StatusOK, statuses)
}
