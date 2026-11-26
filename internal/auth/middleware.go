package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"
)

type contextKey string

const userContextKey contextKey = "user"

// TokenResolver can validate non-JWT bearer tokens (e.g. API tokens) and
// return synthesized Claims. Return nil, nil when the token format is not
// recognised so the caller can try the next resolver or reject the request.
type TokenResolver interface {
	Resolve(ctx context.Context, token string) (*Claims, error)
}

type SessionValidator interface {
	ValidateSession(context.Context, *Claims) error
}

// Middleware validates requests using a JWT secret. If a TokenResolver is
// provided, tokens that fail JWT parsing are tried against it as a fallback
// (e.g. long-lived API tokens with the "pp_tok_" prefix).
func Middleware(secret string, resolver TokenResolver, validators ...SessionValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearerToken(r)
			if tokenStr == "" {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// 1. Try JWT first (must be an access token).
			claims, err := ValidateAccessToken(secret, tokenStr)
			if err == nil {
				claims.CredentialKind = "session"
				for _, validator := range validators {
					if validator == nil || validator.ValidateSession(r.Context(), claims) != nil {
						http.Error(w, `{"error":"invalid or revoked session"}`, http.StatusUnauthorized)
						return
					}
				}
				ctx := context.WithValue(r.Context(), userContextKey, claims)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// 2. Fall back to API token resolver (if wired up).
			if resolver != nil {
				claims, err = resolver.Resolve(r.Context(), tokenStr)
				if err == nil && claims != nil {
					ctx := context.WithValue(r.Context(), userContextKey, claims)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				if err != nil {
					// Recognisable token format but invalid — return specific error.
					http.Error(w, `{"error":"invalid or revoked token"}`, http.StatusUnauthorized)
					return
				}
			}

			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
		})
	}
}

// RequireAdmin rejects requests from authenticated users who do not have the
// "admin" role.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := UserFromContext(r.Context())
		if claims == nil || claims.Role != "admin" {
			http.Error(w, `{"error":"admin access required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireScope rejects requests whose API token lacks the given scope.
// JWT sessions always pass (they are not scope-limited).
func RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := UserFromContext(r.Context())
			if claims == nil || (claims.CredentialKind != "session" &&
				(claims.CredentialKind != "api_token" || !slices.Contains(claims.Scopes, scope))) {
				http.Error(w, `{"error":"insufficient token scope"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireManagementScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope := "write"
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			scope = "read"
		}
		RequireScope(scope)(next).ServeHTTP(w, r)
	})
}

func RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := UserFromContext(r.Context())
		if claims == nil || claims.CredentialKind != "session" {
			http.Error(w, `{"error":"user session required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// UserFromContext retrieves the authenticated user's Claims from the request
// context, or returns nil when no authenticated user is present.
func UserFromContext(ctx context.Context) *Claims {
	claims, _ := ctx.Value(userContextKey).(*Claims)
	return claims
}

func extractBearerToken(r *http.Request) string {
	a := r.Header.Get("Authorization")
	if strings.HasPrefix(a, "Bearer ") {
		return a[7:]
	}
	return ""
}
