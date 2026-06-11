package handlers

import (
	"net/http"
	"strconv"

	"github.com/posternproxy/posternproxy/internal/services"
)

// AuditLogHandler provides HTTP handler methods for the AuditLog resource.
type AuditLogHandler struct {
	svc *services.AuditService
}

// NewAuditLogHandler creates an AuditLogHandler backed by the given service.
func NewAuditLogHandler(svc *services.AuditService) *AuditLogHandler {
	return &AuditLogHandler{svc: svc}
}

// List writes all AuditLog records to the response, with optional resource_type filter and pagination.
func (h *AuditLogHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	resourceType := q.Get("resource_type")

	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}

	entries, total, err := h.svc.List(r.Context(), resourceType, nil, page, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list audit log")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data":  entries,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}
