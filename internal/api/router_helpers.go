package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/posternproxy/posternproxy/internal/auth"
)

// hostRoutes is implemented by every per-host handler. The 7-route shape is
// identical across the 5 host types (proxy / redirection / dead / stream /
// port-forward), so mountHostRoutes wires all of them from one place.
type hostRoutes interface {
	List(http.ResponseWriter, *http.Request)
	Get(http.ResponseWriter, *http.Request)
	Create(http.ResponseWriter, *http.Request)
	Update(http.ResponseWriter, *http.Request)
	Delete(http.ResponseWriter, *http.Request)
	Enable(http.ResponseWriter, *http.Request)
	Disable(http.ResponseWriter, *http.Request)
}

// mountHostRoutes attaches the standard host endpoints under prefix.
// Read endpoints are open to any authenticated user; mutations require admin.
func mountHostRoutes(r chi.Router, prefix string, h hostRoutes) {
	r.Route(prefix, func(r chi.Router) {
		r.Get("/", h.List)
		r.Get("/{id}", h.Get)
		r.With(auth.RequireAdmin).Post("/", h.Create)
		r.With(auth.RequireAdmin).Put("/{id}", h.Update)
		r.With(auth.RequireAdmin).Delete("/{id}", h.Delete)
		r.With(auth.RequireAdmin).Post("/{id}/enable", h.Enable)
		r.With(auth.RequireAdmin).Post("/{id}/disable", h.Disable)
	})
}
