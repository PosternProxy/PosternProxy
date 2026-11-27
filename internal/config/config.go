package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/posternproxy/posternproxy/internal/security"
)

type Config struct {
	CaddyTrustedProxies string
	TrustedProxies      []string
	ListenAddr          string
	DBPath              string
	CaddyAPI            string
	CertDir             string
	CertRuntimeDir      string
	JWTSecret           string
	DataDir             string
	EncryptionKey       [32]byte // AES-256-GCM key derived from JWTSecret

	// Multi-server
	ControllerURL string // Advertised WebSocket URL for agents to connect back to
	CaddyBinPath  string
	AgentBinPath  string // Optional: path to local posternproxy-agent binary for SCP

	// TLS
	BehindTLSProxy bool // True when a TLS-terminating proxy (e.g. Caddy) fronts the controller

	// Security
	LoginRateLimit int // Max POST /login attempts per minute per client IP (default 5)

	// CORS
	AllowedOrigins []string // Origins allowed for CORS (e.g. ["http://localhost:3000"] in dev)

	// Version is set by main at startup (from ldflags or build info).
	Version string
}

func Load() *Config {
	cfg := &Config{
		CaddyTrustedProxies: envOr("POSTERNPROXY_CADDY_TRUSTED_PROXIES", ""),
		TrustedProxies:      parseOrigins(envOr("POSTERNPROXY_TRUSTED_PROXIES", "")),
		ListenAddr:          envOr("POSTERNPROXY_LISTEN", ":81"),
		DBPath:              envOr("POSTERNPROXY_DB_PATH", "data/posternproxy.db"),
		CaddyAPI:            envOr("POSTERNPROXY_CADDY_API", "http://localhost:2019"),
		CertDir:             envOr("POSTERNPROXY_CERT_DIR", "data/certs"),
		CertRuntimeDir:      envOr("POSTERNPROXY_CERT_RUNTIME_DIR", "data/caddy-certs"),
		DataDir:             envOr("POSTERNPROXY_DATA_DIR", "data"),
		ControllerURL:       envOr("POSTERNPROXY_CONTROLLER_URL", ""),
		AgentBinPath:        envOr("POSTERNPROXY_AGENT_BIN", envOr("POSTERNPROXY_AGENT_BIN_PATH", "/usr/local/bin/posternproxy-agent")),
		CaddyBinPath:        envOr("POSTERNPROXY_CADDY_BIN", "/usr/bin/caddy"),
		AllowedOrigins:      parseOrigins(envOr("POSTERNPROXY_ALLOWED_ORIGINS", "")),
		BehindTLSProxy:      envOr("POSTERNPROXY_BEHIND_TLS_PROXY", "true") != "false",
		LoginRateLimit:      envOrInt("POSTERNPROXY_LOGIN_RATE_LIMIT", 5),
	}

	// JWT secret priority: env var → persisted file → generate+persist.
	// The secret is also used to derive the AES-256 key that encrypts stored
	// SSH credentials, so it MUST be stable across restarts. Generating a new
	// secret on every startup (the old behaviour) permanently broke decryption
	// of any previously stored credentials.
	if secret := os.Getenv("POSTERNPROXY_JWT_SECRET"); secret != "" {
		cfg.JWTSecret = secret
	} else {
		cfg.JWTSecret = loadOrCreateSecret(filepath.Join(cfg.DataDir, "secret.key"))
	}

	cfg.EncryptionKey = security.DeriveKey(cfg.JWTSecret)
	return cfg
}

// loadOrCreateSecret reads the persisted secret from path. If the file does
// not exist (first boot), a new random secret is generated, written to path
// with mode 0600, and returned. The directory is created if necessary.
func loadOrCreateSecret(path string) string {
	data, err := os.ReadFile(path) // #nosec G304 -- path is the server's own JWT-secret file, not request input
	if err == nil {
		if s := strings.TrimSpace(string(data)); s != "" {
			return s
		}
		panic("posternproxy: persisted JWT secret is empty; restore the original secret before starting")
	}
	if !errors.Is(err, os.ErrNotExist) {
		panic("posternproxy: failed to read persisted JWT secret: " + err.Error())
	}
	secret := generateSecret()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		panic("posternproxy: failed to create data directory for secret: " + err.Error())
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600) // #nosec G304 -- path is the server's own JWT-secret file, not request input
	if errors.Is(err, os.ErrExist) {
		return loadOrCreateSecret(path)
	}
	if err != nil {
		panic("posternproxy: failed to persist JWT secret to " + path + ": " + err.Error())
	}
	if _, err := file.WriteString(secret + "\n"); err != nil {
		file.Close()
		panic("posternproxy: failed to write JWT secret: " + err.Error())
	}
	if err := file.Sync(); err != nil {
		file.Close()
		panic("posternproxy: failed to sync JWT secret: " + err.Error())
	}
	if err := file.Close(); err != nil {
		panic("posternproxy: failed to close JWT secret: " + err.Error())
	}
	return secret
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envOrInt reads a positive integer env var, falling back when unset, empty, or
// invalid. Non-positive values fall back so a misconfiguration cannot disable a
// limit entirely.
func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func parseOrigins(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, o := range strings.Split(s, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

func generateSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate JWT secret: " + err.Error())
	}
	return hex.EncodeToString(b)
}
