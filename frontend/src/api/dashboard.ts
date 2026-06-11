import client from './client'

export interface CertExpiry {
  id: number
  name: string
  expires_at: string
  days_left: number
}

export interface DashboardStats {
  proxy_hosts: number
  redirection_hosts: number
  stream_hosts: number
  dead_hosts: number
  certificates: number
  access_lists: number
  enabled_hosts: number
  expiring_soon: CertExpiry[]
}

export async function getDashboardStats(): Promise<DashboardStats> {
  const { data } = await client.get<DashboardStats>('/dashboard/stats')
  return data
}
