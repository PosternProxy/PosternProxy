// Package installer installs a controller and its runtime dependencies on Linux.
package installer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/posternproxy/posternproxy/internal/agentconfig"
	"github.com/posternproxy/posternproxy/internal/caddybuild"
	"github.com/rs/zerolog/log"
)

type Options struct {
	AssetsDir         string
	Release           string
	Port              int
	ConfigureFirewall bool
	SSHPort           int
	SkipPackages      bool
}

func (o Options) Validate(version string) error {
	if o.Port < 1 || o.Port > 65535 || o.Port == 80 || o.Port == 443 || o.Port == 2019 {
		return fmt.Errorf("management port must be between 1 and 65535 and differ from 80, 443, and 2019")
	}
	if o.Release != "" && !releaseTag.MatchString(o.Release) {
		return fmt.Errorf("--release must be an exact release tag such as v0.2.0-beta")
	}
	if o.AssetsDir == "" && o.Release == "" && (!releaseTag.MatchString(version) || strings.HasSuffix(version, "-dirty") || developmentRevision.MatchString(version)) {
		return fmt.Errorf("development builds require --assets-dir or an explicit --release")
	}
	if o.ConfigureFirewall && (o.SSHPort < 1 || o.SSHPort > 65535) {
		return fmt.Errorf("--configure-firewall requires --ssh-port with your actual SSH port (1–65535)")
	}
	return nil
}

type commandRunner func(context.Context, string, ...string) ([]byte, error)

type installation struct {
	opts                           Options
	root                           string
	run                            commandRunner
	controllerURL                  string
	caddyURL                       string
	agentConfig                    *agentconfig.Config
	initialAdmin                   *initialAdminCredentials
	initialAdminEnvironmentCreated bool
}

func (i installation) serviceUser() string {
	if i.agentConfig != nil {
		return "posternproxy-agent"
	}
	return "posternproxy"
}

func Run(ctx context.Context, opts Options, version string) error {
	if err := opts.Validate(version); err != nil {
		return err
	}
	if runtime.GOOS != "linux" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		return fmt.Errorf("install supports Linux amd64 and arm64 only")
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("install must run as root; use sudo ./posternproxy install")
	}
	i := installation{opts: opts, run: runCommand}
	defer func() {
		if err := i.removeInitialAdminEnvironment(); err != nil {
			log.Warn().Err(err).Msg("clean up initial admin staging")
		}
	}()
	if err := i.preflight(ctx); err != nil {
		return err
	}
	if err := i.readExistingConfig(); err != nil {
		return err
	}
	stage, err := os.MkdirTemp("", "posternproxy-install-*")
	if err != nil {
		return fmt.Errorf("create installation staging directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(stage); err != nil {
			log.Warn().Err(err).Msg("remove installer staging directory")
		}
	}()
	log.Info().Msg("verifying installation assets")
	if err := stageArtifacts(ctx, opts, version, runtime.GOARCH, stage); err != nil {
		return err
	}
	// The embedded contract is also used by builds and SSH provisioning.
	if err := caddybuild.VerifyArtifact(filepath.Join(stage, "caddy")); err != nil {
		return err
	}
	if _, err := i.run(ctx, "bash", "-c", caddybuild.Script+"\nverify_caddy \"$1\"", "verify-caddy", filepath.Join(stage, "caddy")); err != nil {
		return fmt.Errorf("verify Caddy build contract: %w", err)
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate controller executable: %w", err)
	}
	if err := copyFile(self, filepath.Join(stage, "posternproxy"), 0700); err != nil {
		return err
	}
	log.Info().Str("actor", "root").Str("source", "cli").Str("action", "controller_install").Str("version", version).Msg("applying verified installation")
	if err := i.apply(ctx, stage); err != nil {
		return err
	}
	if err := i.waitReady(ctx); err != nil {
		return err
	}
	if err := i.removeInitialAdminEnvironment(); err != nil {
		return err
	}
	if err := i.printInitialAdmin(os.Stdout); err != nil {
		return fmt.Errorf("installation succeeded but login output failed: %w", err)
	}
	log.Info().Str("actor", "root").Str("action", "controller_install").Str("result", "success").Int("port", i.opts.Port).Msg("PosternProxy installed")
	return nil
}

func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	// Installation runs privileged; ignore caller-provided executable search paths.
	var executable string
	for _, dir := range []string{"/usr/sbin", "/usr/bin", "/sbin", "/bin"} {
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			executable = candidate
			break
		}
	}
	if executable == "" {
		return nil, fmt.Errorf("required system command %s is not installed", name)
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "HOME=/root", "DEBIAN_FRONTEND=noninteractive", "LANG=C.UTF-8"}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (i installation) path(name string) string {
	if i.root == "" {
		return filepath.FromSlash(name)
	}
	return filepath.Join(i.root, filepath.FromSlash(strings.TrimPrefix(name, "/")))
}

func (i installation) preflight(ctx context.Context) error {
	if i.agentConfig == nil {
		if _, err := os.Stat(i.path("/etc/posternproxy/config.env")); os.IsNotExist(err) {
			if _, err := os.Stat(i.path("/var/lib/posternproxy/posternproxy.db")); err == nil {
				return fmt.Errorf("existing database has no config.env; restore its original configuration and encryption secret before installing")
			} else if !os.IsNotExist(err) {
				return err
			}
		} else if err != nil {
			return err
		}
	}
	data, err := os.ReadFile(i.path("/etc/os-release"))
	if err != nil {
		return fmt.Errorf("read supported operating system: %w", err)
	}
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			values[key] = strings.Trim(value, `"'`)
		}
	}
	id, version := values["ID"], values["VERSION_ID"]
	if !((id == "debian" && (version == "11" || version == "12")) || (id == "ubuntu" && (version == "22.04" || version == "24.04"))) {
		return fmt.Errorf("supported systems are Debian 11/12 and Ubuntu 22.04/24.04; found %s %s", id, version)
	}
	if _, err := os.Stat(i.path("/run/systemd/system")); err != nil {
		return fmt.Errorf("a running systemd host is required: %w", err)
	}
	_, err = i.run(ctx, "systemctl", "--version")
	return err
}

func (i *installation) readExistingConfig() error {
	data, err := os.ReadFile(i.path("/etc/posternproxy/config.env"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// Preserve the existing configuration rather than applying new-install flags.
	i.opts.Port = 81
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"'`)
		if value == "" {
			continue
		}
		switch key {
		case "POSTERNPROXY_DATA_DIR", "POSTERNPROXY_DB_PATH", "POSTERNPROXY_CERT_DIR":
			clean := path.Clean(value)
			if !path.IsAbs(value) || (clean != "/var/lib/posternproxy" && !strings.HasPrefix(clean, "/var/lib/posternproxy/")) {
				return fmt.Errorf("existing %s uses a custom storage layout; migrate it under /var/lib/posternproxy or retain your existing service setup before using install", key)
			}
		case "POSTERNPROXY_LISTEN":
			host, port, err := net.SplitHostPort(value)
			if err != nil {
				return fmt.Errorf("invalid management address in existing configuration: %w", err)
			}
			i.opts.Port, err = strconv.Atoi(port)
			if err != nil || i.opts.Port < 1 || i.opts.Port > 65535 {
				return fmt.Errorf("invalid management port in existing configuration")
			}
			if host == "" || host == "0.0.0.0" {
				host = "127.0.0.1"
			} else if host == "::" {
				host = "::1"
			}
			i.controllerURL = "http://" + net.JoinHostPort(host, port) + "/api/version"
		case "POSTERNPROXY_CADDY_API":
			u, err := url.Parse(value)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
				return fmt.Errorf("invalid Caddy API URL in existing configuration")
			}
			i.caddyURL = strings.TrimRight(value, "/") + "/config/"
		}
	}
	return nil
}

func (i *installation) apply(ctx context.Context, stage string) error {
	if !i.opts.SkipPackages {
		log.Info().Msg("installing system packages")
		if _, err := i.run(ctx, "apt-get", "update", "-qq"); err != nil {
			return err
		}
		packages := []string{"install", "-y", "-qq", "ca-certificates", "fail2ban", "iptables", "unattended-upgrades", "apt-listchanges"}
		if i.opts.ConfigureFirewall {
			packages = append(packages, "ufw")
		}
		if _, err := i.run(ctx, "apt-get", packages...); err != nil {
			return err
		}
	}
	user := i.serviceUser()
	for _, name := range []string{"caddy", user} {
		if _, err := i.run(ctx, "id", "-u", name); err != nil {
			if _, err := i.run(ctx, "useradd", "--system", "--no-create-home", "--shell", "/usr/sbin/nologin", name); err != nil {
				return err
			}
		}
	}
	if _, err := i.run(ctx, "getent", "group", "posternproxy-certs"); err != nil {
		if _, err := i.run(ctx, "groupadd", "--system", "posternproxy-certs"); err != nil {
			return err
		}
	}
	for _, dir := range []struct{ path, owner, mode string }{
		{"/var/lib/" + user, user, "0750"},
		{"/var/lib/" + user + "/certs", user, "0700"},
		{"/var/log/" + user, user, "0750"},
		{"/etc/" + user, user, "0750"},
		{"/var/lib/caddy", "caddy", "0700"},
		{"/etc/caddy", "root", "0755"},
	} {
		if _, err := i.run(ctx, "install", "-d", "-m", dir.mode, "-o", dir.owner, "-g", dir.owner, i.path(dir.path)); err != nil {
			return err
		}
	}
	if err := i.writeConfig(); err != nil {
		return err
	}
	if _, err := i.run(ctx, "chown", user+":"+user, i.path("/etc/"+user+"/config.env")); err != nil {
		return err
	}
	if err := i.prepareInitialAdmin(); err != nil {
		return err
	}
	for _, binary := range []struct{ name, dest string }{
		{"caddy", "/usr/bin/caddy"},
		{"posternproxy-agent", "/usr/local/bin/posternproxy-agent"},
		{"posternproxy", "/usr/local/bin/posternproxy"},
	} {
		if i.agentConfig != nil && binary.name == "posternproxy" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(i.path(binary.dest)), 0755); err != nil {
			return err
		}
		if err := copyFile(filepath.Join(stage, binary.name), i.path(binary.dest), 0755); err != nil {
			return err
		}
	}
	for _, file := range systemFiles {
		if i.agentConfig != nil && (file.path == "/etc/systemd/system/posternproxy.service" || strings.HasPrefix(file.path, "/etc/fail2ban/")) {
			continue
		}
		content := strings.ReplaceAll(file.content, "{{managementPort}}", strconv.Itoa(i.opts.Port))
		if err := i.writeSystemFile(file.path, content, file.mode, file.preserve); err != nil {
			return err
		}
	}
	if i.agentConfig != nil {
		if err := i.writeSystemFile("/etc/systemd/system/posternproxy-agent.service", agentUnit, 0644, false); err != nil {
			return err
		}
	}
	if _, err := i.run(ctx, "sysctl", "-p", i.path("/etc/sysctl.d/99-posternproxy.conf")); err != nil {
		return err
	}
	if i.opts.ConfigureFirewall {
		ports := []string{strconv.Itoa(i.opts.SSHPort) + "/tcp", "80/tcp", "443/tcp", "443/udp"}
		if i.agentConfig == nil {
			ports = append(ports, strconv.Itoa(i.opts.Port)+"/tcp")
		}
		for _, port := range ports {
			if _, err := i.run(ctx, "ufw", "allow", port); err != nil {
				return err
			}
		}
		if _, err := i.run(ctx, "ufw", "--force", "enable"); err != nil {
			return err
		}
	}
	for _, args := range [][]string{
		{"daemon-reload"},
		{"enable", "caddy", user, "fail2ban", "unattended-upgrades"},
		{"restart", "caddy"},
		{"restart", user},
		{"restart", "fail2ban", "unattended-upgrades"},
	} {
		if _, err := i.run(ctx, "systemctl", args...); err != nil {
			return err
		}
	}
	return nil
}

func (i installation) writeConfig() error {
	if i.agentConfig != nil {
		config := fmt.Sprintf("POSTERNPROXY_CONTROLLER_URL=%s\nPOSTERNPROXY_AGENT_TOKEN=%s\nPOSTERNPROXY_SERVER_ID=%d\nPOSTERNPROXY_CADDY_API=http://127.0.0.1:2019\nPOSTERNPROXY_CERT_RUNTIME_DIR=/run/posternproxy-agent-certs\n", strconv.Quote(i.agentConfig.ControllerURL), strconv.Quote(i.agentConfig.Token), i.agentConfig.ServerID)
		return i.writeSystemFile("/etc/posternproxy-agent/config.env", config, 0600, false)
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return fmt.Errorf("generate installation secret: %w", err)
	}
	config := fmt.Sprintf(`POSTERNPROXY_LISTEN=:%d
POSTERNPROXY_DATA_DIR=/var/lib/posternproxy
POSTERNPROXY_DB_PATH=/var/lib/posternproxy/posternproxy.db
POSTERNPROXY_JWT_SECRET=%s
POSTERNPROXY_CADDY_API=http://127.0.0.1:2019
POSTERNPROXY_CERT_DIR=/var/lib/posternproxy/certs
POSTERNPROXY_CERT_RUNTIME_DIR=/run/posternproxy-certs
POSTERNPROXY_AGENT_BIN=/usr/local/bin/posternproxy-agent
POSTERNPROXY_BEHIND_TLS_PROXY=false
`, i.opts.Port, hex.EncodeToString(secret[:]))
	return i.writeSystemFile("/etc/posternproxy/config.env", config, 0600, true)
}

func (i installation) writeSystemFile(name, content string, mode os.FileMode, preserve bool) error {
	path := i.path(name)
	if preserve {
		if _, err := os.Lstat(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return atomicWrite(path, strings.NewReader(content), mode)
}

func copyFile(src, dest string, mode os.FileMode) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open installation binary: %w", err)
	}
	defer f.Close()
	return atomicWrite(dest, f, mode)
}

func atomicWrite(dest string, r io.Reader, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(dest), ".posternproxy-*")
	if err != nil {
		return fmt.Errorf("stage %s: %w", dest, err)
	}
	defer func() {
		if err := os.Remove(f.Name()); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Msg("remove temporary installation file")
		}
	}()
	_, copyErr := io.Copy(f, r)
	modeErr := f.Chmod(mode)
	syncErr := f.Sync()
	closeErr := f.Close()
	for _, err := range []error{copyErr, modeErr, syncErr, closeErr} {
		if err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
	}
	if err := os.Rename(f.Name(), dest); err != nil {
		return fmt.Errorf("replace %s: %w", dest, err)
	}
	return nil
}

func (i installation) waitReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 3 * time.Second}
	if i.caddyURL == "" {
		i.caddyURL = "http://127.0.0.1:2019/config/"
	}
	if i.controllerURL == "" {
		i.controllerURL = "http://127.0.0.1:" + strconv.Itoa(i.opts.Port) + "/api/version"
	}
	for {
		active := true
		for _, service := range []string{"caddy", i.serviceUser(), "fail2ban", "unattended-upgrades"} {
			if _, err := i.run(ctx, "systemctl", "is-active", "--quiet", service); err != nil {
				active = false
				break
			}
		}
		if active {
			if probe(ctx, client, i.caddyURL) && (i.agentConfig != nil || probe(ctx, client, i.controllerURL)) {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("services did not become ready: %w; check systemctl status caddy posternproxy and /var/log/posternproxy/posternproxy.log", ctx.Err())
		case <-time.After(time.Second):
		}
	}
}

func probe(ctx context.Context, client *http.Client, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
