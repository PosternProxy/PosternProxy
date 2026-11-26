// posternproxy-agent runs on remote servers and maintains a persistent
// outbound WebSocket connection to the PosternProxy controller.
// It receives configuration commands and applies them to the local
// Caddy instance and OS network rules.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/posternproxy/posternproxy/internal/agent"
	"github.com/posternproxy/posternproxy/internal/agentconfig"
	"github.com/posternproxy/posternproxy/internal/buildinfo"
	"github.com/posternproxy/posternproxy/internal/certstore"
	"github.com/posternproxy/posternproxy/internal/platformservice"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Version is set at build time via -ldflags "-X main.Version=...".
// Falls back to VCS info from the Go module build stamp, or "dev".
var Version = "dev"

func main() {
	if Version == "dev" {
		Version = buildinfo.Version()
	}
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	if len(os.Args) == 2 && os.Args[1] == "version" {
		buildinfo.WriteVersion(os.Stdout, "posternproxy-agent", Version)
		return
	}
	if handled, err := platformservice.Dispatch(os.Args[1:], "posternproxy-agent", runAgent); handled {
		if err != nil {
			log.Fatal().Err(err).Msg("Windows service operation failed")
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "install" {
		runInstall(os.Args[2:])
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runAgent(ctx); err != nil {
		log.Fatal().Err(err).Msg("agent stopped")
	}
}

func runAgent(ctx context.Context) error {
	controllerURL := os.Getenv("POSTERNPROXY_CONTROLLER_URL")
	token := os.Getenv("POSTERNPROXY_AGENT_TOKEN")
	serverIDStr := os.Getenv("POSTERNPROXY_SERVER_ID")
	caddyAPI := envOr("POSTERNPROXY_CADDY_API", "http://localhost:2019")

	serverID, err := strconv.ParseInt(serverIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid POSTERNPROXY_SERVER_ID")
	}
	if err := (agentconfig.Config{ControllerURL: controllerURL, Token: token, ServerID: serverID}).Validate(); err != nil {
		return err
	}

	log.Info().Str("version", Version).Int64("server_id", serverID).Msg("starting posternproxy-agent")

	a := &agentClient{
		ctx:           ctx,
		controllerURL: controllerURL,
		token:         token,
		serverID:      serverID,
		caddyAPI:      caddyAPI,
		startedAt:     time.Now(),
	}
	a.run(ctx)
	return nil
}

// caddyHTTPClient is used for all Caddy admin API calls. The timeout
// prevents the agent from blocking indefinitely if Caddy hangs.
var caddyHTTPClient = &http.Client{Timeout: 30 * time.Second}

type agentClient struct {
	ctx           context.Context
	controllerURL string
	token         string
	serverID      int64
	caddyAPI      string
	startedAt     time.Time
}

// run connects with exponential back-off and loops forever.
func (a *agentClient) run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		connectedAt := time.Now()
		if err := a.connect(ctx); err != nil && ctx.Err() == nil {
			log.Warn().Err(err).Dur("retry_in", backoff).Msg("agent disconnected")
		}
		// Reset backoff if the connection lasted long enough to be useful
		// (survived at least one heartbeat cycle).
		if time.Since(connectedAt) > 60*time.Second {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 60*time.Second {
			backoff *= 2
		}
	}
}

// connect opens one WebSocket session and processes messages until the connection drops.
func (a *agentClient) connect(ctx context.Context) error {
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

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), http.Header{"Authorization": {"Bearer " + a.token}})
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	stopClosing := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClosing()
	conn.SetReadLimit(64 << 20)
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
			select {
			case readCh <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
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
			material := make([]certstore.Material, 0, len(p.Certs))
			for _, cert := range p.Certs {
				material = append(material, certstore.Material{ID: cert.CertID, Pair: certstore.Pair{Certificate: cert.CertPEM, Key: cert.KeyPEM}})
			}
			root := envOr("POSTERNPROXY_CERT_RUNTIME_DIR", "/run/posternproxy-agent-certs")
			config, keep, err := certstore.StageConfig(p.Config, material, root)
			if err != nil {
				errMsg = "Certificate staging failed"
				log.Error().Err(err).Msg("stage required certificates")
				break
			}
			if err := a.caddyLoad(config); err != nil {
				errMsg = err.Error()
			} else {
				if err := certstore.PruneRuntime(root, keep); err != nil {
					errMsg = "Certificate retirement failed"
					log.Error().Err(err).Msg("retire inactive certificates")
				} else if err := certstore.PruneLegacy(envOr("POSTERNPROXY_CERT_DIR", "/var/lib/posternproxy-agent/certs")); err != nil {
					errMsg = "Legacy certificate cleanup failed"
					log.Error().Err(err).Msg("remove legacy certificate copies")
				} else {
					success = true
				}
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
	if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return
	}
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
	if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return
	}
	if err := conn.WriteJSON(msg); err != nil {
		log.Warn().Err(err).Msg("write status")
	}
}

// queryCaddyVersion reads the Server response header from the Caddy admin API.
func (a *agentClient) queryCaddyVersion() string {
	resp, err := a.caddyGet("/config/")
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
	resp, err := a.caddyGet("/config/apps/http/servers/")
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
	req, err := http.NewRequestWithContext(a.requestContext(), method, a.caddyAPI+path, r)
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

func (a *agentClient) requestContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *agentClient) caddyGet(path string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(a.requestContext(), http.MethodGet, a.caddyAPI+path, nil)
	if err != nil {
		return nil, err
	}
	return caddyHTTPClient.Do(request)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
