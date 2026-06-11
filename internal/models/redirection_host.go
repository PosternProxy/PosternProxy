package models

import "encoding/json"

// RedirectionHost is a virtual host that issues an HTTP redirect to another URL.
type RedirectionHost struct {
	ID                int64           `json:"id"`
	ServerID          int64           `json:"server_id"`
	DomainNames       json.RawMessage `json:"domain_names"`
	ForwardScheme     string          `json:"forward_scheme"`
	ForwardDomainName string          `json:"forward_domain_name"`
	ForwardPath       string          `json:"forward_path"`
	PreservePath      bool            `json:"preserve_path"`
	StatusCode        int             `json:"status_code"`
	CertificateID     *int64          `json:"certificate_id"`
	SSLForced         bool            `json:"ssl_forced"`
	HSTSEnabled       bool            `json:"hsts_enabled"`
	AccessListID      *int64          `json:"access_list_id"`
	AdvancedConfig    string          `json:"advanced_config"`
	Enabled           bool            `json:"enabled"`
	CaddyID           string          `json:"caddy_id,omitempty"`
	CreatedAt         string          `json:"created_at"`
	UpdatedAt         string          `json:"updated_at"`
}
