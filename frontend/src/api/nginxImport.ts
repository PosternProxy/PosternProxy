import client from './client'
import type { ParseResult, ImportSummary } from '@/types'

export async function previewNginxImport(file: File): Promise<ParseResult> {
  const formData = new FormData()
  formData.append('file', file)
  const { data } = await client.post<ParseResult>('/import/nginx/preview', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return data
}

// confirmNginxImport re-uploads the original nginx file so the server can
// re-parse it. This prevents tampering with the ParseResult between preview
// and confirm — without it, a client could inject arbitrary proxy-host JSON
// that bypasses parser-level constraints.
export async function confirmNginxImport(file: File): Promise<ImportSummary> {
  const formData = new FormData()
  formData.append('file', file)
  const { data } = await client.post<ImportSummary>('/import/nginx/confirm', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return data
}
