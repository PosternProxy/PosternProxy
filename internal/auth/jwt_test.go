package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-key-for-unit-tests"

func TestGenerateTokenPair_Success(t *testing.T) {
	pair, err := GenerateTokenPair(testSecret, 42, "alice@example.com", "admin")
	if err != nil {
		t.Fatalf("GenerateTokenPair: %v", err)
	}
	if pair.AccessToken == "" {
		t.Error("access token is empty")
	}
	if pair.RefreshToken == "" {
		t.Error("refresh token is empty")
	}
	if pair.AccessToken == pair.RefreshToken {
		t.Error("access and refresh tokens should differ")
	}
}

func TestValidateToken_Valid(t *testing.T) {
	pair, _ := GenerateTokenPair(testSecret, 7, "bob@example.com", "viewer")

	claims, err := ValidateToken(testSecret, pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.UserID != 7 {
		t.Errorf("UserID = %d, want 7", claims.UserID)
	}
	if claims.Email != "bob@example.com" {
		t.Errorf("Email = %q, want bob@example.com", claims.Email)
	}
	if claims.Role != "viewer" {
		t.Errorf("Role = %q, want viewer", claims.Role)
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	pair, _ := GenerateTokenPair(testSecret, 1, "a@a.com", "admin")

	_, err := ValidateToken("wrong-secret", pair.AccessToken)
	if err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	// Manually create an expired token.
	now := time.Now().Add(-1 * time.Hour)
	claims := &Claims{
		UserID: 1,
		Email:  "expired@test.com",
		Role:   "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(testSecret))

	_, err := ValidateToken(testSecret, tokenStr)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	_, err := ValidateToken(testSecret, "not-a-jwt")
	if err == nil {
		t.Error("expected error for malformed token")
	}
}

func TestValidateToken_EmptyString(t *testing.T) {
	_, err := ValidateToken(testSecret, "")
	if err == nil {
		t.Error("expected error for empty token")
	}
}

func TestValidateToken_RefreshToken(t *testing.T) {
	pair, _ := GenerateTokenPair(testSecret, 5, "r@t.com", "viewer")

	claims, err := ValidateToken(testSecret, pair.RefreshToken)
	if err != nil {
		t.Fatalf("refresh token should be valid: %v", err)
	}
	if claims.UserID != 5 {
		t.Errorf("UserID = %d, want 5", claims.UserID)
	}
}

func TestValidateAccessToken_RejectsRefresh(t *testing.T) {
	pair, _ := GenerateTokenPair(testSecret, 1, "a@a.com", "admin")

	_, err := ValidateAccessToken(testSecret, pair.RefreshToken)
	if err == nil {
		t.Error("ValidateAccessToken should reject a refresh token")
	}
}

func TestValidateRefreshToken_RejectsAccess(t *testing.T) {
	pair, _ := GenerateTokenPair(testSecret, 1, "a@a.com", "admin")

	_, err := ValidateRefreshToken(testSecret, pair.AccessToken)
	if err == nil {
		t.Error("ValidateRefreshToken should reject an access token")
	}
}

func TestValidateAccessToken_AcceptsAccess(t *testing.T) {
	pair, _ := GenerateTokenPair(testSecret, 3, "b@b.com", "viewer")

	claims, err := ValidateAccessToken(testSecret, pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.TokenType != TokenTypeAccess {
		t.Errorf("TokenType = %q, want %q", claims.TokenType, TokenTypeAccess)
	}
}

func TestValidateRefreshToken_AcceptsRefresh(t *testing.T) {
	pair, _ := GenerateTokenPair(testSecret, 3, "b@b.com", "viewer")

	claims, err := ValidateRefreshToken(testSecret, pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if claims.TokenType != TokenTypeRefresh {
		t.Errorf("TokenType = %q, want %q", claims.TokenType, TokenTypeRefresh)
	}
}
