package provisioner

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/posternproxy/posternproxy/internal/agentconfig"
	"github.com/posternproxy/posternproxy/internal/caddybuild"
	"github.com/posternproxy/posternproxy/internal/installer"
)

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func installSystemdAgent(input ProvisionInput, c *sshClient, log func(string)) error {
	output, err := c.output("uname -sm")
	if err != nil {
		return fmt.Errorf("inspect remote architecture: %w", err)
	}
	arch, err := remoteArchitecture(string(output))
	if err != nil {
		return err
	}
	agentPath, agentErr := selectArtifact(input.AgentBinPath, "posternproxy-agent", arch, false)
	caddyConfigured := input.CaddyBinPath
	if caddyConfigured == "" {
		caddyConfigured = filepath.Join(filepath.Dir(agentPath), "caddy")
	}
	caddyPath, caddyErr := selectArtifact(caddyConfigured, "caddy", arch, true)
	if agentErr != nil || caddyErr != nil {
		if input.ArtifactCacheDir == "" {
			return fmt.Errorf("matching Linux provisioning assets are unavailable")
		}
		if err := os.MkdirAll(input.ArtifactCacheDir, 0700); err != nil {
			return err
		}
		stage, err := os.MkdirTemp(input.ArtifactCacheDir, "provision-assets-*")
		if err != nil {
			return err
		}
		defer func() {
			if err := os.RemoveAll(stage); err != nil {
				log("Provisioning asset staging cleanup failed.")
			}
		}()
		downloadCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		log("Fetching matching verified Linux provisioning assets for this controller release.")
		if err := installer.StageLinuxAgentAssets(downloadCtx, stage, input.Version, arch); err != nil {
			return err
		}
		agentPath, err = selectArtifact(filepath.Join(stage, "posternproxy-agent"), "posternproxy-agent", arch, false)
		if err != nil {
			return err
		}
		caddyPath, err = selectArtifact(filepath.Join(stage, "caddy"), "caddy", arch, true)
		if err != nil {
			return err
		}
	}
	output, err = c.output("id -u")
	if err != nil {
		return err
	}
	prefix := ""
	if strings.TrimSpace(string(output)) != "0" {
		output, err = c.output("sudo -n id -u")
		if err != nil || strings.TrimSpace(string(output)) != "0" {
			return fmt.Errorf("remote SSH account requires root or passwordless sudo")
		}
		prefix = "sudo -n "
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	dir := "/tmp/posternproxy-provision-" + hex.EncodeToString(nonce[:])
	if err := c.run("mkdir -m 0700 -- "+shellQuote(dir), io.Discard); err != nil {
		return fmt.Errorf("create private remote staging: %w", err)
	}
	defer func() {
		if err := c.run("rm -rf -- "+shellQuote(dir), io.Discard); err != nil {
			log("Private provisioning staging cleanup failed; remove the staged directory on the remote host.")
		}
	}()
	log("Uploading verified Linux/" + arch + " agent and pinned Caddy; no remote compiler is needed.")
	var checksums strings.Builder
	for _, asset := range []struct{ name, path string }{{"posternproxy-agent", agentPath}, {"caddy", caddyPath}} {
		f, err := os.Open(asset.path)
		if err != nil {
			return err
		}
		hash := sha256.New()
		uploadErr := c.upload(dir+"/"+asset.name, io.TeeReader(f, hash))
		closeErr := f.Close()
		if uploadErr != nil {
			return uploadErr
		}
		if closeErr != nil {
			return closeErr
		}
		fmt.Fprintf(&checksums, "%x  %s\n", hash.Sum(nil), asset.name)
	}
	config, err := json.Marshal(agentconfig.Config{ControllerURL: input.ControllerURL, Token: input.AgentToken, ServerID: input.ServerID})
	if err != nil {
		return err
	}
	if err := c.upload(dir+"/agent.json", bytes.NewReader(config)); err != nil {
		return err
	}
	if err := c.upload(dir+"/checksums.txt", strings.NewReader(checksums.String())); err != nil {
		return err
	}
	if err := c.run("chmod 0700 -- "+shellQuote(dir+"/posternproxy-agent"), io.Discard); err != nil {
		return err
	}
	if input.Version != "" {
		output, err := c.output(shellQuote(dir+"/posternproxy-agent") + " version")
		if err != nil {
			return fmt.Errorf("agent artifact cannot report its version; use the matching bundle")
		}
		var identity struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(output, &identity); err != nil || identity.Version != input.Version {
			return fmt.Errorf("agent artifact version does not match the controller; use the same release bundle")
		}
	}
	writer := &lineWriter{onLine: log}
	err = c.run(prefix+shellQuote(dir+"/posternproxy-agent")+" install --config-file "+shellQuote(dir+"/agent.json")+" --assets-dir "+shellQuote(dir), writer)
	writer.flush()
	if err != nil {
		return fmt.Errorf("remote agent installer failed: %w", err)
	}
	log("Remote services installed; waiting for authenticated configuration application.")
	return nil
}

func remoteArchitecture(output string) (string, error) {
	fields := strings.Fields(output)
	if len(fields) == 2 && fields[0] == "Linux" {
		switch fields[1] {
		case "x86_64", "amd64":
			return "amd64", nil
		case "aarch64", "arm64":
			return "arm64", nil
		}
	}
	return "", fmt.Errorf("remote architecture is not Linux amd64 or arm64")
}

func selectArtifact(configured, name, arch string, caddy bool) (string, error) {
	dirs := []string{filepath.Dir(configured), ".", "bin"}
	if caddy {
		dirs = append(dirs, "/usr/bin", "/usr/local/bin")
	}
	var candidates []string
	for _, dir := range dirs {
		candidates = append(candidates, filepath.Join(dir, name+"-linux-"+arch))
	}
	if configured != "" {
		candidates = append(candidates, configured)
	}
	for _, dir := range dirs {
		candidates = append(candidates, filepath.Join(dir, name))
	}
	for _, path := range candidates {
		if err := installer.VerifyArchitecture(path, arch); err != nil {
			continue
		}
		info, err := buildinfo.ReadFile(path)
		if err != nil {
			continue
		}
		if !caddy && info.Path != "github.com/posternproxy/posternproxy/cmd/posternproxy-agent" {
			continue
		}
		osOK, archOK := false, false
		for _, setting := range info.Settings {
			osOK = osOK || (setting.Key == "GOOS" && setting.Value == "linux")
			archOK = archOK || (setting.Key == "GOARCH" && setting.Value == arch)
		}
		if !osOK || !archOK {
			continue
		}
		if caddy {
			if err := caddybuild.VerifyArtifact(path); err != nil {
				continue
			}
		}
		return path, nil
	}
	return "", fmt.Errorf("no compatible %s artifact for Linux/%s; install the matching build on the controller before provisioning", name, arch)
}
