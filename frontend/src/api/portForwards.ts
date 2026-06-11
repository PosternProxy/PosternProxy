import client from './client'
import type { PortForward } from '@/types'
import { createResourceClient } from './resourceClient'

export interface CreatePortForwardInput {
  protocol: 'tcp' | 'udp' | 'tcp+udp'
  incoming_port: number
  forward_host: string
  forward_port: number
  interface: string
  method: 'iptables' | 'nftables' | 'socat'
}

const api = createResourceClient<PortForward, CreatePortForwardInput>('/port-forwards')

export const listPortForwards = (serverId?: number): Promise<PortForward[]> =>
  api.list(serverId ? { server_id: serverId } : undefined)
export const getPortForward = api.get
export const createPortForward = api.create
export const updatePortForward = api.update
export const deletePortForward = api.delete

export async function enablePortForward(id: number): Promise<PortForward> {
  const { data } = await client.post<PortForward>(`/port-forwards/${id}/enable`)
  return data
}

export async function disablePortForward(id: number): Promise<PortForward> {
  const { data } = await client.post<PortForward>(`/port-forwards/${id}/disable`)
  return data
}
