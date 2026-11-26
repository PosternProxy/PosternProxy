package security

import (
	"net/http"
	"net/netip"
	"sync"
	"time"
)

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
	limit   int
	window  time.Duration
	stop    chan struct{}
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		entries: make(map[string]*rateLimitEntry),
		limit:   limit,
		window:  window,
		stop:    make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

// Close stops the background cleanup goroutine.
func (rl *RateLimiter) Close() {
	close(rl.stop)
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := realIP(r)
		if !rl.allow(ip) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.entries[key]
	if !exists || now.After(entry.resetAt) {
		rl.entries[key] = &rateLimitEntry{count: 1, resetAt: now.Add(rl.window)}
		return true
	}

	entry.count++
	return entry.count <= rl.limit
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, entry := range rl.entries {
				if now.After(entry.resetAt) {
					delete(rl.entries, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stop:
			return
		}
	}
}

// realIP returns the rate-limiting key for a request. It relies on
// chi's RealIP middleware having already normalised r.RemoteAddr.
func realIP(r *http.Request) string {
	if peer, err := netip.ParseAddrPort(r.RemoteAddr); err == nil {
		return peer.Addr().Unmap().WithZone("").String()
	}
	if ip, err := netip.ParseAddr(r.RemoteAddr); err == nil {
		return ip.Unmap().WithZone("").String()
	}
	return r.RemoteAddr
}
