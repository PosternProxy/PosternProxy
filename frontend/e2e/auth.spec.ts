import { expect, test } from '@playwright/test'

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
  await page.goto('/login')
  await page.getByLabel('Email').fill('admin@e2e.test')
  await page.getByLabel('Password').fill('Disposable-browser-test-password')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
  const refresh = await context.request.post('/api/auth/refresh')
  expect(refresh.status()).toBe(201)
  const { access_token } = await refresh.json() as { access_token: string }
  const csrf = (await context.cookies()).find((cookie) => cookie.name === 'csrf_token')
  expect(csrf).toBeDefined()
  const result = await context.request.post('/api/proxy-hosts', {
    headers: { Authorization: `Bearer ${access_token}`, 'X-CSRF-Token': csrf!.value },
    data: { domain_names: ['failed-apply.e2e.test'], forward_scheme: 'http', forward_host: '127.0.0.1', forward_port: 8080 },
  })
  expect(result.status()).toBe(502)
  const body = await result.json() as { saved: boolean; server_id: number }
  expect(body.saved).toBe(true)
  expect(body.server_id).toBe(1)
  const notice = page.getByRole('region', { name: 'Configuration status' })
  await expect(notice).toContainText('configuration failed', { timeout: 10_000 })
  await expect(notice.getByRole('button', { name: 'Retry sync' })).toBeVisible()
})
