// Command posternproxy starts the PosternProxy controller HTTP server.
//
// Two run modes:
//
//	posternproxy                       — start the server (default)
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
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Version is set at build time via -ldflags "-X main.Version=...".
// Falls back to VCS info from the Go module build stamp, or "dev".
var Version = buildinfo.Version()

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		runResetPassword(os.Args[2:])
		return
	}

	cfg := config.Load()
	cfg.Version = Version

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to build application")
	}
	defer func() { _ = a.Close() }()

	pw, err := a.EnsureAdminUser(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to ensure admin user")
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
		log.Fatal().Err(err).Msg("server error")
	case <-ctx.Done():
	}
	log.Info().Msg("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
}
