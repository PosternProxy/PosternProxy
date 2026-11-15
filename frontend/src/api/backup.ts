import client from './client'

export async function exportBackup(): Promise<void> {
  // Fetch through the authenticated axios client so Bearer headers are sent.
  const { data } = await client.get<Blob>('/backup/export', {
    responseType: 'blob',
  })
  const url = URL.createObjectURL(data)
  const a = document.createElement('a')
  a.href = url
  a.download = `posternproxy-backup-${new Date().toISOString().slice(0, 10)}.json`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

export async function importBackup(file: File): Promise<{ restored: number; synced: boolean }> {
  const text = await file.text()
  const json = JSON.parse(text)
  const { data } = await client.post<{ restored: number; synced: boolean }>('/backup/import', json)
  return data
}
