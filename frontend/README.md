# PosternProxy Frontend

Vue 3 + TypeScript + Vite SPA for the PosternProxy controller. Built output
(`dist/`) is embedded into the Go controller binary via `go:embed` and served
at the controller's root path — there is no standalone deployment.

## Stack

- **Vue 3** (Composition API, `<script setup>`)
- **TypeScript** (strict)
- **Vite** dev server / build
- **Pinia** for auth and server-context state
- **vue-router** for routing
- **Tailwind CSS** for styling
- **Axios** for the API client (`src/api/`)
- **ESLint** (flat config) + **Prettier**

## Project layout

```
src/
  api/          API clients — one per resource, plus createResourceClient factory
  assets/       Static assets bundled by Vite
  components/   Shared components (DeleteConfirmDialog, ServerPicker, etc.)
  composables/  Reusable composition functions (useResourceList, etc.)
  router/       vue-router config
  stores/       Pinia stores (auth, servers, server context)
  types/        Shared TS types mirroring backend models
  utils/        Shared helpers (errors, formatting)
  views/        Route-level views — one folder per major resource
```

## Prerequisites

- Node `^20.19.0 || >=22.12.0`
- A running controller backend on `http://localhost:81` (default `make dev-backend`)
- For the HTTP development UI, set `POSTERNPROXY_BEHIND_TLS_PROXY=false` on the
  backend and allow `http://localhost:3000` with `POSTERNPROXY_ALLOWED_ORIGINS`

## Development

```sh
npm install
npm run dev          # Vite dev server with HMR, proxies /api and /ws → :81
```

Vite serves on `http://localhost:3000`. The dev server proxies `/api/*`
and `/ws/*` to the Go controller — start the controller first (or run
`make dev` from the repo root to print the two-terminal instructions).
Set `POSTERNPROXY_DEV_API` when starting Vite to override the backend target.

## Build

```sh
npm run build        # type-check + production build → dist/
npm run build-only   # production build without type-check
npm run preview      # serve dist/ for local verification
```

The controller embeds `internal/ui/dist/`, not `frontend/dist/` directly. The
production scripts copy the output into that directory before compiling Go.
Use `bash scripts/build.sh` from the repository root, or `scripts/build.ps1` on
Windows, to build the embedded UI, controller, agent, Caddy, and checksums
together. A bare `go build` embeds whatever files are already in `internal/ui/dist/`.

## Lint and format

```sh
npm run lint           # ESLint with --fix
npm run lint:check     # ESLint without modifying files (CI)
npm run format         # Prettier write
npm run format:check   # Optional Prettier check
npm run type-check     # vue-tsc, no emit
```

## Browser acceptance

Run `npx playwright install chromium` once, then `npm run test:e2e`. The harness
builds isolated controller/agent binaries and uses temporary databases. Ports
18180, 18181, 18184 and 18185 must be free. No existing service is reused.
The HTTPS project uses a disposable self-signed fixture with Chromium hostname
mapping, so the system hosts file and trust store are unchanged. Tests cover
encrypted recovery, applied agent readiness and HTTPS cookies/CSRF in addition
to normal management workflows. `npm run type-check` also checks the test files.

## Recommended IDE setup

VS Code + the official [Vue (Volar)](https://marketplace.visualstudio.com/items?itemName=Vue.volar)
extension. Disable Vetur if present.
