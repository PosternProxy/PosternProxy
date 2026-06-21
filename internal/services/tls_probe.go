package services

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// TLS probe states for a host that is expected to serve HTTPS.
//
//	"valid"   — Caddy presents a non-expired certificate that matches the domain.
//	"pending" — :443 answers but the presented cert doesn't match yet (ACME in
//	            progress, DNS not pointed, or a placeholder/default cert).
//	"failed"  — no HTTPS response, or the cert is expired.
const (
	TLSProbeValid   = "valid"
	TLSProbePending = "pending"
	TLSProbeFailed  = "failed"
)

// TLSProbeStatus is the result of one probe against the local Caddy HTTPS
// listener for a single proxy host's primary domain.
type TLSProbeStatus struct {
	HostID    int64      `json:"host_id"`
	Domain    string     `json:"domain"`
	Status    string     `json:"status"`
	Detail    string     `json:"detail,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CheckedAt time.Time  `json:"checked_at"`
}

// TLSProbeService periodically completes a TLS handshake against the local Caddy
// HTTPS listener (SNI = each host's domain) and records whether Caddy is serving
// a usable certificate. This is the ground-truth signal for "is HTTPS working for
// this host", independent of Caddy's internal ACME state — it self-heals to
// "valid" the moment a certificate is obtained.
//
// Only local (server_id=1) hosts that are expected to serve HTTPS are probed:
// SSL-forced or certificate-assigned, non-passthrough, non-HTTP-only.
type TLSProbeService struct {
	db   *sql.DB
	addr string // local Caddy HTTPS listener, e.g. "127.0.0.1:443"

	mu       sync.RWMutex
	statuses map[int64]TLSProbeStatus
}

func NewTLSProbeService(db *sql.DB, addr string) *TLSProbeService {
	if addr == "" {
		addr = "127.0.0.1:443"
	}
	return &TLSProbeService{db: db, addr: addr, statuses: make(map[int64]TLSProbeStatus)}
}

// Start launches the background polling loop: probe immediately, then every interval.
func (s *TLSProbeService) Start(interval time.Duration) {
	go func() {
		s.poll()
		t := time.NewTicker(interval)
		defer t.Stop()
		for range t.C {
			s.poll()
		}
	}()
}

// Statuses returns the latest cached probe results.
func (s *TLSProbeService) Statuses() []TLSProbeStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TLSProbeStatus, 0, len(s.statuses))
	for _, st := range s.statuses {
		out = append(out, st)
	}
	return out
}

func (s *TLSProbeService) poll() {
	hosts, err := s.httpsHosts()
	if err != nil {
		log.Warn().Err(err).Msg("tls probe: failed to load hosts")
		return
	}

	now := time.Now()
	next := make(map[int64]TLSProbeStatus, len(hosts))
	for id, domain := range hosts {
		st := s.probe(domain)
		st.HostID = id
		st.Domain = domain
		st.CheckedAt = now
		next[id] = st
	}

	s.mu.Lock()
	s.statuses = next
	s.mu.Unlock()
}

// httpsHosts returns the primary domain of each local host expected to serve HTTPS,
// keyed by host id.
func (s *TLSProbeService) httpsHosts() (map[int64]string, error) {
	rows, err := s.db.Query(`
		SELECT id, domain_names FROM proxy_hosts
		WHERE enabled = 1 AND server_id = 1
		  AND tls_passthrough = 0 AND http_only = 0
		  AND (ssl_forced = 1 OR certificate_id IS NOT NULL)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]string)
	for rows.Next() {
		var id int64
		var domainsJSON string
		if err := rows.Scan(&id, &domainsJSON); err != nil {
			return nil, err
		}
		var domains []string
		if err := json.Unmarshal([]byte(domainsJSON), &domains); err != nil || len(domains) == 0 {
			continue
		}
		out[id] = domains[0]
	}
	return out, rows.Err()
}

// probe completes a TLS handshake against the local Caddy listener with the given
// SNI and evaluates the presented leaf certificate. The chain is intentionally not
// verified (InsecureSkipVerify) so internal/self-signed certs the operator chose
// are still reported as serving; we check expiry and hostname match ourselves.
func (s *TLSProbeService) probe(domain string) TLSProbeStatus {
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 5 * time.Second},
		"tcp", s.addr,
		&tls.Config{ServerName: domain, InsecureSkipVerify: true},
	)
	if err != nil {
		return TLSProbeStatus{Status: TLSProbeFailed, Detail: "no HTTPS response from Caddy"}
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return TLSProbeStatus{Status: TLSProbeFailed, Detail: "no certificate presented"}
	}

	leaf := certs[0]
	exp := leaf.NotAfter
	st := TLSProbeStatus{ExpiresAt: &exp}
	switch {
	case time.Now().After(leaf.NotAfter):
		st.Status = TLSProbeFailed
		st.Detail = "certificate expired"
	case leaf.VerifyHostname(domain) != nil:
		st.Status = TLSProbePending
		st.Detail = "no matching certificate yet — provisioning, or DNS not pointed"
	default:
		st.Status = TLSProbeValid
	}
	return st
}
