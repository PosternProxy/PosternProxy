import { spawn, spawnSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { createServer as createHTTPSServer } from 'node:https'
import { request as httpRequest } from 'node:http'
import { tmpdir } from 'node:os'
import { dirname, join, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontend = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const root = resolve(frontend, '..')
const tempRoot = resolve(tmpdir())
const data = mkdtempSync(join(tempRoot, 'posternproxy-e2e-'))
const binary = join(data, process.platform === 'win32' ? 'controller.exe' : 'controller')
const agentBinary = join(data, process.platform === 'win32' ? 'agent.exe' : 'agent')
const metadata = join(frontend, 'node_modules', '.tmp', 'posternproxy-e2e.json')
const children = []
const servers = []
let stopping = false

function stop(code = 0) {
  if (stopping) return
  stopping = true
  for (const server of servers) { server.closeAllConnections(); server.close() }
  for (const child of children) child.kill()
  // Windows may still hold the executable until its process exits.
  Promise.all(children.map((child) => new Promise((done) => {
    if (child.exitCode !== null || child.signalCode !== null || !child.pid) done()
    else child.once('close', done)
  }))).then(() => {
    if (data.startsWith(tempRoot + sep + 'posternproxy-e2e-')) {
      rmSync(data, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 })
    }
    rmSync(metadata, { force: true })
    process.exit(code)
  })
}

const build = spawnSync('go', ['build', '-o', binary, './cmd/posternproxy'], {
  cwd: root,
  stdio: ['ignore', 'ignore', 'pipe'],
})
if (build.status !== 0) {
  process.stderr.write(build.stderr ?? 'Controller build failed')
  rmSync(data, { recursive: true, force: true })
  process.exit(1)
}

const agentBuild = spawnSync('go', ['build', '-o', agentBinary, './cmd/posternproxy-agent'], {
  cwd: root, stdio: ['ignore', 'ignore', 'pipe'],
})
if (agentBuild.status !== 0) {
  process.stderr.write(agentBuild.stderr ?? 'Agent build failed')
  rmSync(data, { recursive: true, force: true })
  process.exit(1)
}
mkdirSync(dirname(metadata), { recursive: true })
writeFileSync(metadata, JSON.stringify({ agentBinary }), { mode: 0o600 })

function launch(command, args, options) {
  const child = spawn(command, args, { stdio: 'ignore', ...options })
  children.push(child)
  child.once('error', () => stop(1))
  child.once('exit', (code) => { if (!stopping) stop(code || 1) })
  return child
}

launch(binary, [], {
  cwd: root,
  env: {
    ...process.env,
    POSTERNPROXY_DATA_DIR: data,
    POSTERNPROXY_DB_PATH: join(data, 'test.db'),
    POSTERNPROXY_CERT_DIR: join(data, 'certs'),
    POSTERNPROXY_CERT_RUNTIME_DIR: join(data, 'runtime'),
    POSTERNPROXY_LISTEN: '127.0.0.1:18181',
    POSTERNPROXY_CONTROLLER_URL: 'http://127.0.0.1:18181',
    POSTERNPROXY_CADDY_API: 'http://127.0.0.1:1',
    POSTERNPROXY_JWT_SECRET: 'disposable-browser-test-secret-never-use-in-deployment',
    POSTERNPROXY_ADMIN_EMAIL: 'admin@e2e.test',
    POSTERNPROXY_ADMIN_PASSWORD: 'Disposable-browser-test-password',
    POSTERNPROXY_BEHIND_TLS_PROXY: 'false',
    POSTERNPROXY_TRUSTED_PROXIES: '127.0.0.1/32,::1/128',
    // The suite logs in per test; relax the per-IP login throttle (default 5/min)
    // so sequential scenarios are not rate-limited. Production keeps the default.
    POSTERNPROXY_LOGIN_RATE_LIMIT: '1000',
  },
})

const secureData = join(data, 'https')
mkdirSync(secureData)
launch(binary, [], {
  cwd: root,
  env: {
    ...process.env,
    POSTERNPROXY_DATA_DIR: secureData,
    POSTERNPROXY_DB_PATH: join(secureData, 'test.db'),
    POSTERNPROXY_CERT_DIR: join(secureData, 'certs'),
    POSTERNPROXY_CERT_RUNTIME_DIR: join(secureData, 'runtime'),
    POSTERNPROXY_LISTEN: '127.0.0.1:18185',
    POSTERNPROXY_CONTROLLER_URL: 'https://cert-upload.e2e.test:18184',
    POSTERNPROXY_CADDY_API: 'http://127.0.0.1:1',
    POSTERNPROXY_JWT_SECRET: 'disposable-secure-browser-fixture',
    POSTERNPROXY_ADMIN_EMAIL: 'admin@e2e.test',
    POSTERNPROXY_ADMIN_PASSWORD: 'Disposable-browser-test-password',
    POSTERNPROXY_BEHIND_TLS_PROXY: 'true',
    POSTERNPROXY_LOGIN_RATE_LIMIT: '1000',
  },
})

const fixture = readFileSync(join(frontend, 'e2e', 'testdata', 'cert-fixture.ts'), 'utf8')
function fixturePEM(name) {
  const match = fixture.match(new RegExp('export const ' + name + ' = `([^`]+)`'))
  if (!match) throw new Error('Missing browser TLS fixture')
  return match[1]
}
const secureProxy = createHTTPSServer({ cert: fixturePEM('certPem'), key: fixturePEM('keyPem') }, (req, res) => {
  if (req.headers.host?.startsWith('attacker.test:')) {
    res.writeHead(200, { 'Content-Type': 'text/html' })
    res.end('<!doctype html><title>Cross-site browser fixture</title>')
    return
  }
  const port = req.url?.startsWith('/api/') ? 18185 : 18180
  const upstream = httpRequest({ hostname: '127.0.0.1', port, path: req.url, method: req.method,
    headers: { ...req.headers, host: `127.0.0.1:${port}`, 'x-forwarded-proto': 'https' } }, (reply) => {
    res.writeHead(reply.statusCode ?? 502, reply.headers)
    reply.pipe(res)
  })
  upstream.on('error', () => { res.writeHead(502); res.end() })
  req.on('aborted', () => upstream.destroy())
  req.pipe(upstream)
})
servers.push(secureProxy)
secureProxy.on('error', () => stop(1))
secureProxy.listen(18184, '127.0.0.1')

// Wait for the isolated controller before exposing the frontend to Playwright.
for (let attempt = 0; ; attempt++) {
  try {
    const response = await fetch('http://127.0.0.1:18181/api/version')
    if (response.ok) break
  } catch { /* The controller has not bound its port yet. */ }
  if (stopping) break
  if (attempt >= 100) { stop(1); break }
  await new Promise((done) => setTimeout(done, 100))
}
if (!stopping) {
  launch(process.execPath, ['node_modules/vite/bin/vite.js', '--host', '127.0.0.1', '--port', '18180', '--strictPort'], {
    cwd: frontend,
    env: { ...process.env, POSTERNPROXY_DEV_API: 'http://127.0.0.1:18181' },
  })
}
process.on('SIGINT', () => stop())
process.on('SIGTERM', () => stop())
