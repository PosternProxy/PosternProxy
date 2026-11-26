export interface User {
  id: number
  email: string
  name: string
  role: 'admin' | 'viewer'
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface Server {
  id: number
  name: string
  hostname: string
  ssh_port: number
  ssh_user: string
  ssh_auth_method: 'password' | 'key'
  status: 'pending' | 'provisioning' | 'online' | 'offline' | 'error'
  online: boolean
  provision_log?: string
  is_local: boolean
  last_seen_at?: string
  created_at: string
  updated_at: string
  caddy_version: string
  uptime_seconds: number
  active_routes: number
  last_status_at: string | null
  deployment_type: 'systemd' | 'docker' | 'manual'
  enrollment?: { controller_url: string; agent_token: string; server_id: number }
  http2_enabled: boolean
}

export interface ProxyHost {
  id: number
  server_id: number
  domain_names: string[]
  forward_scheme: 'http' | 'https'
  forward_host: string
  forward_port: number
  certificate_id?: number
  ssl_forced: boolean
  hsts_enabled: boolean
  hsts_subdomains: boolean
  access_list_id?: number
  advanced_config: string
  locations: ProxyHostLocation[]
  enabled: boolean
  caddy_id?: string
  // Health check configuration
  hc_enabled: boolean
  hc_active_path: string
  hc_active_interval: number // seconds
  hc_active_timeout: number // seconds
  hc_active_expect_status: number
  hc_passive_fail_duration: number // seconds
  hc_passive_max_fails: number
  hc_passive_unhealthy_latency: number // ms
  // Rate limiting
  rate_limit_enabled: boolean
  rate_limit_requests: number
  rate_limit_window: string
  // TLS passthrough
  tls_passthrough: boolean
  // HTTP-only: serve over plain HTTP, no TLS
  http_only: boolean
  // Custom error pages: map of status code string → HTML body string
  // e.g. { "404": "<html>...</html>", "502": "<html>...</html>" }
  error_pages: Record<string, string>
  // Custom header injection rules
  custom_headers: CustomHeaderRule[]
  // Load balancing
  extra_upstreams: ExtraUpstream[]
  lb_policy: string
  created_at: string
  updated_at: string
}

// ExtraUpstream is an additional backend for load-balanced proxy hosts.
export interface ExtraUpstream {
  host: string
  port: number
}

// CustomHeaderRule defines one header mutation applied to this proxy host's traffic.
export interface CustomHeaderRule {
  direction: 'request' | 'response'
  operation: 'set' | 'add' | 'delete'
  name: string
  value: string
}

export interface ConfigHistory {
  id: number
  resource_type: string
  resource_id: number
  action: string
  snapshot: Record<string, unknown>
  created_by?: number
  created_at: string
}

export interface ParsedProxyHost {
  domain_names: string[]
  forward_scheme: string
  forward_host: string
  forward_port: number
  ssl_forced: boolean
}

export interface ParsedRedirectionHost {
  domain_names: string[]
  forward_scheme: string
  forward_domain_name: string
  forward_path: string
  status_code: number
}

export interface ParseResult {
  proxy_hosts: ParsedProxyHost[]
  redirection_hosts: ParsedRedirectionHost[]
  warnings: string[]
}

export interface ImportSummary {
  proxy_hosts_created: number
  redirection_hosts_created: number
  errors: string[]
}

export type TLSProbeState = 'valid' | 'pending' | 'failed'

export interface TLSProbeStatus {
  resource_type: string
  server_id: number
  domains: { domain: string; status: TLSProbeState; detail?: string; expires_at?: string }[]
  host_id: number
  domain: string
  status: TLSProbeState
  detail?: string
  expires_at?: string
  checked_at: string
}

export type UpstreamStatus = 'healthy' | 'unhealthy' | 'unknown'

export interface UpstreamHealth {
  address: string
  status: UpstreamStatus
  num_requests: number
  fails: number
  checked_at: string
}

export interface ProxyHostLocation {
  path: string
  forward_scheme: string
  forward_host: string
  forward_port: number
  advanced_config: string
}

export interface RedirectionHost {
  ssl_forced: boolean
  certificate_id: number | null
  id: number
  server_id: number
  domain_names: string[]
  forward_scheme: 'auto' | 'http' | 'https'
  forward_domain_name: string
  forward_path: string
  preserve_path: boolean
  status_code: 301 | 302 | 307 | 308
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface StreamHost {
  id: number
  server_id: number
  incoming_port: number
  forward_host: string
  forward_port: number
  protocol: 'tcp' | 'udp' | 'tcp+udp'
  tcp_sni_match?: string
  tcp_proxy_protocol: boolean
  udp_timeout: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface DeadHost {
  http_only: boolean
  certificate_id: number | null
  access_list_id: number | null
  id: number
  server_id: number
  domain_names: string[]
  status_code: 200 | 404 | 410 | 503
  custom_body: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface Certificate {
  id: number
  name: string
  provider: string
  domain_names: string[]
  expires_at?: string
  status: 'pending' | 'active' | 'expired' | 'error'
  error_message?: string
  created_at: string
  updated_at: string
}

export interface AccessList {
  id: number
  name: string
  clients: AccessListClient[]
  created_at: string
  updated_at: string
}

export interface AccessListClient {
  id: number
  access_list_id: number
  address: string
  directive: 'allow' | 'deny'
  sort_order: number
}

export interface AuditLogEntry {
  id: number
  user_id?: number
  action: string
  resource_type: string
  resource_id?: number
  details: Record<string, unknown>
  ip_address?: string
  created_at: string
}

export interface LoginResponse {
  access_token: string
  user: User
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  limit: number
}
