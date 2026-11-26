# Admin branding and default page themes

Status: completed locally. Requested by the owner in this chat. Not deployed.

## Scope and acceptance

- Display the supplied logo in the admin sidebar and sign-in screen; use the
  dark variant against the sidebar's #0f172a background. Preserve collapse,
  navigation, sign-in behavior, and accessible branding.
- Default Caddy 502/503 and parked-domain pages use the supplied logo and follow
  the visitor's system light/dark preference, including changes while open.
  Use CSS media queries with light fallback; no theme storage or scripts needed.
- Pages remain self-contained during upstream/controller outages and for remote
  agents. Custom HTML, status codes, routing, placeholders and escaping remain
  unchanged. No auth, provisioning, licensing or connectivity changes.

## Work

1. Independently derive acceptance tests before implementation, run focused red.
2. Prepare compact shared runtime logo assets; embed them in Caddy page HTML and
   bundle the same assets with Vite. Add shared branding and theme styles.
3. Update docs/changelog. Run focused Go tests, full Go tests/build, frontend
   type-check/build and browser checks for themes, branding and responsive layout.
4. Record actual results and move this plan to completed when finished.

## Context

Reviewed AGENTS.md, agent804-adoption.md, beta-next-fixes.md, current Caddy page
templates and frontend layout. Existing uncommitted changes are preserved.
The spec skill calls for independent tests; a read-only test author supplies a
patch to the parent, maintaining sequential writes in the shared checkout.

## Delivered and verified (2026-11-24)

- Shared 320px PNG runtime assets in internal/branding; BrandLogo.vue uses the
  dark variant in expanded/collapsed sidebar and light variant on sign-in.
- Both default Caddy page renderers share embedded pages.css and picture markup.
  CSS and picture media queries follow the system preference without scripts;
  dark panels use #0f172a. Custom bodies and response routing are unchanged.
- Docs and Unreleased changelog updated; frontend production output copied into
  internal/ui/dist for controller embedding. Runtime asset provenance recorded
  in internal/branding/README.md. No licensing or security contract changes.
- Independent Go acceptance tests first failed for missing PNGs/dark media rules;
  escaping/placeholder preservation was already passing. Focused tests now pass,
  including existing custom override and parked status tests.
- Full `go test ./...` and `go build ./...` passed with local socket access.
  Initial sandbox run failed on socket restrictions; concurrent browser fixture
  cleanup also raced package discovery, so subsequent suites ran sequentially.
- Frontend `npm run type-check`, `npm run build`, `npm run lint:check` pass.
- All six Chromium tests pass. Theme fixtures are actual generated Caddy HTML;
  all seven page/status variants switch light/dark/light without reload, load
  their logos with JavaScript disabled and no network requests, and fit 1280,
  390 and 320px widths. Sign-in, sidebar collapse/navigation and existing auth
  flows pass. Fixed the test subprocess's output buffer for embedded PNG data.
- Inspected sign-in/sidebar and proxy/parked light/dark screenshots. No new ADR
  was needed. Existing Caddy deployment/agent transport integration was not rerun;
  the response route/status handling is unchanged and covered by existing tests.

After deploying a rebuilt controller, synchronize existing host configuration to
replace previously applied default page HTML. No deployment performed here.
