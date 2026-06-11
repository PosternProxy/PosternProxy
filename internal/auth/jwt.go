package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour
)

// Token types distinguish access tokens from refresh tokens.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Claims holds the application-specific fields embedded in a JWT.
type Claims struct {
	UserID    int64  `json:"sub"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwt.RegisteredClaims
}

// TokenPair is the pair of tokens returned after a successful login.
// RefreshToken is omitted from JSON responses and delivered via an HttpOnly cookie.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"-"`
}

// GenerateTokenPair signs a short-lived access token and a long-lived refresh
// token for the given user identity using HMAC-SHA256.
func GenerateTokenPair(secret string, userID int64, email, role string) (*TokenPair, error) {
	accessToken, err := generateToken(secret, userID, email, role, TokenTypeAccess, AccessTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := generateToken(secret, userID, email, role, TokenTypeRefresh, RefreshTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ValidateToken parses and validates a signed JWT string, returning its Claims
// on success or an error if the token is malformed, expired, or uses an
// unexpected signing algorithm.
func ValidateToken(secret, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// ValidateAccessToken validates a JWT and ensures it is an access token.
func ValidateAccessToken(secret, tokenStr string) (*Claims, error) {
	claims, err := ValidateToken(secret, tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, fmt.Errorf("expected access token, got %q", claims.TokenType)
	}
	return claims, nil
}

// ValidateRefreshToken validates a JWT and ensures it is a refresh token.
func ValidateRefreshToken(secret, tokenStr string) (*Claims, error) {
	claims, err := ValidateToken(secret, tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.TokenType != TokenTypeRefresh {
		return nil, fmt.Errorf("expected refresh token, got %q", claims.TokenType)
	}
	return claims, nil
}

func generateToken(secret string, userID int64, email, role, tokenType string, duration time.Duration) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
