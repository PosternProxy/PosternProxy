package api

import (
	"database/sql"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/api/handlers"
	"github.com/posternproxy/posternproxy/internal/auth"
	"github.com/posternproxy/posternproxy/internal/security"
	"github.com/posternproxy/posternproxy/internal/services"
)

// RouterConfig holds all service dependencies required to construct the
// application router.
type RouterConfig struct {
	JWTSecret              string
	AllowedOrigins         []string
	DB                     *sql.DB
	Hub                    *agent.Hub
	UserService            *services.UserService
	AuditService           *services.AuditService
	ProxyHostService       *services.ProxyHostService
	RedirectionHostService *services.RedirectionHostService
	DeadHostService        *services.DeadHostService
	StreamHostService      *services.StreamHostService
	AccessListService      *services.AccessListService
	CertificateService     *services.CertificateService
	PortForwardService     *services.PortForwardService
	ServerService          *services.ServerService
	SettingsService        *services.SettingsService
	APITokenService        *services.APITokenService
	JoinTokenService       *services.JoinTokenService
	HealthService          *services.HealthService
	TLSProbeService        *services.TLSProbeService
	BackupService          *services.BackupService
	ConfigHistoryService   *services.ConfigHistoryService
	NginxImportService     *services.NginxImportService
	CaddySyncService       *services.CaddySyncService
	FrontendFS             fs.FS
	Version                string
	SecureCookies          bool // Set Secure flag on cookies (true when behind TLS proxy)
}

// NewRouter wires all handlers into a chi router, applies middleware, and mounts
// the embedded SPA at the catch-all route.
func NewRouter(cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(security.SecureHeaders)
	allowedOrigins := cfg.AllowedOrigins
	if len(allowedOrigins) == 0 {
		// In production the SPA is served same-origin, so no CORS headers are
		// needed. An empty AllowedOrigins list rejects all cross-origin
		// requests. Set POSTERNPROXY_ALLOWED_ORIGINS for dev (e.g. Vite proxy).
		allowedOrigins = []string{}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	authHandler := handlers.NewAuthHandler(cfg.UserService, cfg.AuditService, cfg.JWTSecret, cfg.SecureCookies)
	userHandler := handlers.NewUserHandler(cfg.UserService)
	proxyHostHandler := handlers.NewProxyHostHandler(cfg.ProxyHostService)
	redirectionHostHandler := handlers.NewRedirectionHostHandler(cfg.RedirectionHostService)
	deadHostHandler := handlers.NewDeadHostHandler(cfg.DeadHostService)
	streamHostHandler := handlers.NewStreamHostHandler(cfg.StreamHostService)
	accessListHandler := handlers.NewAccessListHandler(cfg.AccessListService)
	certHandler := handlers.NewCertificateHandler(cfg.CertificateService)
	auditHandler := handlers.NewAuditLogHandler(cfg.AuditService)
	dashboardHandler := handlers.NewDashboardHandler(cfg.DB)
	portForwardHandler := handlers.NewPortForwardHandler(cfg.PortForwardService)
	serverHandler := handlers.NewServerHandler(cfg.ServerService, cfg.Hub, cfg.CaddySyncService)
	settingsHandler := handlers.NewSettingsHandler(cfg.SettingsService)
	apiTokenHandler := handlers.NewAPITokenHandler(cfg.APITokenService)
	joinTokenHandler := handlers.NewJoinTokenHandler(cfg.JoinTokenService, cfg.ServerService, cfg.SettingsService, cfg.AuditService)
	healthHandler := handlers.NewHealthHandler(cfg.HealthService, cfg.TLSProbeService)
	backupHandler := handlers.NewBackupHandler(cfg.BackupService)
	configHistoryHandler := handlers.NewConfigHistoryHandler(cfg.ConfigHistoryService)
	nginxImportHandler := handlers.NewNginxImportHandler(cfg.NginxImportService)

	loginLimiter := security.NewRateLimiter(5, time.Minute)

	// agentLimiter throttles authentication attempts on the agent WebSocket
	// endpoint to prevent brute-forcing the bcrypt-hashed agent tokens. Real
	// agents reconnect at most a few times per minute on transient failure, so
	// 30/min/IP leaves plenty of headroom while making brute force impractical.
	agentLimiter := security.NewRateLimiter(30, time.Minute)

	// apiLimiter applies to all authenticated management API routes.
	// 120 requests/min per IP is generous for interactive use and UI polling
	// while limiting damage from leaked tokens or runaway automation.
	apiLimiter := security.NewRateLimiter(120, time.Minute)

	r.Route("/api", func(r chi.Router) {
		r.Use(security.SeedCSRFCookie(cfg.SecureCookies))

		// Lightweight version endpoint — no auth required.
		r.Get("/version", func(w http.ResponseWriter, _ *http.Request) {
			handlers.RespondJSON(w, http.StatusOK, map[string]string{
				"version": cfg.Version,
			})
		})

		// Auth endpoints are exempt from CSRF header validation — login/refresh
		// require credentials, making them inherently CSRF-safe; logout is
		// protected by the JWT bearer token.
		r.Route("/auth", func(r chi.Router) {
			r.With(loginLimiter.Middleware).Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.With(auth.Middleware(cfg.JWTSecret, nil)).Post("/logout", authHandler.Logout)
			r.With(auth.Middleware(cfg.JWTSecret, nil)).Get("/me", authHandler.Me)
		})

		// All other API routes require CSRF + authentication.
		r.Group(func(r chi.Router) {
			r.Use(security.CSRFProtection(cfg.SecureCookies))
			r.Use(auth.Middleware(cfg.JWTSecret, cfg.APITokenService))
			r.Use(apiLimiter.Middleware)

			r.Route("/users", func(r chi.Router) {
				r.Use(auth.RequireAdmin)
				r.Get("/", userHandler.List)
				r.Post("/", userHandler.Create)
				r.Get("/{id}", userHandler.Get)
				r.Put("/{id}", userHandler.Update)
				r.Delete("/{id}", userHandler.Delete)
				r.Put("/{id}/password", userHandler.ChangePassword)
			})

			// Standard 7-endpoint host routes (read/get/create/update/delete +
			// enable/disable) are wired by mountHostRoutes; see router_helpers.go.
			mountHostRoutes(r, "/proxy-hosts", proxyHostHandler)
			mountHostRoutes(r, "/redirection-hosts", redirectionHostHandler)
			mountHostRoutes(r, "/dead-hosts", deadHostHandler)
			mountHostRoutes(r, "/stream-hosts", streamHostHandler)

			r.Route("/access-lists", func(r chi.Router) {
				r.Get("/", accessListHandler.List)
				r.Get("/{id}", accessListHandler.Get)
				r.With(auth.RequireAdmin).Post("/", accessListHandler.Create)
				r.With(auth.RequireAdmin).Put("/{id}", accessListHandler.Update)
				r.With(auth.RequireAdmin).Delete("/{id}", accessListHandler.Delete)
			})

			r.Route("/certificates", func(r chi.Router) {
				r.Get("/", certHandler.List)
				r.Get("/{id}", certHandler.Get)
				r.With(auth.RequireAdmin).Post("/upload", certHandler.Upload)
				r.With(auth.RequireAdmin).Delete("/{id}", certHandler.Delete)
			})

			mountHostRoutes(r, "/port-forwards", portForwardHandler)

			r.Route("/servers", func(r chi.Router) {
				r.Use(auth.RequireAdmin)
				r.Get("/", serverHandler.List)
				r.Post("/", serverHandler.Create)
				r.Get("/{id}", serverHandler.Get)
				r.Put("/{id}", serverHandler.Update)
				r.Delete("/{id}", serverHandler.Delete)
				r.Get("/{id}/status", serverHandler.Status)
				r.Post("/{id}/resync", serverHandler.Resync)
				r.Put("/{id}/http2", serverHandler.SetHTTP2)
				r.Post("/{id}/provision", serverHandler.Reprovision)
				r.Get("/{id}/provision-log", serverHandler.ProvisionLog)
				r.Post("/{id}/join-token", joinTokenHandler.Issue)
				r.Delete("/{id}/join-token", joinTokenHandler.Revoke)
				r.Post("/{id}/reset-join", serverHandler.ResetJoin)
			})

			// API tokens — any authenticated user can manage their own tokens;
			// admins can see and revoke all tokens.
			r.Route("/tokens", func(r chi.Router) {
				r.Get("/", apiTokenHandler.List)
				r.Post("/", apiTokenHandler.Create)
				r.Delete("/{id}", apiTokenHandler.Revoke)
			})

			r.Get("/audit-log", auditHandler.List)

			r.Route("/health", func(r chi.Router) {
				r.Get("/upstreams", healthHandler.Upstreams)
				r.Get("/tls", healthHandler.TLS)
			})

			r.Route("/dashboard", func(r chi.Router) {
				r.Get("/stats", dashboardHandler.Stats)
			})

			r.Route("/settings", func(r chi.Router) {
				r.Use(auth.RequireAdmin)
				r.Get("/", settingsHandler.List)
				r.Put("/", settingsHandler.Update)
			})

			r.Route("/backup", func(r chi.Router) {
				r.Use(auth.RequireAdmin)
				r.Get("/export", backupHandler.Export)
				r.Post("/import", backupHandler.Import)
			})

			r.Route("/config-history", func(r chi.Router) {
				r.Get("/", configHistoryHandler.List)
				r.With(auth.RequireAdmin).Post("/{id}/rollback", configHistoryHandler.Rollback)
			})

			r.Route("/import", func(r chi.Router) {
				r.With(auth.RequireAdmin).Post("/nginx/preview", nginxImportHandler.Preview)
				r.With(auth.RequireAdmin).Post("/nginx/confirm", nginxImportHandler.Confirm)
			})
		})
	})

	// WebSocket endpoint for agent connections — no CSRF/JWT, uses token auth.
	// Rate-limited per source IP to prevent brute-forcing the bcrypt token.
	r.With(agentLimiter.Middleware).Get("/ws/agent", serverHandler.AgentConnect)

	// Serve the embedded SPA for all non-API routes.
	if cfg.FrontendFS != nil {
		r.Get("/*", SPAHandler(cfg.FrontendFS))
	}

	return r
}
