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

// Returns API headers for an authenticated context, including API-only sessions
// that have not made the safe request which seeds the SPA's CSRF cookie.
export async function apiHeaders(context: BrowserContext): Promise<Record<string, string>> {
  const refresh = await context.request.post('/api/auth/refresh')
  expect(refresh.status()).toBe(201)
  const { access_token } = (await refresh.json()) as { access_token: string }
  const authorization = `Bearer ${access_token}`
  const identity = await context.request.get('/api/auth/me', {
    headers: { Authorization: authorization },
  })
  expect(identity.status()).toBe(200)
  const csrf = (await context.cookies()).find((cookie) => cookie.name === 'csrf_token')
  expect(csrf).toBeDefined()
  return { Authorization: authorization, 'X-CSRF-Token': csrf!.value }
}

type ProxyHost = {
  id: number
  domain_names: string[]
  enabled: boolean
  forward_port: number
  ntlm_enabled: boolean
}

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
