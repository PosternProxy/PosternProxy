import { spawn, spawnSync } from 'node:child_process'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontend = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const root = resolve(frontend, '..')
const tempRoot = resolve(tmpdir())
const data = mkdtempSync(join(tempRoot, 'posternproxy-e2e-'))
const binary = join(data, process.platform === 'win32' ? 'controller.exe' : 'controller')
const children = []
let stopping = false

function stop(code = 0) {
  if (stopping) return
  stopping = true
  for (const child of children) child.kill()
  // Windows may still hold the executable until its process exits.
  Promise.all(children.map((child) => new Promise((done) => {
    if (child.exitCode !== null || child.signalCode !== null || !child.pid) done()
    else child.once('close', done)
  }))).then(() => {
    if (data.startsWith(tempRoot + sep + 'posternproxy-e2e-')) {
      rmSync(data, { recursive: true, force: true, maxRetries: 5, retryDelay: 100 })
    }
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
    POSTERNPROXY_LISTEN: '127.0.0.1:18181',
    POSTERNPROXY_CADDY_API: 'http://127.0.0.1:1',
    POSTERNPROXY_JWT_SECRET: 'disposable-browser-test-secret-never-use-in-deployment',
    POSTERNPROXY_ADMIN_EMAIL: 'admin@e2e.test',
    POSTERNPROXY_ADMIN_PASSWORD: 'Disposable-browser-test-password',
    POSTERNPROXY_BEHIND_TLS_PROXY: 'false',
  },
})

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
