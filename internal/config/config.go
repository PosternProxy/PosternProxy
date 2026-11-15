package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/posternproxy/posternproxy/internal/security"
)

type Config struct {
	TrustedProxies []string
	ListenAddr     string
	DBPath         string
	CaddyAPI       string
	CertDir        string
	JWTSecret      string
	DataDir        string
	EncryptionKey  [32]byte // AES-256-GCM key derived from JWTSecret

	// Multi-server
	ControllerURL string // Advertised WebSocket URL for agents to connect back to
	AgentBinPath  string // Optional: path to local posternproxy-agent binary for SCP

	// TLS
	BehindTLSProxy bool // True when a TLS-terminating proxy (e.g. Caddy) fronts the controller

	// CORS
	AllowedOrigins []string // Origins allowed for CORS (e.g. ["http://localhost:3000"] in dev)

	// Version is set by main at startup (from ldflags or build info).
	Version string
}

func Load() *Config {
	cfg := &Config{
		TrustedProxies: parseOrigins(envOr("POSTERNPROXY_TRUSTED_PROXIES", "")),
		ListenAddr:     envOr("POSTERNPROXY_LISTEN", ":81"),
		DBPath:         envOr("POSTERNPROXY_DB_PATH", "data/posternproxy.db"),
		CaddyAPI:       envOr("POSTERNPROXY_CADDY_API", "http://localhost:2019"),
		CertDir:        envOr("POSTERNPROXY_CERT_DIR", "data/certs"),
		DataDir:        envOr("POSTERNPROXY_DATA_DIR", "data"),
		ControllerURL:  envOr("POSTERNPROXY_CONTROLLER_URL", ""),
		AgentBinPath:   envOr("POSTERNPROXY_AGENT_BIN", ""),
		AllowedOrigins: parseOrigins(envOr("POSTERNPROXY_ALLOWED_ORIGINS", "")),
		BehindTLSProxy: envOr("POSTERNPROXY_BEHIND_TLS_PROXY", "true") != "false",
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
	data, err := os.ReadFile(path)
	if err == nil {
		if s := strings.TrimSpace(string(data)); s != "" {
			return s
		}
	}
	secret := generateSecret()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		panic("posternproxy: failed to create data directory for secret: " + err.Error())
	}
	if err := os.WriteFile(path, []byte(secret+"\n"), 0600); err != nil {
		panic("posternproxy: failed to persist JWT secret to " + path + ": " + err.Error())
	}
	return secret
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
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
