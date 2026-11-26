import { expect, test } from '@playwright/test'
import { login } from './helpers'

test('HTTPS on a non-localhost origin preserves secure sessions and enforces CSRF', async ({ page, context }) => {
  await login(page)
  await expect.poll(async () => (await context.cookies()).some((cookie) => cookie.name === 'csrf_token')).toBe(true)
  const cookies = await context.cookies()
  const refresh = cookies.find((cookie) => cookie.name === 'refresh_token')
  const csrf = cookies.find((cookie) => cookie.name === 'csrf_token')
  expect(refresh?.domain).toBe('cert-upload.e2e.test')
  expect(refresh?.secure).toBe(true)
  expect(refresh?.httpOnly).toBe(true)
  expect(refresh?.sameSite).toBe('Strict')
  expect(csrf?.secure).toBe(true)
  expect(csrf?.httpOnly).toBe(false)
  expect(csrf?.sameSite).toBe('Strict')
  expect(await page.evaluate(() => document.cookie.includes('refresh_token='))).toBe(false)
  await page.reload()
  await expect(page).toHaveURL(/\/dashboard$/)
  const outcomes = await page.evaluate(async () => {
    const response = await fetch('/api/auth/refresh', { method: 'POST' })
    const { access_token } = await response.json() as { access_token: string }
    const csrfToken = document.cookie.split('; ').find((entry) => entry.startsWith('csrf_token='))?.slice('csrf_token='.length) ?? ''
    const statuses: number[] = []
    for (const csrf of [undefined, 'incorrect-cookie', csrfToken]) {
      const headers: Record<string, string> = { Authorization: `Bearer ${access_token}`, 'Content-Type': 'application/json' }
      if (csrf !== undefined) headers['X-CSRF-Token'] = csrf
      const mutation = await fetch('/api/proxy-hosts', { method: 'POST', headers,
        body: JSON.stringify({ domain_names: ['secure-csrf.e2e.test'], forward_scheme: 'http', forward_host: '127.0.0.1', forward_port: 8080 }) })
      statuses.push(mutation.status)
    }
    return { refreshStatus: response.status, statuses }
  })
  expect(outcomes).toEqual({ refreshStatus: 201, statuses: [403, 403, 502] })

  const attacker = await context.newPage()
  await attacker.goto('https://attacker.test:18184')
  const crossSiteRequest = attacker.waitForRequest((request) => request.url().endsWith('/api/auth/refresh'))
  await attacker.evaluate(async () => {
    await fetch('https://cert-upload.e2e.test:18184/api/auth/refresh', { method: 'POST', credentials: 'include' }).catch(() => undefined)
  })
  const sentCookies = (await (await crossSiteRequest).allHeaders()).cookie ?? ''
  expect(sentCookies.includes('refresh_token=')).toBe(false)
  expect(sentCookies.includes('csrf_token=')).toBe(false)
  await attacker.close()

  await page.getByRole('button', { name: 'A Admin', exact: true }).click()
  await page.getByRole('button', { name: /sign out|logout|log out/i }).click()
  await expect(page).toHaveURL(/\/login$/)
  await page.reload()
  await expect(page).toHaveURL(/\/login$/)
  expect((await context.cookies()).some((cookie) => cookie.name === 'refresh_token')).toBe(false)
})
