package installer

import (
	"context"
	"crypto/rand"
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/posternproxy/posternproxy/internal/agentconfig"
	"github.com/posternproxy/posternproxy/internal/caddybuild"
	"github.com/posternproxy/posternproxy/internal/platformservice"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc/mgr"
)

func installWindows(ctx context.Context, opts Options, enrollment *agentconfig.Config, version string) error {
	if runtime.GOARCH != "amd64" {
		return fmt.Errorf("Windows installation supports amd64 only")
	}
	if !windows.GetCurrentProcessToken().IsElevated() {
		return fmt.Errorf("install requires an elevated administrator terminal")
	}
	if opts.ConfigureFirewall {
		return fmt.Errorf("configure Windows Firewall explicitly; --configure-firewall is a Linux option")
	}
	if enrollment != nil {
		if err := verifyControllerIdentity(ctx, *enrollment); err != nil {
			return err
		}
	}
	stage, err := os.MkdirTemp("", "posternproxy-windows-install-*")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(stage); err != nil {
			log.Warn().Err(err).Msg("remove Windows installation staging")
		}
	}()
	if err := stagePlatformArtifacts(ctx, opts, version, "windows", "amd64", stage); err != nil {
		return err
	}
	if err := caddybuild.VerifyArtifact(filepath.Join(stage, "caddy.exe")); err != nil {
		return err
	}
	if err := verifyWindowsAgent(ctx, filepath.Join(stage, "posternproxy-agent.exe"), version); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := verifyPlatformArchitecture(self, "windows", "amd64"); err != nil {
		return err
	}
	role, other := "posternproxy", "posternproxy-agent"
	if enrollment != nil {
		role, other = other, role
	}
	if err := copyFile(self, filepath.Join(stage, role+".exe"), 0700); err != nil {
		return err
	}
	bin, data, err := platformservice.Directories()
	if err != nil {
		return err
	}
	for _, path := range []string{bin, data} {
		if err := ensureWindowsRoot(path); err != nil {
			return err
		}
	}
	definitions := filepath.Join(data, "services")
	appData := filepath.Join(data, role)
	definitionPath := filepath.Join(definitions, role+".dat")
	for _, path := range []string{bin, data, definitions} {
		if err := trustedWindowsPath(path, false); err != nil {
			return err
		}
	}
	if err := trustedWindowsPath(definitionPath, true); err != nil {
		return err
	}
	definition := windowsDefinition(role, bin, appData, definitions, opts.Port)
	if old, err := platformservice.ReadDefinition(definitionPath); err == nil {
		if old.Name != role {
			return fmt.Errorf("installed service role mismatch")
		}
		definition = old
	} else if !os.IsNotExist(err) {
		return err
	} else if _, err := os.Stat(filepath.Join(appData, "posternproxy.db")); err == nil {
		return fmt.Errorf("existing data has no service definition; restore its original configuration before reinstalling")
	} else if !os.IsNotExist(err) {
		return err
	}
	if enrollment == nil {
		if err := applyWindowsSettings(opts.ConfigFile, definition.Environment); err != nil {
			return err
		}
	} else {
		definition.Environment["POSTERNPROXY_CONTROLLER_URL"] = enrollment.ControllerURL
		definition.Environment["POSTERNPROXY_AGENT_TOKEN"] = enrollment.Token
		definition.Environment["POSTERNPROXY_SERVER_ID"] = strconv.FormatInt(enrollment.ServerID, 10)
	}
	manager, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer manager.Disconnect()
	if service, err := manager.OpenService(other); err == nil {
		service.Close()
		return fmt.Errorf("controller and agent services cannot share ownership of Caddy on the same Windows machine")
	} else if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return err
	}
	services, err := windowsServices(manager, role, bin, definitions)
	defer func() {
		for _, service := range services {
			service.Close()
		}
	}()
	if err != nil {
		return err
	}
	if err := stopWindowsServices(ctx, services); err != nil {
		return err
	}
	appSID, err := serviceSID(role)
	if err != nil {
		return err
	}
	caddySID, err := serviceSID(platformservice.CaddyName)
	if err != nil {
		return err
	}
	if err := prepareWindowsLayout(bin, data, appData, definitions, appSID, caddySID); err != nil {
		return err
	}
	binaries := []string{role, "caddy"}
	if enrollment == nil {
		binaries = append(binaries, "posternproxy-agent")
	}
	for _, name := range binaries {
		if err := replaceWindowsBinary(filepath.Join(stage, name+".exe"), filepath.Join(bin, name+".exe")); err != nil {
			return err
		}
	}
	caddyConfig := filepath.Join(definitions, "caddy.json")
	if err := atomicWrite(caddyConfig, strings.NewReader(`{"admin":{"listen":"127.0.0.1:2019"}}`), 0600); err != nil {
		return err
	}
	if err := setWindowsACL(caddyConfig, map[string]string{caddySID: "RX"}); err != nil {
		return err
	}
	caddyData := filepath.Join(data, "caddy")
	caddyDefinition := platformservice.Definition{Name: platformservice.CaddyName, CaddyExecutable: filepath.Join(bin, "caddy.exe"),
		CaddyConfig: caddyConfig, LogPath: filepath.Join(caddyData, "service.log"),
		Environment: map[string]string{"XDG_DATA_HOME": caddyData, "XDG_CONFIG_HOME": caddyData}}
	if err := writeWindowsDefinition(filepath.Join(definitions, platformservice.CaddyName+".dat"), caddyDefinition, caddySID); err != nil {
		return err
	}
	if err := writeWindowsDefinition(definitionPath, definition, appSID); err != nil {
		return err
	}
	var initial *initialAdminCredentials
	if enrollment == nil {
		if _, err := os.Stat(definition.Environment["POSTERNPROXY_DB_PATH"]); os.IsNotExist(err) {
			initial = &initialAdminCredentials{Email: "admin@posternproxy.local", Password: rand.Text()}
			bootstrap := platformservice.Definition{Name: role, LogPath: definition.LogPath, Environment: map[string]string{
				"POSTERNPROXY_ADMIN_EMAIL": initial.Email, "POSTERNPROXY_ADMIN_PASSWORD": initial.Password}}
			if err := writeWindowsDefinition(definition.BootstrapPath, bootstrap, appSID); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		defer func() {
			if err := os.Remove(definition.BootstrapPath); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Msg("remove Windows installation bootstrap")
			}
		}()
	}
	for _, service := range services {
		if err := service.Start(); err != nil {
			return fmt.Errorf("start service %s: %w", service.Name, err)
		}
	}
	urls := []string{"http://127.0.0.1:2019/config/"}
	if enrollment == nil {
		host, port, err := net.SplitHostPort(definition.Environment["POSTERNPROXY_LISTEN"])
		if err != nil {
			return err
		}
		if host == "" || host == "0.0.0.0" {
			host = "127.0.0.1"
		} else if host == "::" {
			host = "::1"
		}
		urls = append(urls, "http://"+net.JoinHostPort(host, port)+"/api/version")
	}
	if err := waitWindowsReady(ctx, services, urls...); err != nil {
		return fmt.Errorf("Windows services did not become ready; inspect protected service logs: %w", err)
	}
	if enrollment == nil {
		if err := os.Remove(definition.BootstrapPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		printer := installation{initialAdmin: initial}
		if err := printer.printInitialAdmin(os.Stdout); err != nil {
			return err
		}
	}
	log.Info().Str("actor", "administrator").Str("action", "windows_install").Str("service", role).Str("version", version).Msg("Windows services installed; agent readiness requires controller acknowledgement")
	return nil
}

func verifyWindowsAgent(ctx context.Context, path, version string) error {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return err
	}
	if info.Path != "github.com/posternproxy/posternproxy/cmd/posternproxy-agent" {
		return fmt.Errorf("agent artifact belongs to another application")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "version").Output()
	if err != nil {
		return fmt.Errorf("verify agent version: %w", err)
	}
	var identity struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(output, &identity); err != nil || identity.Version != version {
		return fmt.Errorf("agent artifact version differs from the installer; use a matching bundle")
	}
	return nil
}

func windowsDefinition(role, bin, appData, definitions string, port int) platformservice.Definition {
	return platformservice.Definition{Name: role, LogPath: filepath.Join(appData, "service.log"), BootstrapPath: filepath.Join(definitions, role+"-bootstrap.dat"),
		Environment: map[string]string{"POSTERNPROXY_LISTEN": fmt.Sprintf(":%d", port), "POSTERNPROXY_DATA_DIR": appData,
			"POSTERNPROXY_DB_PATH": filepath.Join(appData, "posternproxy.db"), "POSTERNPROXY_CERT_DIR": filepath.Join(appData, "certs"),
			"POSTERNPROXY_CERT_RUNTIME_DIR": filepath.Join(appData, "runtime"), "POSTERNPROXY_CADDY_API": "http://127.0.0.1:2019",
			"POSTERNPROXY_AGENT_BIN": filepath.Join(bin, "posternproxy-agent.exe"), "POSTERNPROXY_CADDY_BIN": filepath.Join(bin, "caddy.exe"),
			"POSTERNPROXY_BEHIND_TLS_PROXY": "false"}}
}

func writeWindowsDefinition(path string, definition platformservice.Definition, sid string) error {
	if err := platformservice.WriteDefinition(path, definition); err != nil {
		return err
	}
	return setWindowsACL(path, map[string]string{sid: "RX"})
}

func prepareWindowsLayout(bin, data, appData, definitions, appSID, caddySID string) error {
	for path, grants := range map[string]map[string]string{
		bin: {appSID: "RX", caddySID: "RX"}, data: {appSID: "RX", caddySID: "RX"},
		definitions:                  {appSID: "TRAVERSE", caddySID: "TRAVERSE"},
		appData:                      {appSID: "FA", caddySID: "TRAVERSE"},
		filepath.Join(data, "caddy"): {caddySID: "FA"},
	} {
		if err := makeWindowsDirectory(path, grants); err != nil {
			return err
		}
	}
	return makeWindowsDirectory(filepath.Join(appData, "runtime"), map[string]string{appSID: "FA", caddySID: "RX"})
}
