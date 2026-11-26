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
	sessionID := ""
	if claims := auth.UserFromContext(ctx); claims != nil && claims.CredentialKind != "session" {
		return nil, fmt.Errorf("user session required to create API tokens")
	} else if claims != nil {
		sessionID = claims.SessionID
		if sessionID == "" {
			return nil, fmt.Errorf("user session required to create API tokens")
		}
	}
	if err := s.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id=? AND is_active=1`, callerID).Scan(&callerRole); err != nil {
		return nil, fmt.Errorf("resolve token owner: %w", err)
	}
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
	if input.ExpiresAt != nil {
		exp, err := time.Parse(time.RFC3339, *input.ExpiresAt)
		if err != nil || !exp.After(time.Now()) {
			return nil, fmt.Errorf("expiry must be a future RFC3339 timestamp")
		}
	}

	raw, prefix, hash, err := generateAPIToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	scopesJSON, _ := json.Marshal(input.Scopes)

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO api_tokens (name, token_prefix, token_hash, scopes, creator_role, created_by, expires_at)
		SELECT ?, ?, ?, ?, u.role, u.id, ? FROM users u WHERE u.id=? AND u.is_active=1
		 AND (?='' OR EXISTS (SELECT 1 FROM auth_sessions s WHERE s.id=? AND s.user_id=u.id AND s.expires_at>datetime('now')))`,
		input.Name, prefix, hash, string(scopesJSON), input.ExpiresAt, callerID, sessionID, sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert token: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return nil, fmt.Errorf("credential authority changed during token creation")
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
	if err := NewAuditService(s.db).Log(ctx, &callerID, "create", "api_token", &id, map[string]any{"name": input.Name, "scopes": input.Scopes}, security.RequestIP(ctx)); err != nil {
		log.Error().Err(err).Int64("token_id", id).Msg("failed to audit API token creation")
	}
	return tok, nil
}

// CreateForServer generates an agent token tied to a specific server.
// The raw token is returned for embedding in the agent config.
func (s *APITokenService) CreateForServer(ctx context.Context, serverID int64, callerID *int64) (rawToken string, err error) {
	raw, prefix, hash, err := generateAPIToken()
	if err != nil {
		return "", fmt.Errorf("generate agent token: %w", err)
	}
	agentHash, err := security.BcryptHash(raw)
	if err != nil {
		return "", fmt.Errorf("hash agent token: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE api_tokens SET revoked_at=? WHERE server_id=? AND revoked_at IS NULL`, time.Now().UTC().Format(time.RFC3339), serverID); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO api_tokens (name,token_prefix,token_hash,scopes,creator_role,server_id,created_by) VALUES (?,?,?,'["agent"]','admin',?,?)`, fmt.Sprintf("Agent token (server %d)", serverID), prefix, hash, serverID, callerID); err != nil {
		return "", err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE servers SET agent_token_hash=? WHERE id=?`, agentHash, serverID); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
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
	if err == nil {
		if auditErr := NewAuditService(s.db).Log(ctx, &callerID, "revoke", "api_token", &id, nil, security.RequestIP(ctx)); auditErr != nil {
			log.Error().Err(auditErr).Int64("token_id", id).Msg("failed to audit API token revocation")
		}
	}
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
		SELECT t.id, t.token_hash, t.scopes, u.role, u.id,
		       t.expires_at, t.revoked_at
		FROM api_tokens t JOIN users u ON u.id=t.created_by
		WHERE t.token_prefix = ? AND t.revoked_at IS NULL AND u.is_active=1 AND t.server_id IS NULL`,
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
			return nil, fmt.Errorf("scan token: %w", err)
		}

		// Check hash
		if err := security.BcryptCompare(hash, tokenStr); err != nil {
			continue // hash mismatch — try next (shouldn't happen; prefix is unique in practice)
		}

		// Check expiry
		if expiresAt.Valid && expiresAt.String != "" {
			exp, err := time.Parse(time.RFC3339, expiresAt.String)
			if err != nil || !exp.After(time.Now()) {
				return nil, fmt.Errorf("token expired")
			}
		}

		// Check scopes — agent tokens cannot be used for REST API calls.
		// Invalid JSON in the scopes column yields an empty slice, which fails
		// the agent-token check below (safe default).
		var scopes []string
		if err := json.Unmarshal([]byte(scopesRaw), &scopes); err != nil || len(scopes) == 0 {
			return nil, fmt.Errorf("invalid token scopes")
		}
		for _, sc := range scopes {
			if sc != "read" && sc != "write" {
				return nil, fmt.Errorf("invalid token scopes")
			}
		}

		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("close token lookup: %w", err)
		}
		res, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at=datetime('now') WHERE id=? AND revoked_at IS NULL
		 AND EXISTS (SELECT 1 FROM users WHERE users.id=api_tokens.created_by AND is_active=1)`, id)
		if err != nil {
			return nil, fmt.Errorf("record token use: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil || n != 1 {
			return nil, fmt.Errorf("token revoked during authentication")
		}

		userID := int64(0)
		if createdBy.Valid {
			userID = createdBy.Int64
		}

		return &auth.Claims{
			CredentialKind: "api_token", APITokenID: id, Scopes: scopes,
			UserID: userID,
			Email:  "", // not available without a join; handlers that need it can fetch
			Role:   creatorRole,
		}, nil
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read token lookup: %w", err)
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

func (s *APITokenService) Audit(ctx context.Context, actorID int64, action string, id *int64, ip string) {
	NewAuditService(s.db).Log(ctx, &actorID, action, "api_token", id, nil, ip)
}
