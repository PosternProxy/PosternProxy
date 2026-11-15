package handlers

import (
	"net/http"
	"time"

	"github.com/posternproxy/posternproxy/internal/auth"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

// dummyPasswordHash is a pre-computed bcrypt hash used when the requested user
// does not exist, ensuring login always takes constant time.
var dummyPasswordHash, _ = auth.HashPassword("posternproxy-dummy-password")

// AuthHandler provides HTTP handler methods for authentication.
type AuthHandler struct {
	users         *services.UserService
	audit         *services.AuditService
	jwtSecret     string
	secureCookies bool
	sessions      *services.SessionService
}

// NewAuthHandler creates an AuthHandler backed by the given services and JWT secret.
func NewAuthHandler(users *services.UserService, audit *services.AuditService, jwtSecret string, secureCookies bool, sessions *services.SessionService) *AuthHandler {
	return &AuthHandler{users: users, audit: audit, jwtSecret: jwtSecret, secureCookies: secureCookies, sessions: sessions}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string `json:"access_token"`
	// User is typed concretely (rather than any) so that the compiler enforces
	// the json:"-" tag on User.PasswordHash — a future field rename or struct
	// swap cannot accidentally leak the hash through this response.
	User *models.User `json:"user"`
}

// Login authenticates a user with email and password and issues a token pair.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if user == nil {
		// Run a bcrypt comparison against a dummy hash so the response time
		// is indistinguishable from a real user with a wrong password.
		auth.CheckPassword(dummyPasswordHash, req.Password)
		h.audit.Log(r.Context(), nil, "failed_login", "auth", nil, map[string]string{"email": req.Email}, r.RemoteAddr)
		respondError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		h.audit.Log(r.Context(), nil, "failed_login", "auth", nil, map[string]string{"email": req.Email}, r.RemoteAddr)
		respondError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if !user.IsActive {
		respondError(w, http.StatusForbidden, "account is disabled")
		return
	}

	tokens, err := h.sessions.Issue(r.Context(), user)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    tokens.RefreshToken,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(auth.RefreshTokenDuration / time.Second),
	})

	userID := user.ID
	h.audit.Log(r.Context(), &userID, "login", "auth", nil, nil, r.RemoteAddr)

	respondJSON(w, http.StatusCreated, loginResponse{
		AccessToken: tokens.AccessToken,
		User:        user,
	})
}

// Refresh validates the refresh_token cookie and issues a new token pair.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		respondError(w, http.StatusUnauthorized, "no refresh token")
		return
	}

	tokens, err := h.sessions.Refresh(r.Context(), cookie.Value)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid or revoked refresh token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    tokens.RefreshToken,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(auth.RefreshTokenDuration / time.Second),
	})

	respondJSON(w, http.StatusCreated, map[string]string{"access_token": tokens.AccessToken})
}

// Logout clears the refresh_token cookie and ends the session.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	claims := auth.UserFromContext(r.Context())
	if claims == nil || h.sessions.Revoke(r.Context(), claims) != nil {
		respondError(w, http.StatusInternalServerError, "failed to revoke session")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	respondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// Me writes the authenticated user's profile to the response.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := auth.UserFromContext(r.Context())
	if claims == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil || user == nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	respondJSON(w, http.StatusOK, user)
}
