import client from './client'
import type { UpstreamHealth, TLSProbeStatus } from '@/types'

export async function getUpstreamHealth(): Promise<UpstreamHealth[]> {
  const { data } = await client.get<UpstreamHealth[]>('/health/upstreams')
  return data
}

export async function getTLSStatus(): Promise<TLSProbeStatus[]> {
  const { data } = await client.get<TLSProbeStatus[]>('/health/tls')
  return data
}
