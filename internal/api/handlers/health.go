package handlers

import (
	"net/http"

	"github.com/posternproxy/posternproxy/internal/services"
)

// HealthHandler provides HTTP handler methods for upstream health and TLS status.
type HealthHandler struct {
	health   *services.HealthService
	tlsProbe *services.TLSProbeService
}

// NewHealthHandler creates a HealthHandler backed by the given services.
func NewHealthHandler(health *services.HealthService, tlsProbe *services.TLSProbeService) *HealthHandler {
	return &HealthHandler{health: health, tlsProbe: tlsProbe}
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

// TLS returns the latest cached TLS-probe statuses for HTTPS-serving hosts.
// GET /api/health/tls
func (h *HealthHandler) TLS(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, h.tlsProbe.Statuses())
}
