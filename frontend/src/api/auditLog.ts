import client from './client'
import type { AuditLogEntry } from '@/types'

export interface AuditLogResponse {
  data: AuditLogEntry[]
  total: number
  page: number
  limit: number
}

export async function listAuditLog(params: {
  resource_type?: string
  page?: number
  limit?: number
}): Promise<AuditLogResponse> {
  const { data } = await client.get<AuditLogResponse>('/audit-log', { params })
  return data
}
