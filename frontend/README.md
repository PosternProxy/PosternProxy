# PosternProxy Frontend

Vue 3 + TypeScript + Vite SPA for the PosternProxy controller. Built output
(`dist/`) is embedded into the Go controller binary via `go:embed` and served
at the controller's root path — there is no standalone deployment.

## Stack

- **Vue 3** (Composition API, `<script setup>`)
- **TypeScript** (strict)
- **Vite** dev server / build
- **Pinia** for state (auth, settings)
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
  stores/       Pinia stores (auth, settings)
  types/        Shared TS types mirroring backend models
  utils/        Shared helpers (errors, formatting)
  views/        Route-level views — one folder per major resource
```

## Prerequisites

- Node `^20.19.0 || >=22.12.0`
- A running controller backend on `http://localhost:8080` (default `make dev-backend`)

## Development

```sh
npm install
npm run dev          # Vite dev server with HMR, proxies /api → :8080
```

Vite serves on `http://localhost:5173`. The dev server proxies `/api/*`
and `/ws/*` to the Go controller — start the controller first (or run
`make dev` from the repo root to launch both in parallel).

## Build

```sh
npm run build        # type-check + production build → dist/
npm run build-only   # production build without type-check
npm run preview      # serve dist/ for local verification
```

`dist/` is consumed by the controller's `go:embed` at compile time. After
`npm run build`, running `go build ./cmd/posternproxy` ships the SPA inside
the binary.

## Lint and format

```sh
npm run lint           # ESLint with --fix
npm run format         # Prettier write
npm run format:check   # Prettier check (CI)
npm run type-check     # vue-tsc, no emit
```

## Recommended IDE setup

VS Code + the official [Vue (Volar)](https://marketplace.visualstudio.com/items?itemName=Vue.volar)
extension. Disable Vetur if present.
