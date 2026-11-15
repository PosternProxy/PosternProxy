// Package agent defines the wire protocol used between the controller hub
// and remote caddyproxy-agent processes.  Both sides import this package
// so the message types are always in sync.
package agent

import "encoding/json"

// ── Message types ─────────────────────────────────────────────────────────────

// Message is the top-level envelope sent over the WebSocket connection in both
// directions.  The Payload field carries the JSON-encoded inner struct whose
// type is identified by Kind.
type Message struct {
	ID       string          `json:"id,omitempty"`
	Revision int64           `json:"revision,omitempty"`
	Kind     string          `json:"kind"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

// ── Controller → Agent commands ───────────────────────────────────────────────

const (
	KindConfigFullSync = "config_full_sync"
	KindConfigPatch    = "config_patch"
	KindConfigDelete   = "config_delete"
	KindHealthCheck    = "health_check"
)

// CertFile carries the PEM bytes for one custom certificate so the agent
// can write them to disk before the config that references them is applied.
type CertFile struct {
	CertID  int64  `json:"cert_id"`
	CertPEM []byte `json:"cert_pem"`
	KeyPEM  []byte `json:"key_pem"`
}

// ConfigFullSyncPayload carries a complete Caddy JSON config and any custom
// certificate files needed by that config.
type ConfigFullSyncPayload struct {
	Config json.RawMessage `json:"config"`
	Certs  []CertFile      `json:"certs,omitempty"`
}

// ConfigPatchPayload carries a Caddy admin API path and replacement value.
type ConfigPatchPayload struct {
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// ConfigDeletePayload carries a Caddy admin API path to remove.
type ConfigDeletePayload struct {
	Path string `json:"path"`
}

// ── Agent → Controller responses ──────────────────────────────────────────────

const (
	KindStatus        = "status"
	KindCommandResult = "command_result"
	KindHealthReport  = "health_report"
)

// StatusPayload is the periodic heartbeat from the agent.
type StatusPayload struct {
	ServerID     int64  `json:"server_id"`
	CaddyVersion string `json:"caddy_version"`
	Uptime       int64  `json:"uptime_seconds"`
	ActiveRoutes int    `json:"active_routes"`
}

// CommandResultPayload reports success or failure of a command.
type CommandResultPayload struct {
	CommandID   string `json:"command_id,omitempty"`
	Revision    int64  `json:"revision,omitempty"`
	CommandKind string `json:"command_kind"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}

// HealthReportPayload carries system metrics from the agent host.
type HealthReportPayload struct {
	ServerID int64   `json:"server_id"`
	CPUPct   float64 `json:"cpu_pct"`
	MemPct   float64 `json:"mem_pct"`
	DiskPct  float64 `json:"disk_pct"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// NewMessage encodes payload into a Message with the given kind.
func NewMessage(kind string, payload any) (Message, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return Message{}, err
	}
	return Message{Kind: kind, Payload: b}, nil
}
