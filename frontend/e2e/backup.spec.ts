import { readFile } from 'node:fs/promises'
import { expect, test } from '@playwright/test'
import { apiHeaders, findProxyHost, login } from './helpers'
import { certPem, keyPem } from './testdata/cert-fixture'

test.use({ extraHTTPHeaders: { 'X-Forwarded-For': '198.51.100.20' } })

test('encrypted UI export restores hosts and certificates and reports unapplied configuration', async ({ page, context }) => {
  await login(page)
  const headers = await apiHeaders(context)
  const domain = 'browser-recovery.e2e.test'
  const created = await context.request.post('/api/proxy-hosts', {
    headers,
    data: { domain_names: [domain], forward_scheme: 'http', forward_host: '127.0.0.1', forward_port: 8080 },
  })
  expect(created.status()).toBe(502)
  const host = await findProxyHost(context, headers, domain)
  expect(host).toBeDefined()
  const certificate = await context.request.post('/api/certificates/upload', {
    headers: { Authorization: headers.Authorization, 'X-CSRF-Token': headers['X-CSRF-Token']! },
    multipart: {
      name: 'browser-recovery-certificate', domain_names: '["cert-upload.e2e.test"]',
      cert_file: { name: 'cert.pem', mimeType: 'application/x-pem-file', buffer: Buffer.from(certPem) },
      key_file: { name: 'key.pem', mimeType: 'application/x-pem-file', buffer: Buffer.from(keyPem) },
    },
  })
  expect(certificate.status()).toBe(201)
  const certificateID = ((await certificate.json()) as { id: number }).id
  const servers = (await (await context.request.get('/api/servers', { headers })).json()) as { id: number }[]
  const mapping = Object.fromEntries(servers.filter((server) => server.id !== 1).map((server) => [String(server.id), server.id]))
  await page.goto('/backup')
  const passphrase = 'Disposable browser recovery passphrase'
  await page.getByLabel('Backup passphrase', { exact: true }).fill(passphrase)
  await page.getByLabel('Confirm backup passphrase').fill(passphrase)
  const downloaded = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Export Backup', exact: true }).click()
  const download = await downloaded
  const downloadPath = await download.path()
  expect(downloadPath).not.toBeNull()
  const archive = await readFile(downloadPath!)
  expect((JSON.parse(archive.toString()) as { version: string }).version).toBe('2.0')
  expect(archive.includes(Buffer.from(domain))).toBe(false)
  expect(archive.includes(Buffer.from('BEGIN PRIVATE KEY'))).toBe(false)

  expect((await context.request.delete(`/api/proxy-hosts/${host!.id}`, { headers })).status()).toBe(502)
  expect((await context.request.delete(`/api/certificates/${certificateID}`, { headers })).status()).toBe(204)
  await expect.poll(async () => await findProxyHost(context, headers, domain)).toBeUndefined()
  await page.locator('#backup-file-input').setInputFiles({ name: 'browser-recovery.json', mimeType: 'application/json', buffer: archive })
  await page.getByLabel('Server mapping (source ID to existing destination ID)').fill(JSON.stringify(mapping))
  await page.getByLabel('Archive passphrase').fill('Incorrect recovery passphrase')
  await page.getByRole('button', { name: 'Import Backup', exact: true }).click()
  await page.getByRole('button', { name: 'Yes, Import', exact: true }).click()
  await expect(page.locator('div.bg-red-50')).toBeVisible()
  expect(await findProxyHost(context, headers, domain)).toBeUndefined()

  await page.getByLabel('Archive passphrase').fill(passphrase)
  await page.getByRole('button', { name: 'Import Backup', exact: true }).click()
  await page.getByRole('button', { name: 'Yes, Import', exact: true }).click()
  await expect(page.locator('div.bg-emerald-50')).toContainText('Configuration is still waiting to apply')
  await expect.poll(async () => (await findProxyHost(context, headers, domain))?.forward_port).toBe(8080)
  const restoredCertificates = (await (await context.request.get('/api/certificates', { headers })).json()) as { name: string }[]
  expect(restoredCertificates.some((cert) => cert.name === 'browser-recovery-certificate')).toBe(true)
})
