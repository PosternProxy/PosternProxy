package models

import "encoding/json"

// Certificate represents a TLS certificate managed by the application.
// Provider is either "auto" (Let's Encrypt via Caddy) or "custom" (user-uploaded
// PEM files). CertPath and KeyPath are only set for custom certificates.
type Certificate struct {
	ID           int64           `json:"id"`
	Name         string          `json:"name"`
	Provider     string          `json:"provider"`
	DomainNames  json.RawMessage `json:"domain_names"`
	CertPath     *string         `json:"cert_path,omitempty"`
	KeyPath      *string         `json:"key_path,omitempty"`
	ExpiresAt    *string         `json:"expires_at"`
	Status       string          `json:"status"`
	ErrorMessage *string         `json:"error_message,omitempty"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}
