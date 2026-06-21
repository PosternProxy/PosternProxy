package services

import (
	"sync"
	"time"

	"github.com/posternproxy/posternproxy/internal/caddy"
)

// Upstream health states. Caddy's /reverse_proxy/upstreams endpoint reports
// only observed request/failure counts, not reachability, so we cannot claim an
// upstream is healthy without a positive signal:
//   - "unhealthy": Caddy has recorded failures (passive health checks tripped).
//   - "healthy":   requests have been served with zero recorded failures.
//   - "unknown":   no traffic observed yet — we have no evidence either way.
//
// Reporting "unknown" instead of "healthy" for idle upstreams is deliberate: a
// dead backend that has never been hit must not show green.
const (
	UpstreamHealthy   = "healthy"
	UpstreamUnhealthy = "unhealthy"
	UpstreamUnknown   = "unknown"
)

// UpstreamHealth is a point-in-time snapshot of one upstream's health.
type UpstreamHealth struct {
	Address     string    `json:"address"`
	Status      string    `json:"status"`
	NumRequests int       `json:"num_requests"`
	Fails       int       `json:"fails"`
	CheckedAt   time.Time `json:"checked_at"`
}

// HealthService polls Caddy's /reverse_proxy/upstreams endpoint on a fixed
// interval and keeps the latest status in memory.  The API handler reads from
// this in-memory cache so each dashboard request does not hit Caddy directly.
type HealthService struct {
	caddyClient *caddy.Client

	mu       sync.RWMutex
	statuses []UpstreamHealth
	lastErr  error
}

func NewHealthService(caddyClient *caddy.Client) *HealthService {
	return &HealthService{caddyClient: caddyClient}
}

// Start launches the background polling loop.  It polls immediately, then
// every interval.  Call this once from main.go after all services are wired up.
func (s *HealthService) Start(interval time.Duration) {
	go func() {
		s.poll()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			s.poll()
		}
	}()
}

// Statuses returns the latest cached upstream statuses.
func (s *HealthService) Statuses() ([]UpstreamHealth, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.lastErr != nil && len(s.statuses) == 0 {
		return nil, s.lastErr
	}
	out := make([]UpstreamHealth, len(s.statuses))
	copy(out, s.statuses)
	return out, nil
}

// poll fetches the upstream list from Caddy and updates the in-memory cache.
func (s *HealthService) poll() {
	raw, err := s.caddyClient.GetUpstreams()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastErr = err
	if err != nil {
		return
	}

	now := time.Now()
	statuses := make([]UpstreamHealth, 0, len(raw))
	for _, u := range raw {
		statuses = append(statuses, UpstreamHealth{
			Address:     u.Address,
			Status:      upstreamStatus(u.NumRequests, u.Fails),
			NumRequests: u.NumRequests,
			Fails:       u.Fails,
			CheckedAt:   now,
		})
	}
	s.statuses = statuses
}

// upstreamStatus maps Caddy's observed counters to a health state. Zero fails
// alone is not evidence of health — it only means nothing has failed, which is
// equally true of an upstream that has never been contacted.
func upstreamStatus(numRequests, fails int) string {
	switch {
	case fails > 0:
		return UpstreamUnhealthy
	case numRequests > 0:
		return UpstreamHealthy
	default:
		return UpstreamUnknown
	}
}
