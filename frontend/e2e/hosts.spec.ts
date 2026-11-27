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

test('NTLM passthrough survives a failed apply and refuses a location', async ({ page, context }) => {
  await login(page)
  const headers = await apiHeaders(context)
  const domain = 'ui-ntlm.e2e.test'

  await page.goto('/proxy-hosts')
  await page.getByRole('button', { name: 'Add Proxy Host' }).click()
  const domainInput = page.getByPlaceholder('example.com')
  await domainInput.fill(domain)
  await domainInput.press('Enter')
  await page.getByPlaceholder('192.168.1.10').fill('127.0.0.1')
  await page.getByPlaceholder('80').fill('8080')
  await page.getByRole('button', { name: 'Advanced Settings' }).click()
  await page.getByRole('button', { name: /^security$/i }).click()
  await page.getByRole('checkbox', { name: /NTLM authentication passthrough/ }).check()
  await page.getByRole('button', { name: 'Create', exact: true }).click()

  await expect(page.getByRole('heading', { name: 'Add Proxy Host' })).toBeVisible()
  await expect(page.getByText('Configuration saved, but server')).toBeVisible()
  await expect(page.getByRole('checkbox', { name: /NTLM authentication passthrough/ })).toBeChecked()
  await expect
    .poll(async () => (await findProxyHost(context, headers, domain))?.ntlm_enabled)
    .toBe(true)

  const update = page.waitForResponse(
    (response) => response.request().method() === 'PUT' && response.url().includes('/api/proxy-hosts/')
  )
  await page.getByRole('button', { name: 'Create', exact: true }).click()
  expect((await update).status()).toBe(502)
  const listed = await context.request.get('/api/proxy-hosts', { headers })
  const hosts = (await listed.json()) as { domain_names: string[] }[]
  expect(hosts.filter((host) => host.domain_names.includes(domain))).toHaveLength(1)

  await page.getByRole('button', { name: 'Cancel', exact: true }).click()
  await page.goto('/proxy-hosts')
  await page.getByRole('row').filter({ hasText: domain }).getByRole('button', { name: 'Edit', exact: true }).click()
  await page.getByRole('button', { name: /^security$/i }).click()
  const ntlm = page.getByRole('checkbox', { name: /NTLM authentication passthrough/ })
  await expect(ntlm).toBeChecked()
  await ntlm.uncheck()
  await page.getByRole('button', { name: 'Save Changes', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Edit Proxy Host' })).toBeVisible()
  await expect(page.getByText('Configuration saved, but server')).toBeVisible()
  await expect(ntlm).not.toBeChecked()
  await expect
    .poll(async () => (await findProxyHost(context, headers, domain))?.ntlm_enabled)
    .toBe(false)
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()

  const conflictDomain = 'ui-ntlm-conflict.e2e.test'
  await page.getByRole('button', { name: 'Add Proxy Host' }).click()
  const conflictDomainInput = page.getByPlaceholder('example.com')
  await conflictDomainInput.fill(conflictDomain)
  await conflictDomainInput.press('Enter')
  await page.getByPlaceholder('192.168.1.10').fill('127.0.0.1')
  await page.getByPlaceholder('80').fill('8080')
  await page.getByRole('button', { name: 'Advanced Settings' }).click()
  await page.getByRole('button', { name: /^locations$/i }).click()
  await page.getByRole('button', { name: 'Add Location' }).click()
  await page.getByPlaceholder('/api').fill('/ntlm-conflict')
  await page.getByRole('button', { name: /^security$/i }).click()
  await page.getByRole('checkbox', { name: /NTLM authentication passthrough/ }).check()
  await page.getByRole('button', { name: 'Create', exact: true }).click()

  await expect(page.getByText('NTLM passthrough cannot be combined with path locations')).toBeVisible()
  await expect(page.getByPlaceholder('/api')).toHaveValue('/ntlm-conflict')
  await page.getByRole('button', { name: /^security$/i }).click()
  await expect(page.getByRole('checkbox', { name: /NTLM authentication passthrough/ })).toBeChecked()
  expect(await findProxyHost(context, headers, conflictDomain)).toBeUndefined()
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
