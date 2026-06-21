// Package app wires PosternProxy's services into a runnable HTTP handler.
//
// The cmd/posternproxy/main.go binary owns process-level concerns (logger
// init, signal handling, the http.Server lifecycle, CLI subcommands); it
// delegates everything else — DB open + migrations, service construction,
// hub wiring, config-history rollback registration, initial Caddy sync,
// port-forward restoration, and router construction — to App.New().
package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/api"
	"github.com/posternproxy/posternproxy/internal/caddy"
	"github.com/posternproxy/posternproxy/internal/config"
	"github.com/posternproxy/posternproxy/internal/database"
	"github.com/posternproxy/posternproxy/internal/models"
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

	// healthSvc is held so Close() can stop its background poller.
	healthSvc *services.HealthService
}

// New constructs an App from cfg. It:
//   - opens the database and runs migrations,
//   - constructs every service,
//   - wires the agent hub callbacks,
//   - registers config-history rollback handlers,
//   - performs the initial Caddy full-sync (non-fatal on failure),
//   - restores port-forwarding rules,
//   - starts background health polling,
//   - builds the HTTP router.
//
// It does NOT start the HTTP server — the caller does that. Errors are
// returned rather than fatal-ed so main.go is the only place that calls
// log.Fatal.
//
// The ctx is currently unused at the bootstrap layer but is accepted as the
// first parameter to allow future migration to context-aware service init
// (e.g. when Task 9 lands).
func New(ctx context.Context, cfg *config.Config) (*App, error) {
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
	tokenSvc := services.NewAPITokenService(db)
	joinTokenSvc := services.NewJoinTokenService(db)
	userSvc := services.NewUserService(db, auditSvc)
	accessListSvc := services.NewAccessListService(db, auditSvc)
	certSvc := services.NewCertificateService(db, cfg.CertDir, auditSvc)
	portForwardSvc := services.NewPortForwardService(db, auditSvc)
	healthSvc := services.NewHealthService(caddyClient)
	tlsProbeSvc := services.NewTLSProbeService(db, "127.0.0.1:443")
	historySvc := services.NewConfigHistoryService(db)

	// Agent hub — constructed before CaddySyncService so the onConnect
	// callback can close over caddySyncSvc once it is assigned below.
	var caddySyncSvc *services.CaddySyncService
	var serverSvc *services.ServerService

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
				if err := caddySyncSvc.SyncServer(context.Background(), serverID); err != nil {
					log.Error().Err(err).Int64("server_id", serverID).Msg("full sync on reconnect failed")
				}
			}
		},
	)

	caddySyncSvc = services.NewCaddySyncService(db, caddyClient, hub, cfg.Version)

	// Host services route remote-server mutations through the hub via
	// caddySyncSvc rather than the local Caddy admin API.
	proxyHostSvc := services.NewProxyHostService(db, caddyClient, auditSvc, historySvc, caddySyncSvc)
	redirectionHostSvc := services.NewRedirectionHostService(db, caddyClient, auditSvc, caddySyncSvc)
	deadHostSvc := services.NewDeadHostService(db, caddyClient, auditSvc, caddySyncSvc)
	streamHostSvc := services.NewStreamHostService(db, caddyClient, auditSvc, caddySyncSvc)

	backupSvc := services.NewBackupService(db, caddySyncSvc)
	serverSvc = services.NewServerService(db, auditSvc, hub, tokenSvc, caddyClient, cfg.EncryptionKey, cfg.ControllerURL, cfg.AgentBinPath)
	nginxImportSvc := services.NewNginxImportService(proxyHostSvc, redirectionHostSvc)

	registerRollbackHandlers(historySvc, proxyHostSvc)

	// ── Initial Caddy sync ────────────────────────────────────────────────
	// Caddy may still be starting when the controller boots (systemd orders us
	// after caddy.service but does not wait for its admin API). A single attempt
	// that loses that race used to leave Caddy running its empty default config
	// with no retry, so nothing was ever proxied. Retry in the background until
	// it succeeds.
	log.Info().Str("caddy_api", cfg.CaddyAPI).Msg("syncing Caddy configuration")
	go syncCaddyWithRetry(ctx, caddySyncSvc)

	// ── Port forwards lost on reboot — restore them ──────────────────────
	log.Info().Msg("restoring port forwarding rules")
	portForwardSvc.RestoreAll(ctx)

	// ── Background upstream health polling ────────────────────────────────
	healthSvc.Start(30 * time.Second)

	// ── Background TLS certificate probing ────────────────────────────────
	tlsProbeSvc.Start(60 * time.Second)

	// ── Background purge of expired join tokens ───────────────────────────
	go func() {
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
		PortForwardService:     portForwardSvc,
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
	})

	return &App{
		Cfg:       cfg,
		DB:        db,
		Router:    router,
		Users:     userSvc,
		healthSvc: healthSvc,
	}, nil
}

// Close releases resources held by the App. Idempotent.
func (a *App) Close() error {
	if a == nil {
		return nil
	}
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}

// syncCaddyWithRetry pushes the full config to Caddy, retrying with a capped
// backoff until it succeeds or ctx is cancelled. The first attempt is immediate
// so a healthy boot syncs without delay.
func syncCaddyWithRetry(ctx context.Context, sync *services.CaddySyncService) {
	const maxDelay = 30 * time.Second
	delay := 1 * time.Second
	for {
		if err := sync.FullSync(ctx); err != nil {
			log.Warn().Err(err).Dur("retry_in", delay).Msg("Caddy sync failed (Caddy may not be ready yet); will retry")
		} else {
			log.Info().Msg("Caddy configuration synced")
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < maxDelay {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}
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
			HTTPOnly:   h.HTTPOnly,
			ErrorPages: h.ErrorPages, CustomHeaders: h.CustomHeaders,
			ExtraUpstreams: h.ExtraUpstreams, LBPolicy: h.LBPolicy,
		}
		_, err := proxyHostSvc.Update(ctx, callerID, id, input, ip)
		return err
	})
}
