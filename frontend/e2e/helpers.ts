import { expect, type BrowserContext, type Page } from '@playwright/test'

export const ADMIN = {
  email: 'admin@e2e.test',
  password: 'Disposable-browser-test-password',
}

// Logs in through the real login form and lands on the dashboard.
export async function login(page: Page): Promise<void> {
  await page.goto('/login')
  await page.getByLabel('Email').fill(ADMIN.email)
  await page.getByLabel('Password').fill(ADMIN.password)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page).toHaveURL(/\/dashboard$/)
}

// Returns the Authorization + CSRF headers for direct API assertions, mirroring
// the double-submit cookie the SPA uses. Must be called after login().
export async function apiHeaders(context: BrowserContext): Promise<Record<string, string>> {
  const refresh = await context.request.post('/api/auth/refresh')
  expect(refresh.status()).toBe(201)
  const { access_token } = (await refresh.json()) as { access_token: string }
  const csrf = (await context.cookies()).find((cookie) => cookie.name === 'csrf_token')
  expect(csrf).toBeDefined()
  return { Authorization: `Bearer ${access_token}`, 'X-CSRF-Token': csrf!.value }
}

type ProxyHost = { id: number; domain_names: string[]; enabled: boolean; forward_port: number }

// Fetches the current proxy host for a domain, or undefined if absent. Used to
// assert that UI-driven mutations actually persisted server-side.
export async function findProxyHost(
  context: BrowserContext,
  headers: Record<string, string>,
  domain: string
): Promise<ProxyHost | undefined> {
  const response = await context.request.get('/api/proxy-hosts', { headers })
  const hosts = (await response.json()) as ProxyHost[]
  return hosts.find((host) => host.domain_names.includes(domain))
}
