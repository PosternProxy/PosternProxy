package services

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Probe states describe observed TLS readiness. A mismatch uses "pending" for
// API compatibility; it does not establish that an ACME order is in progress.
const (
	TLSProbeValid   = "valid"
	TLSProbePending = "pending"
	TLSProbeFailed  = "failed"
)

// TLSProbeStatus summarizes the local Caddy TLS checks for one managed host.
type TLSProbeStatus struct {
	ResourceType string           `json:"resource_type"`
	ServerID     int64            `json:"server_id"`
	Domains      []TLSProbeDomain `json:"domains"`
	HostID       int64            `json:"host_id"`
	Domain       string           `json:"domain"`
	Status       string           `json:"status"`
	Detail       string           `json:"detail,omitempty"`
	ExpiresAt    *time.Time       `json:"expires_at,omitempty"`
	CheckedAt    time.Time        `json:"checked_at"`
}

// TLSProbeService checks serving certificates for local proxy, redirect and
// parked hosts. Its results describe TLS readiness, not ACME order state.
type TLSProbeService struct {
	db   *sql.DB
	addr string // local Caddy HTTPS listener, e.g. "127.0.0.1:443"

	mu       sync.RWMutex
	statuses map[string]TLSProbeStatus
}

func NewTLSProbeService(db *sql.DB, addr string) *TLSProbeService {
	if addr == "" {
		addr = "127.0.0.1:443"
	}
	return &TLSProbeService{db: db, addr: addr, statuses: make(map[string]TLSProbeStatus)}
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

type TLSProbeDomain struct {
	Domain    string     `json:"domain"`
	Status    string     `json:"status"`
	Detail    string     `json:"detail,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type tlsTarget struct {
	ID           int64
	ResourceType string
	Domains      []string
}

func (s *TLSProbeService) poll() {
	hosts, err := s.httpsTargets()
	if err != nil {
		log.Warn().Err(err).Msg("tls probe: failed to load hosts")
		return
	}
	next := make(map[string]TLSProbeStatus, len(hosts))
	var mu sync.Mutex
	var workers sync.WaitGroup
	slots := make(chan struct{}, 8)
	for _, host := range hosts {
		slots <- struct{}{}
		workers.Add(1)
		go func(host tlsTarget) {
			defer workers.Done()
			defer func() { <-slots }()
			status := TLSProbeStatus{HostID: host.ID, ResourceType: host.ResourceType, ServerID: 1, Domain: host.Domains[0], Status: TLSProbeValid, CheckedAt: time.Now()}
			for _, domain := range host.Domains {
				name := domain
				if strings.HasPrefix(name, "*.") {
					name = "tls-check." + strings.TrimPrefix(name, "*.")
				}
				result := s.probe(name)
				status.Domains = append(status.Domains, TLSProbeDomain{Domain: domain, Status: result.Status, Detail: result.Detail, ExpiresAt: result.ExpiresAt})
				if result.ExpiresAt != nil && (status.ExpiresAt == nil || result.ExpiresAt.Before(*status.ExpiresAt)) {
					status.ExpiresAt = result.ExpiresAt
				}
				if result.Status == TLSProbeFailed || (result.Status == TLSProbePending && status.Status == TLSProbeValid) {
					status.Status = result.Status
					status.Detail = domain + ": " + result.Detail
				}
			}
			mu.Lock()
			next[fmt.Sprintf("%s:%d", host.ResourceType, host.ID)] = status
			mu.Unlock()
		}(host)
	}
	workers.Wait()
	s.mu.Lock()
	s.statuses = next
	s.mu.Unlock()
}

func (s *TLSProbeService) httpsTargets() ([]tlsTarget, error) {
	rows, err := s.db.Query(`
 SELECT 'proxy_host',id,domain_names FROM proxy_hosts
 WHERE enabled=1 AND server_id=1 AND tls_passthrough=0 AND http_only=0 AND (ssl_forced=1 OR certificate_id IS NOT NULL)
 UNION ALL
 SELECT 'redirection_host',id,domain_names FROM redirection_hosts
 WHERE enabled=1 AND server_id=1 AND (ssl_forced=1 OR certificate_id IS NOT NULL)
 UNION ALL
 SELECT 'dead_host',id,domain_names FROM dead_hosts
 WHERE enabled=1 AND server_id=1 AND http_only=0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var targets []tlsTarget
	for rows.Next() {
		var target tlsTarget
		var raw string
		if err := rows.Scan(&target.ResourceType, &target.ID, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &target.Domains); err != nil || len(target.Domains) == 0 {
			continue
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
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
		return TLSProbeStatus{Status: TLSProbeFailed, Detail: "TLS handshake unavailable; certificate issuance may still be pending. Check Caddy logs for the exact issuer error."}
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
	case time.Now().Before(leaf.NotBefore):
		st.Status = TLSProbeFailed
		st.Detail = "certificate is not valid yet"
	case time.Now().After(leaf.NotAfter):
		st.Status = TLSProbeFailed
		st.Detail = "certificate expired"
	case leaf.VerifyHostname(domain) != nil:
		st.Status = TLSProbePending
		st.Detail = "presented certificate does not match this domain"
	default:
		st.Status = TLSProbeValid
	}
	return st
}
