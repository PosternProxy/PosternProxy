package models

// StreamHost is a Layer-4 TCP/UDP port-forwarding rule managed through Caddy's
// layer4 app. Unlike PortForward it operates entirely within the Caddy process.
type StreamHost struct {
	ID               int64   `json:"id"`
	ServerID         int64   `json:"server_id"`
	IncomingPort     int     `json:"incoming_port"`
	ForwardHost      string  `json:"forward_host"`
	ForwardPort      int     `json:"forward_port"`
	Protocol         string  `json:"protocol"`
	TCPSNIMatch      *string `json:"tcp_sni_match"`
	TCPProxyProtocol bool    `json:"tcp_proxy_protocol"`
	UDPTimeout       int     `json:"udp_timeout"`
	Enabled          bool    `json:"enabled"`
	CaddyID          string  `json:"caddy_id,omitempty"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}
