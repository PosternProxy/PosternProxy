package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/posternproxy/posternproxy/internal/agentconfig"
	"github.com/posternproxy/posternproxy/internal/installer"
	"github.com/rs/zerolog/log"
)

func runInstall(args []string) {
	opts := installer.Options{Port: 81}
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	configPath := fs.String("config-file", "", "protected JSON file containing controller_url, agent_token and server_id")
	fs.StringVar(&opts.AssetsDir, "assets-dir", "", "local agent/Caddy assets and checksums.txt")
	fs.StringVar(&opts.Release, "release", "", "exact release tag for asset downloads")
	fs.BoolVar(&opts.SkipPackages, "skip-packages", false, "use already installed OS packages")
	fs.BoolVar(&opts.ConfigureFirewall, "configure-firewall", false, "add UFW rules without resetting existing rules")
	fs.IntVar(&opts.SSHPort, "ssh-port", 0, "actual SSH port, required with --configure-firewall")
	err := fs.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err == nil && fs.NArg() != 0 {
		err = fmt.Errorf("install accepts flags only")
	}
	var cfg agentconfig.Config
	if err == nil {
		cfg, err = readInstallConfig(*configPath)
	}
	if err != nil {
		log.Error().Err(err).Msg("invalid agent installation configuration")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := installer.InstallAgent(ctx, opts, cfg, Version); err != nil {
		log.Error().Err(err).Msg("agent installation failed")
		os.Exit(1)
	}
}

func readInstallConfig(path string) (agentconfig.Config, error) {
	var config agentconfig.Config
	if path == "" {
		return config, fmt.Errorf("--config-file is required; do not pass agent credentials as command arguments")
	}
	f, err := os.Open(path) // #nosec G304 -- path is the agent's own protected config file, not request input
	if err != nil {
		return config, fmt.Errorf("open protected agent configuration: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return config, err
	}
	if !info.Mode().IsRegular() || info.Size() > 16384 || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return config, fmt.Errorf("agent configuration must be a private regular file (mode 0600, at most 16 KiB)")
	}
	decoder := json.NewDecoder(io.LimitReader(f, 16385))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, fmt.Errorf("invalid agent configuration JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return config, fmt.Errorf("agent configuration must contain one JSON object")
	}
	return config, config.Validate()
}
