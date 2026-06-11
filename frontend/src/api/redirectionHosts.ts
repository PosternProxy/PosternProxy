import client from './client'
import type { RedirectionHost } from '@/types'
import { createResourceClient } from './resourceClient'

export interface CreateRedirectionHostInput {
  domain_names: string[]
  server_id?: number
  forward_scheme: 'auto' | 'http' | 'https'
  forward_domain_name: string
  forward_path?: string
  preserve_path?: boolean
  status_code?: 301 | 302 | 307 | 308
}

const api = createResourceClient<RedirectionHost, CreateRedirectionHostInput>('/redirection-hosts')

export const listRedirectionHosts = (serverId?: number): Promise<RedirectionHost[]> =>
  api.list(serverId ? { server_id: serverId } : undefined)
export const getRedirectionHost = api.get
export const createRedirectionHost = api.create
export const updateRedirectionHost = api.update
export const deleteRedirectionHost = api.delete

export async function enableRedirectionHost(id: number): Promise<RedirectionHost> {
  const { data } = await client.post<RedirectionHost>(`/redirection-hosts/${id}/enable`)
  return data
}

export async function disableRedirectionHost(id: number): Promise<RedirectionHost> {
  const { data } = await client.post<RedirectionHost>(`/redirection-hosts/${id}/disable`)
  return data
}
