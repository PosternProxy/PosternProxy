import client from './client'
import type { StreamHost } from '@/types'
import { createResourceClient } from './resourceClient'

export interface CreateStreamHostInput {
  server_id?: number
  incoming_port: number
  forward_host: string
  forward_port: number
  protocol?: 'tcp' | 'udp' | 'tcp+udp'
  tcp_sni_match?: string | null
  tcp_proxy_protocol?: boolean
  udp_timeout?: number
}

const api = createResourceClient<StreamHost, CreateStreamHostInput>('/stream-hosts')

export const listStreamHosts = (serverId?: number): Promise<StreamHost[]> =>
  api.list(serverId ? { server_id: serverId } : undefined)
export const getStreamHost = api.get
export const createStreamHost = api.create
export const updateStreamHost = api.update
export const deleteStreamHost = api.delete

export async function enableStreamHost(id: number): Promise<StreamHost> {
  const { data } = await client.post<StreamHost>(`/stream-hosts/${id}/enable`)
  return data
}

export async function disableStreamHost(id: number): Promise<StreamHost> {
  const { data } = await client.post<StreamHost>(`/stream-hosts/${id}/disable`)
  return data
}
