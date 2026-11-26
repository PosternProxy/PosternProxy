package models

import "encoding/json"

// DeadHost is a virtual host that returns a configurable error response for all
// requests, used to explicitly park domains without forwarding traffic.
type DeadHost struct {
	ID            int64           `json:"id"`
	ServerID      int64           `json:"server_id"`
	DomainNames   json.RawMessage `json:"domain_names"`
	StatusCode    int             `json:"status_code"`
	CustomBody    string          `json:"custom_body"`
	HTTPOnly      bool            `json:"http_only"`
	CertificateID *int64          `json:"certificate_id"`
	AccessListID  *int64          `json:"access_list_id"`
	Enabled       bool            `json:"enabled"`
	CaddyID       string          `json:"caddy_id,omitempty"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}
