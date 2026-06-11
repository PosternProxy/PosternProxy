import client from './client'
import type { UpstreamHealth } from '@/types'

export async function getUpstreamHealth(): Promise<UpstreamHealth[]> {
  const { data } = await client.get<UpstreamHealth[]>('/health/upstreams')
  return data
}
