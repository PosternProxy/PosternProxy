package handlers

import (
	"database/sql"
	"net/http"

	"github.com/rs/zerolog/log"
)

// DashboardHandler provides HTTP handler methods for dashboard summary data.
type DashboardHandler struct {
	db *sql.DB
}

// NewDashboardHandler creates a DashboardHandler backed by the given database.
func NewDashboardHandler(db *sql.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

type dashboardStats struct {
	ProxyHosts       int          `json:"proxy_hosts"`
	RedirectionHosts int          `json:"redirection_hosts"`
	StreamHosts      int          `json:"stream_hosts"`
	DeadHosts        int          `json:"dead_hosts"`
	Certificates     int          `json:"certificates"`
	AccessLists      int          `json:"access_lists"`
	EnabledHosts     int          `json:"enabled_hosts"`
	ExpiringSoon     []certExpiry `json:"expiring_soon"`
}

type certExpiry struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
	DaysLeft  int    `json:"days_left"`
}

// Stats writes aggregate host and certificate counts, plus certificates expiring within 30 days.
func (h *DashboardHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats := dashboardStats{}

	counts := []struct {
		table  string
		target *int
	}{
		{"proxy_hosts", &stats.ProxyHosts},
		{"redirection_hosts", &stats.RedirectionHosts},
		{"stream_hosts", &stats.StreamHosts},
		{"dead_hosts", &stats.DeadHosts},
		{"certificates", &stats.Certificates},
		{"access_lists", &stats.AccessLists},
	}

	// Table names come from the hardcoded `counts` slice above — never from
	// user input — so string concatenation is safe here.
	for _, c := range counts {
		if err := h.db.QueryRow("SELECT COUNT(*) FROM " + c.table).Scan(c.target); err != nil {
			log.Error().Err(err).Str("table", c.table).Msg("dashboard: count query failed")
			respondError(w, http.StatusInternalServerError, "failed to load dashboard")
			return
		}
	}

	// Total enabled hosts across all host tables
	if err := h.db.QueryRow(`
		SELECT
		  (SELECT COUNT(*) FROM proxy_hosts WHERE enabled=1) +
		  (SELECT COUNT(*) FROM redirection_hosts WHERE enabled=1) +
		  (SELECT COUNT(*) FROM stream_hosts WHERE enabled=1) +
		  (SELECT COUNT(*) FROM dead_hosts WHERE enabled=1)
	`).Scan(&stats.EnabledHosts); err != nil {
		log.Error().Err(err).Msg("dashboard: enabled-hosts query failed")
		respondError(w, http.StatusInternalServerError, "failed to load dashboard")
		return
	}

	// Certificates expiring within 30 days
	rows, err := h.db.Query(`
		SELECT id, name, expires_at,
		       CAST((julianday(expires_at) - julianday('now')) AS INTEGER) AS days_left
		FROM certificates
		WHERE expires_at IS NOT NULL
		  AND julianday(expires_at) - julianday('now') <= 30
		  AND julianday(expires_at) > julianday('now')
		ORDER BY expires_at ASC
		LIMIT 10`)
	if err != nil {
		log.Error().Err(err).Msg("dashboard: expiring-certs query failed")
		respondError(w, http.StatusInternalServerError, "failed to load dashboard")
		return
	}
	defer rows.Close()
	for rows.Next() {
		var e certExpiry
		if err := rows.Scan(&e.ID, &e.Name, &e.ExpiresAt, &e.DaysLeft); err != nil {
			log.Error().Err(err).Msg("dashboard: expiring-certs scan failed")
			respondError(w, http.StatusInternalServerError, "failed to load dashboard")
			return
		}
		stats.ExpiringSoon = append(stats.ExpiringSoon, e)
	}
	if err := rows.Err(); err != nil {
		log.Error().Err(err).Msg("dashboard: expiring-certs iteration failed")
		respondError(w, http.StatusInternalServerError, "failed to load dashboard")
		return
	}
	if stats.ExpiringSoon == nil {
		stats.ExpiringSoon = []certExpiry{}
	}

	respondJSON(w, http.StatusOK, stats)
}
