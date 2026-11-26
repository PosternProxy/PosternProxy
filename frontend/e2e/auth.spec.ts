import { expect, test } from '@playwright/test'
import { apiHeaders, login } from './helpers'

test('login, reload using the refresh cookie, and logout', async ({ page, context }) => {
  await page.goto('/dashboard')
  await expect(page).toHaveURL(/\/login$/)
  await page.getByLabel('Email').fill('admin@e2e.test')
  await page.getByLabel('Password').fill('Disposable-browser-test-password')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page).toHaveURL(/\/dashboard$/)

  const cookie = (await context.cookies()).find((entry) => entry.name === 'refresh_token')
  expect(cookie?.httpOnly).toBe(true)
  expect(cookie?.sameSite).toBe('Strict')
  await page.reload()
  await expect(page).toHaveURL(/\/dashboard$/)
  await page.getByRole('button', { name: 'A Admin', exact: true }).click()
  await expect(page.getByRole('button', { name: /sign out|logout|log out/i })).toBeVisible()
  await page.getByRole('button', { name: /sign out|logout|log out/i }).click()
  await expect(page).toHaveURL(/\/login$/)
  await page.reload()
  await expect(page).toHaveURL(/\/login$/)
})

test('saved configuration has a visible application failure', async ({ page, context }) => {
  await login(page)
  const headers = await apiHeaders(context)
  const result = await context.request.post('/api/proxy-hosts', {
    headers,
    data: {
      domain_names: ['failed-apply.e2e.test'],
      forward_scheme: 'http',
      forward_host: '127.0.0.1',
      forward_port: 8080,
    },
  })
  expect(result.status()).toBe(502)
  const body = (await result.json()) as { saved: boolean; server_id: number }
  expect(body.saved).toBe(true)
  expect(body.server_id).toBe(1)
  const notice = page.getByRole('region', { name: 'Configuration status' })
  await expect(notice).toContainText('configuration failed', { timeout: 10_000 })
  await expect(notice.getByRole('button', { name: 'Retry sync' })).toBeVisible()
})

test('manual enrollment exposes credentials once and settings reflect runtime', async ({
  page,
}) => {
  await page.goto('/login')
  await page.getByLabel('Email').fill('admin@e2e.test')
  await page.getByLabel('Password').fill('Disposable-browser-test-password')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
  await page.goto('/servers')
  await page.getByRole('button', { name: /Add Server/ }).click()
  await page.getByRole('combobox').last().selectOption('manual')
  await page.getByPlaceholder('prod-proxy-01').fill('manual-browser-node')
  await page.getByPlaceholder('203.0.113.10').fill('manual.e2e.test')
  await page.getByRole('button', { name: 'Create enrollment' }).click()
  await expect(page.getByRole('heading', { name: 'Agent enrollment' })).toBeVisible()
  await expect(page.locator('pre')).toContainText('agent_token')
  await page.getByRole('button', { name: 'Close', exact: true }).click()
  await page.reload()
  await expect(page.getByText('manual-browser-node')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Agent enrollment' })).toHaveCount(0)
  await page.goto('/settings')
  await expect(page.locator('input').first()).toHaveValue('http://127.0.0.1:1')
  await expect(page.locator('input').first()).toHaveAttribute('readonly', '')
})

test('parked domains expose HTTP-only mode and TLS diagnostics', async ({ page, context }) => {
  await login(page)
  const headers = await apiHeaders(context)
  const created = await context.request.post('/api/dead-hosts', {
    headers,
    data: {
      domain_names: ['park-http.e2e.test'],
      http_only: true,
      status_code: 200,
      custom_body: 'parked',
    },
  })
  expect(created.status()).toBe(502)
  const listed = await context.request.get('/api/dead-hosts', { headers })
  const records = (await listed.json()) as {
    id: number
    domain_names: string[]
    http_only: boolean
  }[]
  const host = records.find((entry) => entry.domain_names.includes('park-http.e2e.test'))!
  expect(host.http_only).toBe(true)
  const httpsCreated = await context.request.post('/api/dead-hosts', {
    headers,
    data: { domain_names: ['park-tls.e2e.test'] },
  })
  expect(httpsCreated.status()).toBe(502)
  const updated = await context.request.get('/api/dead-hosts', { headers })
  const httpsHost = ((await updated.json()) as { id: number; domain_names: string[] }[]).find(
    (entry) => entry.domain_names.includes('park-tls.e2e.test')
  )!
  await page.route('**/api/health/tls', (route) =>
    route.fulfill({
      json: [
        {
          resource_type: 'dead_host',
          server_id: 1,
          host_id: httpsHost.id,
          domain: 'park-tls.e2e.test',
          status: 'failed',
          detail: 'TLS handshake unavailable',
          checked_at: new Date().toISOString(),
          domains: [{ domain: 'park-tls.e2e.test', status: 'failed' }],
        },
      ],
    })
  )
  await page.goto('/dead-hosts')
  const tlsRow = page.getByRole('row').filter({ hasText: 'park-tls.e2e.test' })
  await tlsRow.getByText('TLS unavailable', { exact: true }).click()
  await expect(
    tlsRow.getByText('sudo journalctl -u caddy -n 100 --no-pager', { exact: true })
  ).toBeVisible()
  const row = page.getByRole('row').filter({ hasText: 'park-http.e2e.test' })
  await expect(row.getByText('HTTP only', { exact: true })).toBeVisible()
  await row.getByRole('button', { name: 'Edit', exact: true }).click()
  await expect(page.getByRole('checkbox', { name: 'HTTP only (no certificate)' })).toBeChecked()
})
