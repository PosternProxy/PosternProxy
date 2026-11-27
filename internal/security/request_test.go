package security

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestClientIdentity(t *testing.T) {
	trusted, err := ParseTrustedProxies([]string{"10.0.0.0/8", "2001:db8:1::/48"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, peer, xff, want string }{
		{"direct", "192.0.2.1:1000", "198.51.100.1", "192.0.2.1"},
		{"trusted", "10.1.1.1:22", "198.51.100.1", "198.51.100.1"},
		{"chain", "10.1.1.1:22", "198.51.100.1, 10.2.2.2", "198.51.100.1"},
		{"untrusted hop", "10.1.1.1:22", "203.0.113.1, 198.51.100.1", "198.51.100.1"},
		{"malformed", "10.1.1.1:22", "bad, 198.51.100.1", "10.1.1.1"},
		{"empty", "10.1.1.1:22", "", "10.1.1.1"},
		{"ipv6", "[2001:db8:1::1]:1000", "2001:db8:2::1", "2001:db8:2::1"},
		{"mapped", "[::ffff:192.0.2.1]:1000", "198.51.100.1", "192.0.2.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.peer
			r.Header.Set("X-Forwarded-For", tc.xff)
			r.Header.Set("X-Real-IP", "203.0.113.9")
			r.Header.Set("Forwarded", "for=203.0.113.9")
			ClientIdentity(trusted)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.RemoteAddr != tc.want || RequestIP(r.Context()) != tc.want {
					t.Fatalf("got %s", r.RemoteAddr)
				}
			})).ServeHTTP(httptest.NewRecorder(), r)
		})
	}
	for _, bad := range []string{"bad", "192.0.2.1", "::ffff:192.0.2.1/120"} {
		if _, err := ParseTrustedProxies([]string{bad}); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}

func TestUntrustedHeadersAndSourcePortsCannotBypassRateLimit(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	defer rl.Close()
	h := ClientIdentity(nil)(rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })))
	for i, peer := range []string{"192.0.2.1:1000", "192.0.2.1:2000"} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = peer
		r.Header.Set("X-Forwarded-For", []string{"198.51.100.1", "198.51.100.2"}[i])
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, r)
		want := 204
		if i == 1 {
			want = 429
		}
		if rr.Code != want {
			t.Fatalf("request %d = %d", i, rr.Code)
		}
	}
}

func TestRequestLogsOmitCredentialMaterial(t *testing.T) {
	var output bytes.Buffer
	previous := log.Logger
	log.Logger = zerolog.New(&output)
	defer func() { log.Logger = previous }()
	router := chi.NewRouter()
	router.Use(ClientIdentity(nil))
	router.Use(RequestLogger)
	router.Get("/ws/agent", func(w http.ResponseWriter, r *http.Request) { http.Error(w, "unauthorized", http.StatusUnauthorized) })
	for _, path := range []string{"/ws/agent?token=query-secret", "/unknown/path-secret?token=query-secret"} {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer header-secret")
		r.Header.Set("Cookie", "session=cookie-secret")
		router.ServeHTTP(httptest.NewRecorder(), r)
	}
	for _, sentinel := range []string{"query-secret", "header-secret", "cookie-secret", "path-secret"} {
		if strings.Contains(output.String(), sentinel) {
			t.Fatalf("logged %s", sentinel)
		}
	}
	if !strings.Contains(output.String(), "/ws/agent") {
		t.Fatal("missing route attribution")
	}
}
