import client from './client'
import type { ProxyHost, CustomHeaderRule, ExtraUpstream } from '@/types'
import { createResourceClient } from './resourceClient'

export interface CreateProxyHostInput {
  domain_names: string[]
  server_id?: number
  forward_scheme: 'http' | 'https'
  forward_host: string
  forward_port: number
  ssl_forced?: boolean
  hsts_enabled?: boolean
  hsts_subdomains?: boolean
  certificate_id?: number | null
  access_list_id?: number | null
  advanced_config?: string
  locations?: Array<{
    path: string
    forward_scheme: string
    forward_host: string
    forward_port: number
    advanced_config: string
  }>
  // Health checks
  hc_enabled?: boolean
  hc_active_path?: string
  hc_active_interval?: number
  hc_active_timeout?: number
  hc_active_expect_status?: number
  hc_passive_fail_duration?: number
  hc_passive_max_fails?: number
  hc_passive_unhealthy_latency?: number
  // Rate limiting
  rate_limit_enabled?: boolean
  rate_limit_requests?: number
  rate_limit_window?: string
  // TLS passthrough
  tls_passthrough?: boolean
  // HTTP-only: serve over plain HTTP, no TLS
  http_only?: boolean
  // Custom error pages: map of status code string → HTML body
  error_pages?: Record<string, string>
  // Custom header injection rules
  custom_headers?: CustomHeaderRule[]
  // Load balancing
  extra_upstreams?: ExtraUpstream[]
  lb_policy?: string
}

const api = createResourceClient<ProxyHost, CreateProxyHostInput>('/proxy-hosts')

export const listProxyHosts = (serverId?: number): Promise<ProxyHost[]> =>
  api.list(serverId ? { server_id: serverId } : undefined)
export const getProxyHost = api.get
export const createProxyHost = api.create
export const updateProxyHost = api.update
export const deleteProxyHost = api.delete

// Bespoke enable/disable endpoints — not part of the CRUD shape.
export async function enableProxyHost(id: number): Promise<ProxyHost> {
  const { data } = await client.post<ProxyHost>(`/proxy-hosts/${id}/enable`)
  return data
}

export async function disableProxyHost(id: number): Promise<ProxyHost> {
  const { data } = await client.post<ProxyHost>(`/proxy-hosts/${id}/disable`)
  return data
}
