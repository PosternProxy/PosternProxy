package models

// PortForward is a TCP/UDP port-forwarding rule applied at the OS level.
type PortForward struct {
	ID           int64  `json:"id"`
	ServerID     int64  `json:"server_id"`
	Protocol     string `json:"protocol"`
	IncomingPort int    `json:"incoming_port"`
	ForwardHost  string `json:"forward_host"`
	ForwardPort  int    `json:"forward_port"`
	Interface    string `json:"interface"`
	Method       string `json:"method"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}
