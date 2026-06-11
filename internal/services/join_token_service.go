package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/posternproxy/posternproxy/internal/security"
)

var (
	// ErrServerNotFound is returned by JoinTokenService.Issue when serverID
	// doesn't refer to an existing row.
	ErrServerNotFound = errors.New("server not found")

	// ErrServerAlreadyJoined is returned by JoinTokenService.Issue when the
	// target server already has an agent token. Callers must call
	// ServerService.ResetJoin first to invalidate the existing agent.
	ErrServerAlreadyJoined = errors.New("server already joined")

	// ErrInvalidToken is returned by JoinTokenService.Consume when the raw
	// token doesn't match any outstanding row.
	ErrInvalidToken = errors.New("invalid join token")

	// ErrTokenExpired is returned by JoinTokenService.Consume when the matching
	// row's expires_at is in the past.
	ErrTokenExpired = errors.New("join token expired")

	// ErrTokenAlreadyUsed is returned by JoinTokenService.Consume when the
	// matching row has already been consumed.
	ErrTokenAlreadyUsed = errors.New("join token already used")
)

const (
	defaultJoinTokenTTL = 15 * time.Minute
	minJoinTokenTTL     = 1 * time.Minute
	maxJoinTokenTTL     = 1 * time.Hour

	// joinTokenRetention controls how long expired-and-unconsumed rows survive
	// before PurgeExpired deletes them. Keeping them for a week leaves a paper
	// trail for audit/debugging without bloating the table.
	joinTokenRetention = 7 * 24 * time.Hour
)

// JoinTokenService issues, consumes, and purges single-use join tokens that
// allow an agent to exchange a short-lived enrollment secret for a long-lived
// bearer token on first connect.
type JoinTokenService struct {
	db *sql.DB
}

// NewJoinTokenService creates a JoinTokenService backed by db.
func NewJoinTokenService(db *sql.DB) *JoinTokenService {
	return &JoinTokenService{db: db}
}

// Issue creates a new join token for the given server. The raw token is
// returned exactly once — only its bcrypt hash is persisted. ttl is clamped
// to [1 minute, 1 hour]; a zero ttl uses the 15-minute default.
//
// Returns ErrServerNotFound if serverID does not exist.
// Returns ErrServerAlreadyJoined if the server already has an agent_token_hash.
func (s *JoinTokenService) Issue(ctx context.Context, callerID *int64, serverID int64, ttl time.Duration) (rawToken string, expiresAt time.Time, err error) {
	var agentTokenHash string
	err = s.db.QueryRowContext(ctx, `SELECT agent_token_hash FROM servers WHERE id = ?`, serverID).Scan(&agentTokenHash)
	if err == sql.ErrNoRows {
		return "", time.Time{}, ErrServerNotFound
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("lookup server: %w", err)
	}
	if agentTokenHash != "" {
		return "", time.Time{}, ErrServerAlreadyJoined
	}

	if ttl <= 0 {
		ttl = defaultJoinTokenTTL
	}
	if ttl < minJoinTokenTTL {
		ttl = minJoinTokenTTL
	}
	if ttl > maxJoinTokenTTL {
		ttl = maxJoinTokenTTL
	}

	raw, hash, err := generateJoinToken()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("generate token: %w", err)
	}

	expiresAt = time.Now().UTC().Add(ttl)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO join_tokens (token_hash, server_id, created_by, expires_at)
		VALUES (?, ?, ?, ?)`,
		hash, serverID, callerID, expiresAt.Format(time.RFC3339),
	)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("insert token: %w", err)
	}
	return raw, expiresAt, nil
}

// Consume validates the raw token and atomically marks it consumed. On success
// returns the associated server ID. Called by the agent websocket handshake
// (Task 5), not by the HTTP API.
//
// Returns ErrInvalidToken if no unconsumed row matches.
// Returns ErrTokenExpired if the matching row is expired.
// Returns ErrTokenAlreadyUsed if the matching row was already consumed.
func (s *JoinTokenService) Consume(ctx context.Context, rawToken, remoteAddr string) (int64, error) {
	if rawToken == "" {
		return 0, ErrInvalidToken
	}

	// We can't index by hash (bcrypt salt randomises the hash on each call),
	// so we scan candidates and bcrypt-compare. The set is bounded — only
	// outstanding tokens within the 7-day retention window — so a sequential
	// scan is fine. Drain into memory before bcrypt+update so we don't hold
	// the only sqlite connection open while doing CPU work or a follow-up
	// write (database/sql is configured with MaxOpenConns=1).
	type candidate struct {
		id         int64
		hash       string
		serverID   int64
		expiresStr string
		consumedAt sql.NullString
	}
	var candidates []candidate
	{
		rows, err := s.db.QueryContext(ctx, `
			SELECT id, token_hash, server_id, expires_at, consumed_at
			FROM join_tokens`)
		if err != nil {
			return 0, fmt.Errorf("scan tokens: %w", err)
		}
		for rows.Next() {
			var c candidate
			if err := rows.Scan(&c.id, &c.hash, &c.serverID, &c.expiresStr, &c.consumedAt); err != nil {
				rows.Close()
				return 0, fmt.Errorf("scan: %w", err)
			}
			candidates = append(candidates, c)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return 0, err
		}
		rows.Close()
	}

	now := time.Now().UTC()
	for _, c := range candidates {
		if security.BcryptCompare(c.hash, rawToken) != nil {
			continue
		}
		if c.consumedAt.Valid {
			return 0, ErrTokenAlreadyUsed
		}
		expires, err := time.Parse(time.RFC3339, c.expiresStr)
		if err == nil && now.After(expires) {
			return 0, ErrTokenExpired
		}

		// Atomic single-use guard: only mark consumed if not already consumed.
		// If two requests race we want exactly one to succeed.
		res, err := s.db.ExecContext(ctx, `
			UPDATE join_tokens
			   SET consumed_at = ?, consumed_from = ?
			 WHERE id = ? AND consumed_at IS NULL`,
			now.Format(time.RFC3339), remoteAddr, c.id,
		)
		if err != nil {
			return 0, fmt.Errorf("mark consumed: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return 0, ErrTokenAlreadyUsed
		}
		return c.serverID, nil
	}
	return 0, ErrInvalidToken
}

// RevokeForServer deletes all outstanding (unconsumed) join tokens for the
// given server. Returns the number of rows deleted.
func (s *JoinTokenService) RevokeForServer(ctx context.Context, serverID int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM join_tokens
		 WHERE server_id = ? AND consumed_at IS NULL`,
		serverID,
	)
	if err != nil {
		return 0, fmt.Errorf("revoke join tokens: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// PurgeExpired deletes expired-and-unconsumed tokens older than the retention
// window, plus consumed tokens older than the retention window.
func (s *JoinTokenService) PurgeExpired(ctx context.Context) error {
	cutoff := time.Now().UTC().Add(-joinTokenRetention).Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM join_tokens
		 WHERE (consumed_at IS NULL AND expires_at < ?)
		    OR (consumed_at IS NOT NULL AND consumed_at < ?)`,
		cutoff, cutoff,
	)
	if err != nil {
		return fmt.Errorf("purge join tokens: %w", err)
	}
	return nil
}

// generateJoinToken returns a 32-byte cryptographically random token encoded
// as unpadded base64url (~43 chars) along with its bcrypt hash.
func generateJoinToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	hash, err = security.BcryptHash(raw)
	return
}
