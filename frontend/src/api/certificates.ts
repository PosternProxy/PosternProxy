import client from './client'
import type { Certificate } from '@/types'
import { createResourceClient } from './resourceClient'

// Certificates have no JSON create/update path — they are uploaded via the
// bespoke multipart endpoint below. The factory still supplies list/get/delete.
const api = createResourceClient<Certificate, never>('/certificates')

export const listCertificates = api.list
export const getCertificate = api.get
export const deleteCertificate = api.delete

export async function uploadCustomCertificate(
  name: string,
  domainNames: string[],
  certFile: File,
  keyFile: File
): Promise<Certificate> {
  const form = new FormData()
  form.append('name', name)
  form.append('domain_names', JSON.stringify(domainNames))
  form.append('cert_file', certFile)
  form.append('key_file', keyFile)
  const { data } = await client.post<Certificate>('/certificates/upload', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return data
}
