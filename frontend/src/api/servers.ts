import client from './client'
import type { Server } from '@/types'
import { createResourceClient } from './resourceClient'

export interface CreateServerInput {
  name: string
  hostname: string
  ssh_port: number
  ssh_user: string
  ssh_auth_method: 'password' | 'key'
  ssh_credentials: string
  deployment_type: 'systemd' | 'docker'
}

export type UpdateServerInput = Partial<CreateServerInput>

const api = createResourceClient<Server, CreateServerInput, UpdateServerInput>('/servers')

export const listServers = api.list
export const getServer = api.get
export const createServer = api.create
export const updateServer = api.update
export const deleteServer = api.delete

// Bespoke server-lifecycle endpoints — not part of the CRUD shape.
export async function getServerStatus(id: number): Promise<{ server_id: number; online: boolean }> {
  const { data } = await client.get(`/servers/${id}/status`)
  return data
}

export async function resyncServer(id: number): Promise<void> {
  await client.post(`/servers/${id}/resync`)
}

/** Toggles the server-wide HTTP/2 setting and re-syncs Caddy. Returns the updated server. */
export async function setServerHTTP2(id: number, enabled: boolean): Promise<Server> {
  const { data } = await client.put<Server>(`/servers/${id}/http2`, { enabled })
  return data
}

/** Opens a native EventSource stream to /api/servers/{id}/provision-log */
export function openProvisionLog(id: number): EventSource {
  // EventSource doesn't send auth headers, so we need the access token in the URL
  // via a short-lived approach. For now use the standard URL — the provision-log
  // endpoint is admin-only and protected by the session cookie.
  return new EventSource(`/api/servers/${id}/provision-log`, { withCredentials: true })
}
