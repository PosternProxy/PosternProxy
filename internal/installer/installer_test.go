package installer

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/posternproxy/posternproxy/internal/security"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func fixtureInstallation(t *testing.T) (installation, string, *[]string) {
	t.Helper()
	root, stage := t.TempDir(), t.TempDir()
	for _, name := range []string{"usr/bin", "usr/local/bin", "etc/posternproxy"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"caddy", "posternproxy", "posternproxy-agent"} {
		if err := os.WriteFile(filepath.Join(stage, name), []byte(name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	var commands []string
	i := installation{
		root: root,
		opts: Options{Port: 81, SkipPackages: true},
		run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return nil, nil
		},
	}
	return i, stage, &commands
}

func TestInstallPreservesConfigurationAndFirewall(t *testing.T) {
	i, stage, commands := fixtureInstallation(t)
	config := "POSTERNPROXY_JWT_SECRET=existing-secret\nPOSTERNPROXY_LISTEN=:8081\n"
	if err := os.WriteFile(i.path("/etc/posternproxy/config.env"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if err := i.apply(context.Background(), stage); err != nil {
		t.Fatal(err)
	}
	if err := i.apply(context.Background(), stage); err != nil {
		t.Fatalf("repeat installation: %v", err)
	}
	got, err := os.ReadFile(i.path("/etc/posternproxy/config.env"))
	if err != nil || string(got) != config {
		t.Fatalf("existing config changed: %q, %v", got, err)
	}
	for _, cmd := range *commands {
		if strings.HasPrefix(cmd, "ufw ") || strings.Contains(cmd, "sshd") || strings.HasPrefix(cmd, "go ") {
			t.Fatalf("unexpected system modification: %s", cmd)
		}
	}
}

func TestInstallNewConfigAndCaddyBootstrap(t *testing.T) {
	i, stage, _ := fixtureInstallation(t)
	if err := i.apply(context.Background(), stage); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(i.path("/etc/posternproxy/config.env"))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{
		"POSTERNPROXY_LISTEN=:81\n",
		"POSTERNPROXY_DATA_DIR=/var/lib/posternproxy\n",
		"POSTERNPROXY_AGENT_BIN=/usr/local/bin/posternproxy-agent\n",
		"POSTERNPROXY_BEHIND_TLS_PROXY=false\n",
	} {
		if !strings.Contains(string(data), value) {
			t.Fatalf("missing configuration %q", value)
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(i.path("/etc/posternproxy/config.env"))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("unprotected config: %v, %v", info, err)
		}
	}
	bootstrap, err := os.ReadFile(i.path("/etc/caddy/posternproxy.json"))
	if err != nil || !strings.Contains(string(bootstrap), `"listen":"127.0.0.1:2019"`) {
		t.Fatalf("missing loopback bootstrap: %s, %v", bootstrap, err)
	}
	if strings.Contains(controllerUnit, "CAP_NET_ADMIN") || !strings.Contains(controllerUnit, "ProtectSystem=strict") {
		t.Fatal("controller unit has unexpected privileges")
	}
}

func TestFirewallAddsSSHFirstWithoutReset(t *testing.T) {
	i, stage, commands := fixtureInstallation(t)
	i.opts.ConfigureFirewall, i.opts.SSHPort = true, 2222
	if err := i.apply(context.Background(), stage); err != nil {
		t.Fatal(err)
	}
	var firewall []string
	for _, cmd := range *commands {
		if strings.HasPrefix(cmd, "ufw ") {
			firewall = append(firewall, cmd)
		}
	}
	want := "ufw allow 2222/tcp;ufw allow 80/tcp;ufw allow 443/tcp;ufw allow 443/udp;ufw allow 81/tcp;ufw --force enable"
	if strings.Join(firewall, ";") != want {
		t.Fatalf("unexpected firewall operations: %v", firewall)
	}
}

func TestInstallStopsOnServiceFailure(t *testing.T) {
	i, stage, _ := fixtureInstallation(t)
	var laterServiceStarted bool
	i.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "systemctl" && strings.Join(args, " ") == "restart caddy" {
			return nil, errors.New("caddy failed")
		}
		if name == "systemctl" && strings.Join(args, " ") == "restart posternproxy" {
			laterServiceStarted = true
		}
		return nil, nil
	}
	err := i.apply(context.Background(), stage)
	if err == nil || !strings.Contains(err.Error(), "caddy failed") || laterServiceStarted {
		t.Fatalf("service failure not propagated: %v (continued=%v)", err, laterServiceStarted)
	}
}

func TestReadinessDoesNotReportSuccessOnActiveServiceAlone(t *testing.T) {
	i, _, _ := fixtureInstallation(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := i.waitReady(ctx); err == nil {
		t.Fatal("reported ready without reachable APIs")
	}
}

func TestPreflightRejectsUnsupportedHostBeforeCommands(t *testing.T) {
	i, _, commands := fixtureInstallation(t)
	if err := os.WriteFile(i.path("/etc/os-release"), []byte("ID=ubuntu\nVERSION_ID=20.04\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := i.preflight(context.Background()); err == nil {
		t.Fatal("unsupported host accepted")
	}
	if len(*commands) != 0 {
		t.Fatalf("commands ran on unsupported host: %v", *commands)
	}
}

func TestExistingListenPortUsedForFirewallAndReadiness(t *testing.T) {
	i, _, _ := fixtureInstallation(t)
	config := "POSTERNPROXY_LISTEN='[::]:8081'\nPOSTERNPROXY_CADDY_API=http://localhost:2020\n"
	if err := os.WriteFile(i.path("/etc/posternproxy/config.env"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if err := i.readExistingConfig(); err != nil {
		t.Fatal(err)
	}
	if i.opts.Port != 8081 || i.controllerURL != "http://[::1]:8081/api/version" || i.caddyURL != "http://localhost:2020/config/" {
		t.Fatalf("existing endpoints ignored: %+v", i)
	}
}

func TestPreflightRefusesNewSecretForExistingDatabase(t *testing.T) {
	i, _, commands := fixtureInstallation(t)
	if err := os.MkdirAll(i.path("/var/lib/posternproxy"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(i.path("/var/lib/posternproxy/posternproxy.db"), []byte("existing database"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := i.preflight(context.Background()); err == nil || !strings.Contains(err.Error(), "encryption secret") {
		t.Fatalf("existing data would receive a replacement secret: %v", err)
	}
	if len(*commands) != 0 {
		t.Fatalf("modified host before detecting missing secret: %v", *commands)
	}
}

func TestExistingCustomStorageRejectedBeforeInstallation(t *testing.T) {
	for _, key := range []string{"POSTERNPROXY_DATA_DIR", "POSTERNPROXY_DB_PATH", "POSTERNPROXY_CERT_DIR"} {
		t.Run(key, func(t *testing.T) {
			i, _, commands := fixtureInstallation(t)
			config := key + "=/srv/posternproxy/custom\n"
			if err := os.WriteFile(i.path("/etc/posternproxy/config.env"), []byte(config), 0600); err != nil {
				t.Fatal(err)
			}
			if err := i.readExistingConfig(); err == nil || !strings.Contains(err.Error(), "custom storage layout") {
				t.Fatalf("would replace unit without custom path access: %v", err)
			}
			if len(*commands) != 0 {
				t.Fatalf("modified system before validating existing paths: %v", *commands)
			}
		})
	}
}

func TestDevelopmentRevisionRequiresExplicitArtifacts(t *testing.T) {
	opts := Options{Port: 81}
	if err := opts.Validate("v0.2.0-beta-7-gabcdef"); err == nil {
		t.Fatal("git revision treated as a published release")
	}
}

func TestReadinessChecksBothAPIs(t *testing.T) {
	i, _, _ := fixtureInstallation(t)
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	i.caddyURL = server.URL + "/config/"
	i.controllerURL = server.URL + "/api/version"
	if err := i.waitReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	if strings.Join(paths, ",") != "/config/,/api/version" {
		t.Fatalf("missing readiness probes: %v", paths)
	}
}

func TestReadinessRequiresEachService(t *testing.T) {
	i, _, _ := fixtureInstallation(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	i.caddyURL, i.controllerURL = server.URL, server.URL
	i.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		if name == "systemctl" && args[len(args)-1] == "posternproxy" {
			return nil, errors.New("inactive")
		}
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := i.waitReady(ctx); err == nil {
		t.Fatal("reported ready while only Caddy was active")
	}
}

func TestFail2banFilterMatchesFailedLoginRequestLog(t *testing.T) {
	var output bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&output)
	t.Cleanup(func() { log.Logger = previous })
	router := chi.NewRouter()
	router.Use(security.ClientIdentity(nil))
	router.Use(security.RequestLogger)
	router.Post("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test-Success") == "yes" {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})
	var pattern string
	for _, file := range systemFiles {
		if file.path == "/etc/fail2ban/filter.d/posternproxy.conf" {
			for _, line := range strings.Split(file.content, "\n") {
				if strings.HasPrefix(line, "failregex = ") {
					pattern = strings.TrimPrefix(line, "failregex = ")
				}
			}
		}
	}
	if pattern == "" {
		t.Fatal("missing fail2ban pattern")
	}
	for _, tc := range []struct {
		ip      string
		success bool
	}{
		{"192.0.2.1", false},
		{"2001:db8::1", false},
		{"192.0.2.1", true},
	} {
		output.Reset()
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		req.RemoteAddr = net.JoinHostPort(tc.ip, "12345")
		if tc.success {
			req.Header.Set("X-Test-Success", "yes")
		}
		router.ServeHTTP(httptest.NewRecorder(), req)
		filter := regexp.MustCompile(strings.ReplaceAll(pattern, "<HOST>", regexp.QuoteMeta(tc.ip)))
		if filter.Match(output.Bytes()) == tc.success {
			t.Fatalf("filter match incorrect for success=%v: %s", tc.success, output.String())
		}
	}
}
