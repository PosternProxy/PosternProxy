package models

import "encoding/json"

// AuditLog records a single user-initiated change to a managed resource.
type AuditLog struct {
	ID           int64           `json:"id"`
	UserID       *int64          `json:"user_id"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   *int64          `json:"resource_id"`
	Details      json.RawMessage `json:"details"`
	IPAddress    *string         `json:"ip_address"`
	CreatedAt    string          `json:"created_at"`
}
