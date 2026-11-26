import client from './client'

export async function exportBackup(passphrase: string): Promise<void> {
  // Fetch through the authenticated axios client so Bearer headers are sent.
  const { data } = await client.post<Blob>('/backup/export', { passphrase }, {
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

export async function importBackup(file: File, passphrase: string, createUnenrolled: boolean, serverMapping: Record<string, number>): Promise<{ restored: number; synced: boolean }> {
  const text = await file.text()
  const json = JSON.parse(text)
  const { data } = await client.post<{ restored: number; synced: boolean }>(json.version === '2.0' ? '/backup/restore' : '/backup/import', json.version === '2.0' ? {archive: json, passphrase, create_unenrolled: createUnenrolled, server_mapping: serverMapping} : json)
  return data
}
