-- PosternProxy initial schema (pre-release consolidation of migrations 001-019).
-- All tables, indexes, triggers, and bootstrap rows live here.

-- ── Users ─────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT    NOT NULL UNIQUE,
    name          TEXT    NOT NULL,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'viewer' CHECK (role IN ('admin', 'viewer')),
    is_active     INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── Servers (multi-server controller + remote agents) ─────────────────────────

CREATE TABLE IF NOT EXISTS servers (
    id                        INTEGER PRIMARY KEY AUTOINCREMENT,
    name                      TEXT    NOT NULL,
    hostname                  TEXT    NOT NULL,
    ssh_port                  INTEGER NOT NULL DEFAULT 22,
    ssh_user                  TEXT    NOT NULL DEFAULT 'root',
    ssh_auth_method           TEXT    NOT NULL DEFAULT 'password' CHECK (ssh_auth_method IN ('password', 'key')),
    ssh_credentials_encrypted TEXT    NOT NULL DEFAULT '',
    ssh_host_key              TEXT    NOT NULL DEFAULT '',   -- SHA256 fingerprint, TOFU
    agent_token_hash          TEXT    NOT NULL DEFAULT '',
    status                    TEXT    NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'provisioning', 'online', 'offline', 'error')),
    provision_log             TEXT    NOT NULL DEFAULT '',
    deployment_type           TEXT    NOT NULL DEFAULT 'systemd',  -- "systemd" | "docker"
    http2_enabled             INTEGER NOT NULL DEFAULT 1,  -- server-wide HTTP/2 toggle
    is_local                  INTEGER NOT NULL DEFAULT 0,
    -- Heartbeat / health fields populated by the agent's status payload.
    caddy_version             TEXT    NOT NULL DEFAULT '',
    uptime_seconds            INTEGER NOT NULL DEFAULT 0,
    active_routes             INTEGER NOT NULL DEFAULT 0,
    last_status_at            TEXT,
    last_seen_at              TEXT,
    created_at                TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at                TEXT    NOT NULL DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO servers (id, name, hostname, is_local, status)
VALUES (1, 'Local', 'localhost', 1, 'online');

-- ── Certificates ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS certificates (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT    NOT NULL,
    provider      TEXT    NOT NULL DEFAULT 'auto' CHECK (provider IN ('auto', 'custom', 'dns')),
    domain_names  TEXT    NOT NULL DEFAULT '[]',
    cert_path     TEXT,
    key_path      TEXT,
    expires_at    TEXT,
    status        TEXT    NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'expired', 'error')),
    error_message TEXT,
    created_at    TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

-- ── Access lists ──────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS access_lists (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    created_at TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS access_list_clients (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    access_list_id INTEGER NOT NULL REFERENCES access_lists(id) ON DELETE CASCADE,
    address        TEXT    NOT NULL,
    directive      TEXT    NOT NULL CHECK (directive IN ('allow', 'deny')),
    sort_order     INTEGER NOT NULL DEFAULT 0
);

-- ── Proxy hosts ───────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS proxy_hosts (
    id                            INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id                     INTEGER NOT NULL DEFAULT 1 REFERENCES servers(id),
    domain_names                  TEXT    NOT NULL DEFAULT '[]',
    forward_scheme                TEXT    NOT NULL DEFAULT 'http' CHECK (forward_scheme IN ('http', 'https')),
    forward_host                  TEXT    NOT NULL,
    forward_port                  INTEGER NOT NULL,
    certificate_id                INTEGER REFERENCES certificates(id) ON DELETE SET NULL,
    ssl_forced                    INTEGER NOT NULL DEFAULT 0,
    hsts_enabled                  INTEGER NOT NULL DEFAULT 0,
    hsts_subdomains               INTEGER NOT NULL DEFAULT 0,
    http_only                     INTEGER NOT NULL DEFAULT 0,
    tls_passthrough               INTEGER NOT NULL DEFAULT 0,
    access_list_id                INTEGER REFERENCES access_lists(id) ON DELETE SET NULL,
    advanced_config               TEXT    NOT NULL DEFAULT '',
    locations                     TEXT    NOT NULL DEFAULT '[]',
    enabled                       INTEGER NOT NULL DEFAULT 1,
    caddy_id                      TEXT,
    -- Health checks (nullable = "use Caddy defaults / feature off")
    hc_enabled                    INTEGER NOT NULL DEFAULT 0,
    hc_active_path                TEXT,
    hc_active_interval            INTEGER,
    hc_active_timeout             INTEGER,
    hc_active_expect_status       INTEGER,
    hc_passive_fail_duration      INTEGER,
    hc_passive_max_fails          INTEGER,
    hc_passive_unhealthy_latency  INTEGER,
    -- Rate limiting (caddy-ratelimit)
    rate_limit_enabled            INTEGER NOT NULL DEFAULT 0,
    rate_limit_requests           INTEGER NOT NULL DEFAULT 100,
    rate_limit_window             TEXT    NOT NULL DEFAULT '1m',
    -- Custom error pages: JSON map of status-code → HTML body
    error_pages                   TEXT    NOT NULL DEFAULT '{}',
    -- Custom request/response headers: JSON array of rule objects
    custom_headers                TEXT    NOT NULL DEFAULT '[]',
    -- Load balancing extras (primary upstream is forward_host/forward_port)
    extra_upstreams               TEXT    NOT NULL DEFAULT '[]',
    lb_policy                     TEXT    NOT NULL DEFAULT 'random',
    created_at                    TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at                    TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_proxy_hosts_server_id ON proxy_hosts(server_id);

-- ── Redirection hosts ─────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS redirection_hosts (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id           INTEGER NOT NULL DEFAULT 1 REFERENCES servers(id),
    domain_names        TEXT    NOT NULL DEFAULT '[]',
    forward_scheme      TEXT    NOT NULL DEFAULT 'auto' CHECK (forward_scheme IN ('auto', 'http', 'https')),
    forward_domain_name TEXT    NOT NULL,
    forward_path        TEXT    NOT NULL DEFAULT '',
    preserve_path       INTEGER NOT NULL DEFAULT 1,
    status_code         INTEGER NOT NULL DEFAULT 301 CHECK (status_code IN (301, 302, 307, 308)),
    certificate_id      INTEGER REFERENCES certificates(id) ON DELETE SET NULL,
    ssl_forced          INTEGER NOT NULL DEFAULT 0,
    hsts_enabled        INTEGER NOT NULL DEFAULT 0,
    access_list_id      INTEGER REFERENCES access_lists(id) ON DELETE SET NULL,
    advanced_config     TEXT    NOT NULL DEFAULT '',
    enabled             INTEGER NOT NULL DEFAULT 1,
    caddy_id            TEXT,
    created_at          TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_redirection_hosts_server_id ON redirection_hosts(server_id);

-- ── Stream hosts (L4 TCP/UDP) ─────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS stream_hosts (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id          INTEGER NOT NULL DEFAULT 1 REFERENCES servers(id),
    incoming_port      INTEGER NOT NULL,
    forward_host       TEXT    NOT NULL,
    forward_port       INTEGER NOT NULL,
    protocol           TEXT    NOT NULL DEFAULT 'tcp' CHECK (protocol IN ('tcp', 'udp', 'tcp+udp')),
    tcp_sni_match      TEXT,
    tcp_proxy_protocol INTEGER NOT NULL DEFAULT 0,
    udp_timeout        INTEGER NOT NULL DEFAULT 30,
    enabled            INTEGER NOT NULL DEFAULT 1,
    caddy_id           TEXT,
    created_at         TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_stream_hosts_server_id ON stream_hosts(server_id);

-- ── Dead hosts ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS dead_hosts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id       INTEGER NOT NULL DEFAULT 1 REFERENCES servers(id),
    domain_names    TEXT    NOT NULL DEFAULT '[]',
    certificate_id  INTEGER REFERENCES certificates(id) ON DELETE SET NULL,
    ssl_forced      INTEGER NOT NULL DEFAULT 0,
    access_list_id  INTEGER REFERENCES access_lists(id) ON DELETE SET NULL,
    advanced_config TEXT    NOT NULL DEFAULT '',
    status_code     INTEGER NOT NULL DEFAULT 404,
    custom_body     TEXT    NOT NULL DEFAULT '',
    enabled         INTEGER NOT NULL DEFAULT 1,
    caddy_id        TEXT,
    created_at      TEXT    NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dead_hosts_server_id ON dead_hosts(server_id);

-- ── Audit log ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS audit_log (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action        TEXT    NOT NULL,
    resource_type TEXT    NOT NULL,
    resource_id   INTEGER,
    details       TEXT    NOT NULL DEFAULT '{}',
    ip_address    TEXT,
    created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_resource   ON audit_log(resource_type, resource_id);

-- ── Settings (KV) ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO settings (key, value) VALUES
    ('caddy_api_url',     'http://localhost:2019'),
    ('cert_storage_path', 'data/certs'),
    ('setup_complete',    '0');

-- ── API tokens (unified human + agent credential store) ───────────────────────

CREATE TABLE IF NOT EXISTS api_tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT    NOT NULL,
    token_prefix TEXT    NOT NULL,          -- e.g. "pp_tok_a1b2c3d4"
    token_hash   TEXT    NOT NULL,          -- bcrypt of the full token
    scopes       TEXT    NOT NULL DEFAULT '["read","write"]',
    creator_role TEXT    NOT NULL DEFAULT 'admin',
    server_id    INTEGER REFERENCES servers(id) ON DELETE CASCADE,  -- NULL = human token
    created_by   INTEGER REFERENCES users(id)   ON DELETE SET NULL,
    last_used_at TEXT,
    expires_at   TEXT,
    revoked_at   TEXT,
    created_at   TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_prefix ON api_tokens(token_prefix);
CREATE INDEX IF NOT EXISTS idx_api_tokens_server ON api_tokens(server_id);

-- ── Join tokens (single-use agent enrollment) ─────────────────────────────────

CREATE TABLE IF NOT EXISTS join_tokens (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash    TEXT    NOT NULL UNIQUE,                          -- bcrypt of the raw token
    server_id     INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    created_by    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at    TEXT    NOT NULL DEFAULT (datetime('now')),
    expires_at    TEXT    NOT NULL,
    consumed_at   TEXT,
    consumed_from TEXT                                              -- remote IP that consumed it
);

CREATE INDEX IF NOT EXISTS idx_join_tokens_server_id ON join_tokens(server_id);
CREATE INDEX IF NOT EXISTS idx_join_tokens_expires   ON join_tokens(expires_at);

-- ── Config history (snapshot + rollback) ──────────────────────────────────────

CREATE TABLE IF NOT EXISTS config_history (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    resource_type TEXT    NOT NULL,
    resource_id   INTEGER NOT NULL,
    action        TEXT    NOT NULL CHECK (action IN ('create','update','delete')),
    snapshot      TEXT    NOT NULL,
    created_by    INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at    TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_config_history_resource
    ON config_history(resource_type, resource_id, created_at DESC);

-- Retention: keep the latest 100 snapshots per (resource_type, resource_id).
CREATE TRIGGER IF NOT EXISTS config_history_prune
AFTER INSERT ON config_history
BEGIN
    DELETE FROM config_history
    WHERE resource_type = NEW.resource_type
      AND resource_id   = NEW.resource_id
      AND id NOT IN (
          SELECT id FROM config_history
          WHERE resource_type = NEW.resource_type
            AND resource_id   = NEW.resource_id
          ORDER BY created_at DESC
          LIMIT 100
      );
END;
