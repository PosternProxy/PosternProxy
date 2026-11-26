import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { createServer } from 'node:http'
import { tmpdir } from 'node:os'
import { join, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'
import { apiHeaders, login } from './helpers'

test.use({ extraHTTPHeaders: { 'X-Forwarded-For': '198.51.100.21' } })

test('agent connectivity is distinct from applied readiness and failed apply can be retried', async ({ page, context }) => {
  await login(page)
  const headers = await apiHeaders(context)
  const created = await context.request.post('/api/servers', { headers,
    data: { name: 'browser-readiness-agent', hostname: '127.0.0.1', deployment_type: 'manual' } })
  expect(created.status()).toBe(201)
  const server = await created.json() as { id: number; enrollment: { controller_url: string; agent_token: string } }
  let accept = false
  let release: (() => void) | undefined
  const firstLoad = new Promise<void>((done) => { release = done })
  let loadStarted = false
  const caddy = createServer(async (request, response) => {
    request.resume()
    if (request.url === '/load') {
      loadStarted = true
      await firstLoad
      response.writeHead(accept ? 200 : 400)
      response.end(accept ? '' : 'Rejected by the browser readiness fixture')
    } else { response.writeHead(200); response.end('{}') }
  })
  caddy.listen(0, '127.0.0.1')
  await once(caddy, 'listening')
  const address = caddy.address()
  if (!address || typeof address === 'string') throw new Error('Missing fixture listen port')
  const metadata = JSON.parse(await readFile(fileURLToPath(new URL('../node_modules/.tmp/posternproxy-e2e.json', import.meta.url)), 'utf8')) as { agentBinary: string }
  const work = await mkdtemp(join(tmpdir(), 'posternproxy-browser-agent-'))
  const agent = spawn(metadata.agentBinary, [], { stdio: 'ignore', env: { ...process.env,
    POSTERNPROXY_CONTROLLER_URL: server.enrollment.controller_url,
    POSTERNPROXY_AGENT_TOKEN: server.enrollment.agent_token,
    POSTERNPROXY_SERVER_ID: String(server.id),
    POSTERNPROXY_CADDY_API: `http://127.0.0.1:${address.port}`,
    POSTERNPROXY_CERT_RUNTIME_DIR: join(work, 'runtime'),
    POSTERNPROXY_CERT_DIR: join(work, 'legacy'),
  } })
  const exited = once(agent, 'close')
  try {
    await expect.poll(() => loadStarted).toBe(true)
    await page.goto('/servers')
    const row = page.getByRole('row').filter({ hasText: 'browser-readiness-agent' })
    await expect(row.getByText('Online', { exact: true })).toBeVisible()
    const notice = page.getByRole('region', { name: 'Configuration status' })
    const status = notice.locator('p.font-medium').filter({ hasText: new RegExp(`^Server ${server.id}:`) })
    await expect(status).toHaveText(`Server ${server.id}: applying saved changes`)
    release?.()
    await expect(status).toHaveText(`Server ${server.id}: configuration failed`, { timeout: 10_000 })
    accept = true
    await status.locator('../..').getByRole('button', { name: 'Retry sync' }).click()
    await expect.poll(async () => {
      const result = await context.request.get('/api/configuration', { headers })
      const states = await result.json() as { server_id: number; status: string; desired_revision: number; applied_revision: number }[]
      const state = states.find((entry) => entry.server_id === server.id)
      return state?.status === 'applied' && state.desired_revision === state.applied_revision
    }).toBe(true)
    await expect(status).toHaveCount(0, { timeout: 10_000 })
  } finally {
    release?.()
    agent.kill()
    await exited
    caddy.closeAllConnections()
    await new Promise<void>((done) => caddy.close(() => done()))
    const deleted = await context.request.delete(`/api/servers/${server.id}`, { headers })
    if (resolve(work).startsWith(resolve(tmpdir()) + sep + 'posternproxy-browser-agent-')) await rm(work, { recursive: true, force: true })
    expect(deleted.status()).toBe(204)
  }
})
