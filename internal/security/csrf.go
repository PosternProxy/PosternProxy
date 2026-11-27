package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
)

// SeedCSRFCookie returns middleware that sets the csrf_token cookie on
// safe-method requests that lack one. Apply it to routes exempt from
// CSRFProtection (e.g. /auth) so that a POST immediately after page load
// already has a cookie to submit.
func SeedCSRFCookie(secureCookies bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				ensureCSRFCookie(w, r, secureCookies)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CSRFProtection returns middleware that validates the X-CSRF-Token header
// against the csrf_token cookie on unsafe-method requests (double-submit
// cookie pattern). It also seeds the cookie on safe methods.
func CSRFProtection(secureCookies bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				ensureCSRFCookie(w, r, secureCookies)
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(csrfCookieName)
			if err != nil {
				http.Error(w, `{"error":"missing CSRF token"}`, http.StatusForbidden)
				return
			}

			header := r.Header.Get(csrfHeaderName)
			if header == "" || subtle.ConstantTimeCompare([]byte(header), []byte(cookie.Value)) != 1 {
				http.Error(w, `{"error":"invalid CSRF token"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ensureCSRFCookie(w http.ResponseWriter, r *http.Request, secure bool) {
	if _, err := r.Cookie(csrfCookieName); err == nil {
		return
	}
	token := generateCSRFToken()
	http.SetCookie(w, &http.Cookie{ // #nosec G124 -- double-submit CSRF cookie must be JS-readable (HttpOnly:false); Secure is config-driven
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
