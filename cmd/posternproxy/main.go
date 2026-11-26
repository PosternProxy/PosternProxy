// Command posternproxy starts the PosternProxy controller HTTP server.
//
// Run modes:
//
//	posternproxy                       — start the server (default)
//	posternproxy install               — install the controller and dependencies
//	posternproxy reset-password [--email <addr>]
//	                                   — generate a new password for an admin
//	                                     account; the server does NOT need
//	                                     to be running.
//
// All bootstrap work (DB open, service wiring, router construction, initial
// Caddy sync) lives in internal/app. This file owns only process-level
// concerns: logger init, signal handling, and the http.Server lifecycle.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/posternproxy/posternproxy/internal/app"
	"github.com/posternproxy/posternproxy/internal/buildinfo"
	"github.com/posternproxy/posternproxy/internal/config"
	"github.com/posternproxy/posternproxy/internal/platformservice"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Version is set at build time via -ldflags "-X main.Version=...".
// Falls back to VCS info from the Go module build stamp, or "dev".
var Version = "dev"

func main() {
	if Version == "dev" {
		Version = buildinfo.Version()
	}
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if os.Getenv("POSTERNPROXY_LOG_FORMAT") != "json" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	if len(os.Args) == 2 && os.Args[1] == "version" {
		buildinfo.WriteVersion(os.Stdout, "posternproxy", Version)
		return
	}
	if handled, err := platformservice.Dispatch(os.Args[1:], "posternproxy", runController); handled {
		if err != nil {
			log.Fatal().Err(err).Msg("Windows service operation failed")
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "install" {
		runInstall(os.Args[2:])
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		runResetPassword(os.Args[2:])
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := runController(ctx); err != nil {
		log.Fatal().Err(err).Msg("controller stopped")
	}
}

func runController(ctx context.Context) error {
	cfg := config.Load()
	cfg.Version = Version
	a, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = a.Close() }()

	pw, err := a.EnsureAdminUser(ctx)
	if err != nil {
		return err
	}
	if pw != "" {
		email := os.Getenv("POSTERNPROXY_ADMIN_EMAIL")
		if email == "" {
			email = "admin@posternproxy.local"
		}
		printSetupBanner(email, pw)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           a.Router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      0, // SSE endpoints stream for minutes; per-route timeouts are set via chi middleware
		IdleTimeout:       60 * time.Second,
	}

	listenErr := make(chan error, 1)
	go func() {
		log.Info().Str("addr", cfg.ListenAddr).Str("version", Version).Msg("starting PosternProxy")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr <- err
		}
	}()

	select {
	case err := <-listenErr:
		return err
	case <-ctx.Done():
	}
	log.Info().Msg("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return nil
}
