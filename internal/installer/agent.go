package installer

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/posternproxy/posternproxy/internal/agentconfig"
	"github.com/posternproxy/posternproxy/internal/caddybuild"
	"github.com/rs/zerolog/log"
)

func InstallAgent(ctx context.Context, opts Options, config agentconfig.Config, version string) error {
	if err := config.Validate(); err != nil {
		return err
	}
	if err := opts.Validate(version); err != nil {
		return err
	}
	if runtime.GOOS != "linux" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") || os.Geteuid() != 0 {
		return fmt.Errorf("agent installation requires root on Linux amd64 or arm64")
	}
	i := installation{opts: opts, run: runCommand, agentConfig: &config}
	if err := i.preflight(ctx); err != nil {
		return err
	}
	if err := verifyControllerIdentity(ctx, config); err != nil {
		return err
	}
	stage, err := os.MkdirTemp("", "posternproxy-agent-install-*")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(stage); err != nil {
			log.Warn().Err(err).Msg("remove agent installation staging")
		}
	}()
	if err := stageArtifacts(ctx, opts, version, runtime.GOARCH, stage); err != nil {
		return err
	}
	if err := caddybuild.VerifyArtifact(filepath.Join(stage, "caddy")); err != nil {
		return err
	}
	if _, err := i.run(ctx, "bash", "-c", caddybuild.Script+"\nverify_caddy \"$1\"", "verify-caddy", filepath.Join(stage, "caddy")); err != nil {
		return fmt.Errorf("verify Caddy build: %w", err)
	}
	if err := i.apply(ctx, stage); err != nil {
		return err
	}
	if err := i.waitReady(ctx); err != nil {
		return err
	}
	log.Info().Str("actor", "root").Str("action", "agent_install").Int64("server_id", config.ServerID).Msg("agent services installed; controller must confirm authenticated and applied readiness")
	return nil
}

func verifyControllerIdentity(ctx context.Context, config agentconfig.Config) error {
	u, err := url.Parse(config.ControllerURL)
	if err != nil {
		return fmt.Errorf("invalid controller URL")
	}
	if u.Scheme == "wss" {
		u.Scheme = "https"
	} else if u.Scheme == "ws" {
		u.Scheme = "http"
	}
	u.Path = "/agent/verify"
	q := url.Values{}
	q.Set("server_id", strconv.FormatInt(config.ServerID, 10))
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create controller identity check: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.Token)
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("controller identity check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("controller did not accept this server identity (HTTP %d); upgrade the controller first and check the URL, server ID and agent token", resp.StatusCode)
	}
	return nil
}
