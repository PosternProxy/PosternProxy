// posternproxy-agent runs on remote servers and maintains a persistent
// outbound WebSocket connection to the PosternProxy controller.
// It receives configuration commands and applies them to the local
// Caddy instance and OS network rules.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/buildinfo"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Version is set at build time via -ldflags "-X main.Version=...".
// Falls back to VCS info from the Go module build stamp, or "dev".
var Version = buildinfo.Version()

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	controllerURL := mustEnv("POSTERNPROXY_CONTROLLER_URL") // wss://host:81
	token := mustEnv("POSTERNPROXY_AGENT_TOKEN")
	serverIDStr := mustEnv("POSTERNPROXY_SERVER_ID")
	caddyAPI := envOr("POSTERNPROXY_CADDY_API", "http://localhost:2019")

	serverID, err := strconv.ParseInt(serverIDStr, 10, 64)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid POSTERNPROXY_SERVER_ID")
	}

	log.Info().Str("version", Version).Int64("server_id", serverID).Msg("starting posternproxy-agent")

	a := &agentClient{
		controllerURL: controllerURL,
		token:         token,
		serverID:      serverID,
		caddyAPI:      caddyAPI,
		startedAt:     time.Now(),
	}
	a.run()
}

// caddyHTTPClient is used for all Caddy admin API calls. The timeout
// prevents the agent from blocking indefinitely if Caddy hangs.
var caddyHTTPClient = &http.Client{Timeout: 30 * time.Second}

type agentClient struct {
	controllerURL string
	token         string
	serverID      int64
	caddyAPI      string
	startedAt     time.Time
}

// run connects with exponential back-off and loops forever.
func (a *agentClient) run() {
	backoff := time.Second
	for {
		connectedAt := time.Now()
		if err := a.connect(); err != nil {
			log.Warn().Err(err).Dur("retry_in", backoff).Msg("agent disconnected")
		}
		// Reset backoff if the connection lasted long enough to be useful
		// (survived at least one heartbeat cycle).
		if time.Since(connectedAt) > 60*time.Second {
			backoff = time.Second
		}
		time.Sleep(backoff)
		if backoff < 60*time.Second {
			backoff *= 2
		}
	}
}

// connect opens one WebSocket session and processes messages until the connection drops.
func (a *agentClient) connect() error {
	u, err := url.Parse(a.controllerURL)
	if err != nil {
		return fmt.Errorf("invalid controller URL")
	}

	// Convert http(s) to ws(s) if caller gave an HTTP URL.
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}
	if u.User != nil || u.Host == "" || (u.Scheme != "ws" && u.Scheme != "wss") {
		return fmt.Errorf("invalid controller URL")
	}
	u.Path = "/ws/agent"
	u.RawPath = ""
	u.Fragment = ""
	q := url.Values{}
	q.Set("server_id", strconv.FormatInt(a.serverID, 10))
	u.RawQuery = q.Encode()

	log.Info().Str("controller_host", u.Host).Msg("connecting to controller")

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{"Authorization": {"Bearer " + a.token}})
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	log.Info().Msg("connected")

	// Send initial status report.
	a.sendStatus(conn)

	// Heartbeat ticker.
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	readCh := make(chan agent.Message, 16)
	errCh := make(chan error, 1)
	go func() {
		for {
			var msg agent.Message
			if err := conn.ReadJSON(&msg); err != nil {
				errCh <- err
				return
			}
			readCh <- msg
		}
	}()

	for {
		select {
		case msg := <-readCh:
			a.handleMessage(conn, msg)
		case <-ticker.C:
			a.sendStatus(conn)
		case err := <-errCh:
			return err
		}
	}
}

func (a *agentClient) handleMessage(conn *websocket.Conn, msg agent.Message) {
	var success bool
	var errMsg string

	switch msg.Kind {
	case agent.KindConfigFullSync:
		var p agent.ConfigFullSyncPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			if len(p.Certs) > 0 {
				if err := writeCerts(p.Certs); err != nil {
					log.Error().Err(err).Msg("failed to write cert files")
					errMsg = err.Error()
					break
				}
			}
			if err := a.caddyLoad(p.Config); err != nil {
				errMsg = err.Error()
			} else {
				success = true
			}
		}

	case agent.KindConfigPatch:
		var p agent.ConfigPatchPayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			if err := a.caddyPatch(p.Path, p.Value); err != nil {
				errMsg = err.Error()
			} else {
				success = true
			}
		}

	case agent.KindConfigDelete:
		var p agent.ConfigDeletePayload
		if err := json.Unmarshal(msg.Payload, &p); err == nil {
			if err := a.caddyDelete(p.Path); err != nil {
				errMsg = err.Error()
			} else {
				success = true
			}
		}

	case agent.KindHealthCheck:
		a.sendStatus(conn)
		return

	default:
		log.Warn().Str("kind", msg.Kind).Msg("unknown message kind")
		return
	}

	result, _ := agent.NewMessage(agent.KindCommandResult, agent.CommandResultPayload{
		CommandID:   msg.ID,
		Revision:    msg.Revision,
		CommandKind: msg.Kind,
		Success:     success,
		Error:       errMsg,
	})
	if err := conn.WriteJSON(result); err != nil {
		log.Warn().Err(err).Str("kind", msg.Kind).Msg("write command result")
	}
}

func (a *agentClient) sendStatus(conn *websocket.Conn) {
	version := a.queryCaddyVersion()
	routes := a.queryCaddyRouteCount()
	msg, _ := agent.NewMessage(agent.KindStatus, agent.StatusPayload{
		ServerID:     a.serverID,
		Uptime:       int64(time.Since(a.startedAt).Seconds()),
		CaddyVersion: version,
		ActiveRoutes: routes,
	})
	if err := conn.WriteJSON(msg); err != nil {
		log.Warn().Err(err).Msg("write status")
	}
}

// queryCaddyVersion reads the Server response header from the Caddy admin API.
func (a *agentClient) queryCaddyVersion() string {
	resp, err := caddyHTTPClient.Get(a.caddyAPI + "/config/")
	if err != nil {
		return ""
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if s := resp.Header.Get("Server"); strings.HasPrefix(s, "Caddy/") {
		return strings.TrimPrefix(s, "Caddy/")
	}
	return ""
}

// queryCaddyRouteCount returns total HTTP routes across all Caddy servers.
func (a *agentClient) queryCaddyRouteCount() int {
	resp, err := caddyHTTPClient.Get(a.caddyAPI + "/config/apps/http/servers/")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	var servers map[string]struct {
		Routes []json.RawMessage `json:"routes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&servers); err != nil {
		return 0
	}
	total := 0
	for _, srv := range servers {
		total += len(srv.Routes)
	}
	return total
}

// ── Caddy admin API helpers ───────────────────────────────────────────────────

func (a *agentClient) caddyLoad(config json.RawMessage) error {
	return a.caddyRequest("POST", "/load", config)
}

func (a *agentClient) caddyPatch(path string, value json.RawMessage) error {
	return a.caddyRequest("PATCH", "/config/"+path, value)
}

func (a *agentClient) caddyDelete(path string) error {
	return a.caddyRequest("DELETE", "/config/"+path, nil)
}

func (a *agentClient) caddyRequest(method, path string, body json.RawMessage) error {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, a.caddyAPI+path, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := caddyHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("caddy %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy %s %s: status %d: %s", method, path, resp.StatusCode, string(b))
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// writeCerts writes custom certificate files to the agent's cert directory so
// they are available on disk before the Caddy config that references them loads.
func writeCerts(certs []agent.CertFile) error {
	certDir := envOr("POSTERNPROXY_CERT_DIR", "/var/lib/posternproxy-agent/certs")
	for _, c := range certs {
		dir := filepath.Join(certDir, strconv.FormatInt(c.CertID, 10))
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create cert dir %s: %w", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cert.pem"), c.CertPEM, 0600); err != nil {
			return fmt.Errorf("write cert %d: %w", c.CertID, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "key.pem"), c.KeyPEM, 0600); err != nil {
			return fmt.Errorf("write key %d: %w", c.CertID, err)
		}
		log.Debug().Int64("cert_id", c.CertID).Str("dir", dir).Msg("cert files written")
	}
	return nil
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatal().Str("env", key).Msg("required environment variable not set")
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
