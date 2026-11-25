import { expect, test } from '@playwright/test'
import { apiHeaders, findProxyHost, login } from './helpers'

test('a proxy host can be created through the form', async ({ page, context }) => {
  await login(page)
  await page.goto('/proxy-hosts')
  await page.getByRole('button', { name: 'Add Proxy Host' }).click()

  const domain = 'ui-create.e2e.test'
  const domainInput = page.getByPlaceholder('example.com')
  await domainInput.fill(domain)
  await domainInput.press('Enter')
  await page.getByPlaceholder('192.168.1.10').fill('127.0.0.1')
  await page.getByPlaceholder('80').fill('8080')
  await page.getByRole('button', { name: 'Create', exact: true }).click()

  // Apply fails against the dead Caddy, but the record must be saved; assert
  // the UI create actually persisted server-side.
  const headers = await apiHeaders(context)
  await expect
    .poll(async () => (await findProxyHost(context, headers, domain))?.enabled)
    .toBe(true)
})

test('a proxy host can be disabled and edited through the UI', async ({ page, context }) => {
  await login(page)
  const headers = await apiHeaders(context)
  const domain = 'ui-edit.e2e.test'

  // Seed via the API (saved-but-apply-failed) so the row exists to act on.
  const created = await context.request.post('/api/proxy-hosts', {
    headers,
    data: { domain_names: [domain], forward_scheme: 'http', forward_host: '127.0.0.1', forward_port: 8080 },
    failOnStatusCode: false,
  })
  expect(created.status()).toBe(502)

  await page.goto('/proxy-hosts')
  const row = page.getByRole('row').filter({ hasText: domain })
  await expect(row).toBeVisible()

  // Disable through the status toggle. The apply fails against the dead Caddy so
  // the row does not optimistically flip, but the change is persisted; confirm
  // via the API, then reload to confirm the UI reflects the stored state.
  await row.getByRole('button', { name: 'Enabled', exact: true }).click()
  await expect
    .poll(async () => (await findProxyHost(context, headers, domain))?.enabled)
    .toBe(false)
  await page.goto('/proxy-hosts')
  await expect(
    page.getByRole('row').filter({ hasText: domain }).getByRole('button', { name: 'Disabled', exact: true })
  ).toBeVisible()

  // Edit the forward port through the form; verify it persisted.
  await page.getByRole('row').filter({ hasText: domain }).getByRole('button', { name: 'Edit', exact: true }).click()
  await page.getByPlaceholder('80').fill('9090')
  await page.getByRole('button', { name: 'Save Changes', exact: true }).click()
  await expect
    .poll(async () => (await findProxyHost(context, headers, domain))?.forward_port)
    .toBe(9090)
})
