package models

// ConfigHistory is a point-in-time snapshot of a managed resource, stored
// before every mutating operation so the change can be rolled back.
type ConfigHistory struct {
	ID           int64  `json:"id"`
	ResourceType string `json:"resource_type"`
	ResourceID   int64  `json:"resource_id"`
	Action       string `json:"action"`
	Snapshot     string `json:"snapshot"` // raw JSON of entity before change
	CreatedBy    *int64 `json:"created_by,omitempty"`
	CreatedAt    string `json:"created_at"`
}
