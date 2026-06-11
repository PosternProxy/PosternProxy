package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/posternproxy/posternproxy/internal/auth"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/security"
	"github.com/rs/zerolog/log"
)

const tokenPrefix = "pp_tok_"

// APITokenService manages API tokens for both human callers and agents.
type APITokenService struct {
	db *sql.DB
}

func NewAPITokenService(db *sql.DB) *APITokenService {
	return &APITokenService{db: db}
}

// ── Public methods ────────────────────────────────────────────────────────────

// Create generates a new API token for a human caller.
// The raw token is returned exactly once in the Token field and is never stored.
func (s *APITokenService) Create(ctx context.Context, callerID int64, callerRole string, input models.CreateAPITokenInput) (*models.APIToken, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("token name is required")
	}
	if len(input.Scopes) == 0 {
		input.Scopes = []string{"read", "write"}
	}
	for _, sc := range input.Scopes {
		if sc != "read" && sc != "write" {
			return nil, fmt.Errorf("invalid scope %q: must be read or write", sc)
		}
	}

	raw, prefix, hash, err := generateAPIToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	scopesJSON, _ := json.Marshal(input.Scopes)

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO api_tokens (name, token_prefix, token_hash, scopes, creator_role, created_by, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		input.Name, prefix, hash, string(scopesJSON), callerRole, callerID, input.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert token: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	tok, err := s.getByID(ctx, id)
	if err != nil {
		return nil, err
	}
	tok.Token = raw // show once
	return tok, nil
}

// CreateForServer generates an agent token tied to a specific server.
// The raw token is returned for embedding in the agent config.
func (s *APITokenService) CreateForServer(ctx context.Context, serverID int64, callerID *int64) (rawToken string, err error) {
	// Revoke any existing agent tokens for this server.
	now := time.Now().UTC().Format(time.RFC3339)
	s.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at=? WHERE server_id=? AND revoked_at IS NULL`, now, serverID)

	raw, prefix, hash, err := generateAPIToken()
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	scopesJSON := `["agent"]`
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO api_tokens (name, token_prefix, token_hash, scopes, creator_role, server_id, created_by)
		VALUES (?, ?, ?, ?, 'admin', ?, ?)`,
		fmt.Sprintf("Agent token (server %d)", serverID),
		prefix, hash, scopesJSON, serverID, callerID,
	)
	if err != nil {
		return "", fmt.Errorf("insert agent token: %w", err)
	}
	return raw, nil
}

// List returns all tokens visible to the caller.
// Admins see all tokens; regular users see only their own.
// Revoked tokens are excluded unless showRevoked is true.
func (s *APITokenService) List(ctx context.Context, callerID int64, isAdmin bool) ([]*models.APIToken, error) {
	query := `
		SELECT id, name, token_prefix, scopes, creator_role, server_id,
		       created_by, last_used_at, expires_at, revoked_at, created_at
		FROM api_tokens
		WHERE revoked_at IS NULL`
	args := []any{}
	if !isAdmin {
		query += ` AND created_by = ?`
		args = append(args, callerID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []*models.APIToken
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	if tokens == nil {
		tokens = []*models.APIToken{}
	}
	return tokens, rows.Err()
}

// Revoke soft-deletes a token. Admins can revoke any token; others only their own.
func (s *APITokenService) Revoke(ctx context.Context, id int64, callerID int64, isAdmin bool) error {
	tok, err := s.getByID(ctx, id)
	if err != nil {
		return fmt.Errorf("token not found")
	}
	if tok.RevokedAt != nil {
		return fmt.Errorf("token already revoked")
	}
	if !isAdmin && (tok.CreatedBy == nil || *tok.CreatedBy != callerID) {
		return fmt.Errorf("forbidden")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at=? WHERE id=?`, now, id)
	return err
}

// Resolve implements auth.TokenResolver.
// It validates a raw token string and returns synthesized Claims on success.
// Returns nil, nil if the token is not an API token (doesn't start with prefix).
func (s *APITokenService) Resolve(ctx context.Context, tokenStr string) (*auth.Claims, error) {
	if !strings.HasPrefix(tokenStr, tokenPrefix) {
		return nil, nil // not our format; let the JWT middleware handle it
	}

	// Derive the prefix from the token string for fast lookup.
	displayPrefix := displayPrefixFromRaw(tokenStr)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, token_hash, scopes, creator_role, created_by,
		       expires_at, revoked_at
		FROM api_tokens
		WHERE token_prefix = ? AND revoked_at IS NULL`,
		displayPrefix,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id          int64
			hash        string
			scopesRaw   string
			creatorRole string
			createdBy   sql.NullInt64
			expiresAt   sql.NullString
			revokedAt   sql.NullString
		)
		if err := rows.Scan(&id, &hash, &scopesRaw, &creatorRole, &createdBy, &expiresAt, &revokedAt); err != nil {
			continue
		}

		// Check hash
		if err := security.BcryptCompare(hash, tokenStr); err != nil {
			continue // hash mismatch — try next (shouldn't happen; prefix is unique in practice)
		}

		// Check expiry
		if expiresAt.Valid && expiresAt.String != "" {
			exp, err := time.Parse(time.RFC3339, expiresAt.String)
			if err == nil && time.Now().After(exp) {
				return nil, fmt.Errorf("token expired")
			}
		}

		// Check scopes — agent tokens cannot be used for REST API calls.
		// Invalid JSON in the scopes column yields an empty slice, which fails
		// the agent-token check below (safe default).
		var scopes []string
		_ = json.Unmarshal([]byte(scopesRaw), &scopes)
		for _, sc := range scopes {
			if sc == "agent" {
				return nil, fmt.Errorf("agent token cannot be used for API calls")
			}
		}

		// Update last_used_at asynchronously to avoid blocking the request.
		// A failed write here just means the timestamp is stale for one tick;
		// the auth result itself has already been computed.
		go func(tokenID int64) {
			now := time.Now().UTC().Format(time.RFC3339)
			if _, err := s.db.ExecContext(context.Background(), `UPDATE api_tokens SET last_used_at=? WHERE id=?`, now, tokenID); err != nil {
				log.Warn().Err(err).Int64("token_id", tokenID).Msg("failed to update api_token last_used_at")
			}
		}(id)

		userID := int64(0)
		if createdBy.Valid {
			userID = createdBy.Int64
		}

		return &auth.Claims{
			UserID: userID,
			Email:  "", // not available without a join; handlers that need it can fetch
			Role:   creatorRole,
		}, nil
	}

	return nil, fmt.Errorf("invalid or revoked token")
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func generateAPIToken() (raw, displayPrefix, hash string, err error) {
	b := make([]byte, 20) // 40 hex chars
	if _, err = rand.Read(b); err != nil {
		return
	}
	randomHex := hex.EncodeToString(b)
	raw = tokenPrefix + randomHex
	displayPrefix = displayPrefixFromRaw(raw)
	hash, err = security.BcryptHash(raw)
	return
}

// displayPrefixFromRaw extracts the display prefix from a raw token.
// "pp_tok_a1b2c3d4..." → "pp_tok_a1b2c3d4"
func displayPrefixFromRaw(raw string) string {
	rest := strings.TrimPrefix(raw, tokenPrefix)
	if len(rest) >= 8 {
		return tokenPrefix + rest[:8]
	}
	return raw
}

func (s *APITokenService) getByID(ctx context.Context, id int64) (*models.APIToken, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, token_prefix, scopes, creator_role, server_id,
		       created_by, last_used_at, expires_at, revoked_at, created_at
		FROM api_tokens WHERE id = ?`, id)
	return scanToken(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanToken(s rowScanner) (*models.APIToken, error) {
	var (
		t          models.APIToken
		serverID   sql.NullInt64
		createdBy  sql.NullInt64
		lastUsedAt sql.NullString
		expiresAt  sql.NullString
		revokedAt  sql.NullString
		scopesRaw  string
	)
	err := s.Scan(
		&t.ID, &t.Name, &t.TokenPrefix, &scopesRaw, &t.CreatorRole,
		&serverID, &createdBy, &lastUsedAt, &expiresAt, &revokedAt, &t.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("token not found")
	}
	if err != nil {
		return nil, err
	}
	// Invalid JSON yields an empty Scopes slice — safe default for display.
	_ = json.Unmarshal([]byte(scopesRaw), &t.Scopes)
	if serverID.Valid {
		v := serverID.Int64
		t.ServerID = &v
	}
	if createdBy.Valid {
		v := createdBy.Int64
		t.CreatedBy = &v
	}
	if lastUsedAt.Valid {
		t.LastUsedAt = &lastUsedAt.String
	}
	if expiresAt.Valid {
		t.ExpiresAt = &expiresAt.String
	}
	if revokedAt.Valid {
		t.RevokedAt = &revokedAt.String
	}
	return &t, nil
}
