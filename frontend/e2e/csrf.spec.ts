import { expect, test } from '@playwright/test'
import { apiHeaders, login } from './helpers'

const host = {
  domain_names: ['csrf.e2e.test'],
  forward_scheme: 'http',
  forward_host: '127.0.0.1',
  forward_port: 8080,
}

test('CSRF double-submit is enforced on state-changing requests', async ({ page, context }) => {
  await login(page)
  const headers = await apiHeaders(context)

  // The csrf_token cookie is sent automatically; omitting the matching header
  // must be rejected even with a valid bearer token.
  const missing = await context.request.post('/api/proxy-hosts', {
    headers: { Authorization: headers.Authorization },
    data: host,
    failOnStatusCode: false,
  })
  expect(missing.status()).toBe(403)

  // A header that does not match the cookie is rejected.
  const mismatched = await context.request.post('/api/proxy-hosts', {
    headers: { Authorization: headers.Authorization, 'X-CSRF-Token': 'not-the-cookie-value' },
    data: host,
    failOnStatusCode: false,
  })
  expect(mismatched.status()).toBe(403)

  // The correct double-submit passes CSRF; it then fails apply against the
  // harness's dead Caddy endpoint, which is the expected saved-but-failed path.
  const accepted = await context.request.post('/api/proxy-hosts', {
    headers,
    data: host,
    failOnStatusCode: false,
  })
  expect(accepted.status()).toBe(502)
  expect(((await accepted.json()) as { saved: boolean }).saved).toBe(true)
})

test('csrf_token cookie is strict, script-readable, and non-secure over http', async ({
  page,
  context,
}) => {
  await login(page)
  // The SPA seeds the csrf_token cookie on its first safe-method API call after
  // login; wait for it rather than racing that request.
  await expect
    .poll(async () => (await context.cookies()).some((cookie) => cookie.name === 'csrf_token'))
    .toBe(true)
  const csrf = (await context.cookies()).find((cookie) => cookie.name === 'csrf_token')
  expect(csrf).toBeDefined()
  // Strict so it is not sent cross-site, readable by JS so the SPA can echo it
  // into the header, and non-secure because the harness serves plain HTTP.
  expect(csrf?.sameSite).toBe('Strict')
  expect(csrf?.httpOnly).toBe(false)
  expect(csrf?.secure).toBe(false)
})
