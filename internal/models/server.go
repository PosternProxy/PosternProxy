package models

// Server represents a managed proxy server — either the local controller
// instance (is_local = true, id = 1) or a remote server managed via agent.
type Server struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Hostname       string  `json:"hostname"`
	SSHPort        int     `json:"ssh_port"`
	SSHUser        string  `json:"ssh_user"`
	SSHAuthMethod  string  `json:"ssh_auth_method"`
	AgentTokenHash string  `json:"-"`
	Status         string  `json:"status"`
	ProvisionLog   string  `json:"provision_log,omitempty"`
	IsLocal        bool    `json:"is_local"`
	LastSeenAt     *string `json:"last_seen_at"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`

	// Online is populated at read-time from the hub, not stored in DB.
	Online bool `json:"online"`

	// Health fields — persisted from agent status heartbeats.
	CaddyVersion  string  `json:"caddy_version"`
	UptimeSeconds int64   `json:"uptime_seconds"`
	ActiveRoutes  int     `json:"active_routes"`
	LastStatusAt  *string `json:"last_status_at"`

	// DeploymentType controls how the agent is installed on a remote host.
	// "systemd" uses the SSH+xcaddy provisioner; "docker" uses docker run.
	DeploymentType string `json:"deployment_type"`

	// HTTP2Enabled controls whether this server's HTTPS listener offers HTTP/2.
	// It is a server-wide property (not per-host); default on.
	HTTP2Enabled bool `json:"http2_enabled"`

	// SSHHostKey is the SHA256 fingerprint of the remote host's SSH public key,
	// stored on first successful provisioning (TOFU). Not exposed via API.
	SSHHostKey string `json:"-"`
}

// CreateServerInput is the payload for registering a new remote server.
type CreateServerInput struct {
	Name           string `json:"name"`
	Hostname       string `json:"hostname"`
	SSHPort        int    `json:"ssh_port"`
	SSHUser        string `json:"ssh_user"`
	SSHAuthMethod  string `json:"ssh_auth_method"` // "password" | "key"
	SSHCredentials string `json:"ssh_credentials"` // password or private key PEM
	DeploymentType string `json:"deployment_type"` // "systemd" | "docker"
}
