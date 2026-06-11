package models

// APIToken is the public representation of a token — the raw secret is never
// stored and is only returned in the Token field immediately after creation.
type APIToken struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	TokenPrefix string   `json:"token_prefix"` // "pp_tok_a1b2c3d4" — display only
	Scopes      []string `json:"scopes"`
	CreatorRole string   `json:"creator_role"`
	ServerID    *int64   `json:"server_id,omitempty"`
	CreatedBy   *int64   `json:"created_by,omitempty"`
	LastUsedAt  *string  `json:"last_used_at"`
	ExpiresAt   *string  `json:"expires_at"`
	RevokedAt   *string  `json:"revoked_at"`
	CreatedAt   string   `json:"created_at"`

	// Token is populated exactly once — in the Create response.
	// It is never stored and is never returned again.
	Token string `json:"token,omitempty"`
}

// CreateAPITokenInput is the request body for POST /api/tokens.
type CreateAPITokenInput struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`     // ["read"] or ["read","write"]
	ExpiresAt *string  `json:"expires_at"` // optional ISO-8601 datetime; nil = never
}
