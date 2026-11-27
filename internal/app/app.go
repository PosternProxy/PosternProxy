// Package app wires PosternProxy's services into a runnable HTTP handler.
//
// The cmd/posternproxy/main.go binary owns process-level concerns (logger
// init, signal handling, the http.Server lifecycle, CLI subcommands); it
// delegates everything else — DB open + migrations, service construction,
// hub wiring, config-history rollback registration, initial Caddy sync,
// reconciliation, and router construction — to App.New().
package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/api"
	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/certstore"
	"github.com/posternproxy/posternproxy/internal/config"
	"github.com/posternproxy/posternproxy/internal/database"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/security"
	"github.com/posternproxy/posternproxy/internal/services"
	"github.com/posternproxy/posternproxy/internal/ui"
	"github.com/rs/zerolog/log"
)

// App holds the long-lived state for one PosternProxy process. main.go
// constructs an App, mounts App.Router on an http.Server, and calls App.Close
// on shutdown.
type App struct {
	Cfg    *config.Config
	DB     *sql.DB
	Router http.Handler

	// users is exposed so the CLI ensure-admin and reset-password flows can
	// reuse the same service the HTTP layer uses.
	Users *services.UserService

	healthSvc *services.HealthService
	syncSvc   *services.CaddySyncService
	cancel    context.CancelFunc
}

// New constructs an App from cfg. It:
//   - opens the database and runs migrations,
//   - constructs every service,
//   - wires the agent hub callbacks,
//   - registers config-history rollback handlers,
//   - performs the initial Caddy full-sync (non-fatal on failure),
//   - starts background health polling,
//   - builds the HTTP router.
//
// It does NOT start the HTTP server — the caller does that. Errors are
// returned rather than fatal-ed so main.go is the only place that calls
// log.Fatal.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	if err := config.ValidateManagementAddress(cfg.ListenAddr); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	ready := false
	defer func() {
		if !ready {
			cancel()
		}
	}()
	trustedProxies, err := security.ParseTrustedProxies(cfg.TrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("configure client identity: %w", err)
	}
	caddyProxies, err := caddy.ParseTrustedProxyPolicies(cfg.CaddyTrustedProxies)
	if err != nil {
		return nil, fmt.Errorf("configure Caddy client identity: %w", err)
	}
	log.Info().Str("db", cfg.DBPath).Msg("opening database")
	db, err := database.Open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := os.MkdirAll(cfg.CertDir, 0700); err != nil {
		db.Close()
		return nil, fmt.Errorf("create cert dir: %w", err)
	}

	caddyClient := caddy.NewClient(cfg.CaddyAPI)

	// ── Services ──────────────────────────────────────────────────────────
	auditSvc := services.NewAuditService(db)
	settingsSvc := services.NewSettingsService(db)
	settingsSvc.ConfigureRuntime(cfg.CaddyAPI, cfg.CertDir)
	tokenSvc := services.NewAPITokenService(db)
	joinTokenSvc := services.NewJoinTokenService(db)
	userSvc := services.NewUserService(db, auditSvc)
	certSvc := services.NewCertificateService(db, cfg.CertDir, auditSvc, cfg.EncryptionKey)
	if err := certSvc.ProtectExisting(ctx); err != nil {
		log.Error().Err(err).Msg("certificate storage migration incomplete; repair missing files or replace affected certificates")
	}
	healthSvc := services.NewHealthService(caddyClient)
	tlsProbeSvc := services.NewTLSProbeService(db, "127.0.0.1:443")
	historySvc := services.NewConfigHistoryService(db, cfg.EncryptionKey)
	if err := historySvc.ProtectExisting(ctx); err != nil {
		db.Close()
		return nil, err
	}

	// Agent hub — constructed before CaddySyncService so the onConnect
	// callback can close over caddySyncSvc once it is assigned below.
	var caddySyncSvc *services.CaddySyncService
	var serverSvc *services.ServerService
	var accessListSvc *services.AccessListService

	hub := agent.NewHub(
		// onStatus: called on every heartbeat from any agent.
		func(serverID int64, status agent.StatusPayload) {
			log.Info().Int64("server_id", serverID).Int64("uptime", status.Uptime).Msg("agent heartbeat")
			if serverSvc != nil {
				// Background work — request context would cancel the heartbeat write if the user navigated away.
				serverSvc.UpdateFromStatus(context.Background(), serverID, status)
			}
		},
		// onConnect: called when an agent reconnects after missed config changes.
		func(serverID int64) {
			if caddySyncSvc != nil {
				// Background work — request context would cancel sync if the user navigated away.
				if err := caddySyncSvc.Reconnect(ctx, serverID); err != nil {
					log.Error().Err(err).Int64("server_id", serverID).Msg("full sync on reconnect failed")
				}
			}
		},
	)

	caddySyncSvc = services.NewCaddySyncService(db, caddyClient, hub, cfg.Version)
	caddySyncSvc.ConfigureCertificates(certstore.Store{Root: cfg.CertDir, Key: cfg.EncryptionKey}, cfg.CertRuntimeDir)
	caddySyncSvc.ConfigureTrustedProxies(caddyProxies)
	accessListSvc = services.NewAccessListService(db, auditSvc, caddySyncSvc)

	// Host services route remote-server mutations through the hub via
	// caddySyncSvc rather than the local Caddy admin API.
	proxyHostSvc := services.NewProxyHostService(db, caddyClient, auditSvc, historySvc, caddySyncSvc)
	redirectionHostSvc := services.NewRedirectionHostService(db, caddyClient, auditSvc, caddySyncSvc)
	deadHostSvc := services.NewDeadHostService(db, caddyClient, auditSvc, caddySyncSvc)
	streamHostSvc := services.NewStreamHostService(db, caddyClient, auditSvc, caddySyncSvc)
	if _, port, err := net.SplitHostPort(cfg.ListenAddr); err == nil {
		if port, err := strconv.Atoi(port); err == nil {
			streamHostSvc.ConfigureManagementPort(port)
		}
	}

	backupSvc := services.NewBackupService(db, caddySyncSvc)
	backupSvc.ConfigureCertificates(certstore.Store{Root: cfg.CertDir, Key: cfg.EncryptionKey})
	serverSvc = services.NewServerService(db, auditSvc, hub, tokenSvc, caddyClient, cfg.EncryptionKey, cfg.ControllerURL, cfg.AgentBinPath)
	serverSvc.ConfigureArtifacts(cfg.CaddyBinPath, cfg.Version)
	serverSvc.ConfigureArtifactCache(filepath.Join(cfg.DataDir, "artifacts"))
	nginxImportSvc := services.NewNginxImportService(proxyHostSvc, redirectionHostSvc)

	registerRollbackHandlers(historySvc, proxyHostSvc)

	// ── Initial Caddy sync ────────────────────────────────────────────────
	// Caddy may still be starting when the controller boots (systemd orders us
	// after caddy.service but does not wait for its admin API). A single attempt
	// that loses that race used to leave Caddy running its empty default config
	// with no retry, so nothing was ever proxied. Retry in the background until
	// it succeeds.
	log.Info().Str("caddy_api", cfg.CaddyAPI).Msg("syncing Caddy configuration")
	if err := caddySyncSvc.Start(ctx); err != nil {
		db.Close()
		return nil, err
	}

	// ── Background upstream health polling ────────────────────────────────
	healthSvc.Start(30 * time.Second)

	// ── Background TLS certificate probing ────────────────────────────────
	tlsProbeSvc.Start(60 * time.Second)

	// ── Background purge of expired join tokens ───────────────────────────
	go func() { // #nosec G118 -- intentional background task; a request-scoped context would cancel the hourly purge
		t := time.NewTicker(1 * time.Hour)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := joinTokenSvc.PurgeExpired(context.Background()); err != nil {
					log.Warn().Err(err).Msg("join token purge failed")
				}
			}
		}
	}()

	router := api.NewRouter(api.RouterConfig{
		TrustedProxies:         trustedProxies,
		JWTSecret:              cfg.JWTSecret,
		AllowedOrigins:         cfg.AllowedOrigins,
		DB:                     db,
		Hub:                    hub,
		UserService:            userSvc,
		AuditService:           auditSvc,
		ProxyHostService:       proxyHostSvc,
		RedirectionHostService: redirectionHostSvc,
		DeadHostService:        deadHostSvc,
		StreamHostService:      streamHostSvc,
		AccessListService:      accessListSvc,
		CertificateService:     certSvc,
		ServerService:          serverSvc,
		SettingsService:        settingsSvc,
		APITokenService:        tokenSvc,
		JoinTokenService:       joinTokenSvc,
		HealthService:          healthSvc,
		TLSProbeService:        tlsProbeSvc,
		BackupService:          backupSvc,
		ConfigHistoryService:   historySvc,
		NginxImportService:     nginxImportSvc,
		CaddySyncService:       caddySyncSvc,
		FrontendFS:             ui.FS(),
		Version:                cfg.Version,
		SecureCookies:          cfg.BehindTLSProxy,
		LoginRateLimit:         cfg.LoginRateLimit,
	})

	ready = true
	return &App{
		cancel:    cancel,
		Cfg:       cfg,
		DB:        db,
		Router:    router,
		Users:     userSvc,
		healthSvc: healthSvc,
		syncSvc:   caddySyncSvc,
	}, nil
}

// Close releases resources held by the App. Idempotent.
func (a *App) Close() error {
	if a == nil {
		return nil
	}
	if a.cancel != nil {
		a.cancel()
	}
	if a.DB != nil {
		if a.syncSvc != nil {
			a.syncSvc.Close()
		}
		return a.DB.Close()
	}
	return nil
}

// registerRollbackHandlers wires every resource-rollback handler the
// config-history service needs. Adding a new rollback-capable resource means
// adding a case here.
func registerRollbackHandlers(historySvc *services.ConfigHistoryService, proxyHostSvc *services.ProxyHostService) {
	historySvc.Register("proxy_host", func(ctx context.Context, callerID *int64, id int64, snapshot []byte, ip string) error {
		var h models.ProxyHost
		if err := json.Unmarshal(snapshot, &h); err != nil {
			return fmt.Errorf("unmarshal proxy_host snapshot: %w", err)
		}
		input := models.CreateProxyHostInput{
			ServerID:    h.ServerID,
			DomainNames: h.DomainNames, ForwardScheme: h.ForwardScheme,
			ForwardHost: h.ForwardHost, ForwardPort: h.ForwardPort,
			CertificateID: h.CertificateID, SSLForced: h.SSLForced,
			HSTSEnabled: h.HSTSEnabled, HSTSSubdomains: h.HSTSSubdomains,
			AccessListID:   h.AccessListID,
			AdvancedConfig: h.AdvancedConfig, Locations: h.Locations,
			HCEnabled: h.HCEnabled, HCActivePath: h.HCActivePath,
			HCActiveInterval: h.HCActiveInterval, HCActiveTimeout: h.HCActiveTimeout,
			HCActiveExpectStatus:  h.HCActiveExpectStatus,
			HCPassiveFailDuration: h.HCPassiveFailDuration, HCPassiveMaxFails: h.HCPassiveMaxFails,
			HCPassiveUnhealthyLatency: h.HCPassiveUnhealthyLatency,
			RateLimitEnabled:          h.RateLimitEnabled, RateLimitRequests: h.RateLimitRequests,
			RateLimitWindow: h.RateLimitWindow, TLSPassthrough: h.TLSPassthrough,
			HTTPOnly: h.HTTPOnly, NTLMEnabled: h.NTLMEnabled,
			ErrorPages: h.ErrorPages, CustomHeaders: h.CustomHeaders,
			ExtraUpstreams: h.ExtraUpstreams, LBPolicy: h.LBPolicy,
		}
		_, err := proxyHostSvc.Update(ctx, callerID, id, input, ip)
		return err
	})
}
