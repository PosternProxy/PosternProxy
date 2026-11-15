package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/posternproxy/posternproxy/internal/auth"
	"github.com/posternproxy/posternproxy/internal/models"
)

type SessionService struct {
	db     *sql.DB
	secret string
}

func NewSessionService(db *sql.DB, secret string) *SessionService {
	return &SessionService{db: db, secret: secret}
}

func (s *SessionService) Issue(ctx context.Context, user *models.User) (*auth.TokenPair, error) {
	var id [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, fmt.Errorf("generate session: %w", err)
	}
	sessionID := hex.EncodeToString(id[:])
	pair, err := auth.GenerateSessionTokenPair(s.secret, user.ID, user.Email, user.Role, sessionID)
	if err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO auth_sessions (id, user_id, refresh_hash, expires_at)
	 SELECT ?, id, ?, datetime('now', '+7 days') FROM users WHERE id=? AND is_active=1 AND password_hash=? AND role=?`,
		sessionID, sessionHash(pair.RefreshToken), user.ID, user.PasswordHash, user.Role)
	if err != nil {
		return nil, fmt.Errorf("persist session: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return nil, fmt.Errorf("account changed during login")
	}
	return pair, nil
}

func (s *SessionService) ValidateSession(ctx context.Context, claims *auth.Claims) error {
	if claims.SessionID == "" {
		return fmt.Errorf("session required")
	}
	err := s.db.QueryRowContext(ctx, `SELECT u.role, u.email FROM auth_sessions s JOIN users u ON u.id=s.user_id
	 WHERE s.id=? AND s.user_id=? AND s.expires_at>datetime('now') AND u.is_active=1`, claims.SessionID, claims.UserID).Scan(&claims.Role, &claims.Email)
	if err != nil {
		return fmt.Errorf("resolve session: %w", err)
	}
	return nil
}

func (s *SessionService) Refresh(ctx context.Context, raw string) (*auth.TokenPair, error) {
	claims, err := auth.ValidateRefreshToken(s.secret, raw)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}
	if err := s.ValidateSession(ctx, claims); err != nil {
		return nil, err
	}
	var expected string
	if err := s.db.QueryRowContext(ctx, `SELECT refresh_hash FROM auth_sessions WHERE id=?`, claims.SessionID).Scan(&expected); err != nil {
		return nil, fmt.Errorf("read refresh identity: %w", err)
	}
	actual := sessionHash(raw)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) != 1 {
		return nil, fmt.Errorf("refresh token already used")
	}
	pair, err := auth.GenerateSessionTokenPair(s.secret, claims.UserID, claims.Email, claims.Role, claims.SessionID)
	if err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE auth_sessions SET refresh_hash=?, expires_at=datetime('now', '+7 days')
	 WHERE id=? AND refresh_hash=? AND expires_at>datetime('now')`, sessionHash(pair.RefreshToken), claims.SessionID, expected)
	if err != nil {
		return nil, fmt.Errorf("rotate refresh token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		return nil, fmt.Errorf("session revoked or refresh token already used")
	}
	return pair, nil
}

func (s *SessionService) Revoke(ctx context.Context, claims *auth.Claims) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE id=? AND user_id=?`, claims.SessionID, claims.UserID)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func sessionHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
