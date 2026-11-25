import { expect, test, type BrowserContext } from '@playwright/test'
import { apiHeaders, login } from './helpers'
import { certPem, keyPem } from './testdata/cert-fixture'

async function certificateNames(
  context: BrowserContext,
  headers: Record<string, string>
): Promise<string[]> {
  const response = await context.request.get('/api/certificates', { headers })
  const certs = (await response.json()) as { name: string }[]
  return certs.map((cert) => cert.name)
}

test('uploads a valid custom certificate through the form', async ({ page, context }) => {
  await login(page)
  await page.goto('/certificates')
  await page.getByRole('button', { name: '+ Upload Certificate', exact: true }).click()

  await page.getByPlaceholder('Internal CA Wildcard').fill('e2e-upload')
  const domainInput = page.getByPlaceholder('example.com')
  await domainInput.fill('cert-upload.e2e.test')
  await domainInput.press('Enter')
  await page
    .locator('input[type="file"]')
    .nth(0)
    .setInputFiles({ name: 'cert.pem', mimeType: 'application/x-pem-file', buffer: Buffer.from(certPem) })
  await page
    .locator('input[type="file"]')
    .nth(1)
    .setInputFiles({ name: 'key.pem', mimeType: 'application/x-pem-file', buffer: Buffer.from(keyPem) })
  await page.getByRole('button', { name: 'Upload Certificate', exact: true }).click()

  const headers = await apiHeaders(context)
  await expect.poll(async () => await certificateNames(context, headers)).toContain('e2e-upload')
})

test('rejects invalid certificate material without storing it', async ({ page, context }) => {
  await login(page)
  const headers = await apiHeaders(context)
  const before = await certificateNames(context, headers)

  await page.goto('/certificates')
  await page.getByRole('button', { name: '+ Upload Certificate', exact: true }).click()
  await page.getByPlaceholder('Internal CA Wildcard').fill('bad-upload')
  const domainInput = page.getByPlaceholder('example.com')
  await domainInput.fill('bad.e2e.test')
  await domainInput.press('Enter')
  await page
    .locator('input[type="file"]')
    .nth(0)
    .setInputFiles({ name: 'bad.pem', mimeType: 'application/x-pem-file', buffer: Buffer.from('not a certificate') })
  await page
    .locator('input[type="file"]')
    .nth(1)
    .setInputFiles({ name: 'bad.key', mimeType: 'application/x-pem-file', buffer: Buffer.from('not a key') })
  await page.getByRole('button', { name: 'Upload Certificate', exact: true }).click()

  // An error surfaces in the modal and nothing is persisted.
  await expect(page.locator('div.bg-red-50')).toBeVisible()
  await expect.poll(async () => await certificateNames(context, headers)).not.toContain('bad-upload')
  expect(await certificateNames(context, headers)).toEqual(before)
})
