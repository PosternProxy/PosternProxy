import client from './client'
import type { ConfigHistory } from '@/types'

export async function listHistory(
  resourceType: string,
  resourceId: number
): Promise<ConfigHistory[]> {
  const { data } = await client.get<ConfigHistory[]>('/config-history', {
    params: { resource_type: resourceType, resource_id: resourceId },
  })
  return data
}

export async function rollbackHistory(id: number): Promise<void> {
  await client.post(`/config-history/${id}/rollback`)
}
