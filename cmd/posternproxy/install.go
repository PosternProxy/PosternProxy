package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/posternproxy/posternproxy/internal/installer"
	"github.com/rs/zerolog/log"
)

func parseInstallOptions(args []string, output io.Writer) (installer.Options, error) {
	var opts installer.Options
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.StringVar(&opts.ConfigFile, "config-file", "", "Windows service settings JSON file (optional; configuration is encrypted on installation)")
	fs.SetOutput(output)
	fs.StringVar(&opts.AssetsDir, "assets-dir", "", "local Caddy/agent assets and checksums.txt (no release downloads)")
	fs.StringVar(&opts.Release, "release", "", "exact release tag for downloads (default: this controller's version)")
	fs.IntVar(&opts.Port, "port", 81, "management HTTP port for a new configuration")
	fs.BoolVar(&opts.ConfigureFirewall, "configure-firewall", false, "add UFW rules and enable UFW; requires --ssh-port")
	fs.IntVar(&opts.SSHPort, "ssh-port", 0, "actual SSH port to allow when configuring UFW")
	fs.BoolVar(&opts.SkipPackages, "skip-packages", false, "use preinstalled OS packages (for offline installation)")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() != 0 {
		return opts, errors.New("install takes flags only; see posternproxy install --help")
	}
	return opts, opts.Validate(Version)
}

func runInstall(args []string) {
	opts, err := parseInstallOptions(args, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		log.Error().Err(err).Msg("invalid installation options")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := installer.Run(ctx, opts, Version); err != nil {
		log.Error().Err(err).Str("actor", "root").Str("action", "controller_install").Str("result", "failed").Msg("installation failed")
		os.Exit(1)
	}
}
