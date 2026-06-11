import client from './client'
import type { DeadHost } from '@/types'
import { createResourceClient } from './resourceClient'

export interface CreateDeadHostInput {
  domain_names: string[]
  server_id?: number
  status_code?: 200 | 404 | 410 | 503
  custom_body?: string
}

const api = createResourceClient<DeadHost, CreateDeadHostInput>('/dead-hosts')

export const listDeadHosts = (serverId?: number): Promise<DeadHost[]> =>
  api.list(serverId ? { server_id: serverId } : undefined)
export const getDeadHost = api.get
export const createDeadHost = api.create
export const updateDeadHost = api.update
export const deleteDeadHost = api.delete

export async function enableDeadHost(id: number): Promise<DeadHost> {
  const { data } = await client.post<DeadHost>(`/dead-hosts/${id}/enable`)
  return data
}

export async function disableDeadHost(id: number): Promise<DeadHost> {
  const { data } = await client.post<DeadHost>(`/dead-hosts/${id}/disable`)
  return data
}
