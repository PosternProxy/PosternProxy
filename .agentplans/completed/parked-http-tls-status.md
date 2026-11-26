# Parked HTTP mode and host TLS status

Status: implementation and local validation complete. User reports missing non-HTTPS parked-domain mode and asks where
to see Let's Encrypt status/failures.

Acceptance: parked domains can explicitly use HTTP only, persisted through CRUD,
sync and backup. Existing records retain HTTPS behavior via a forward migration.
HTTP-only parked domains produce no HTTPS route/certificate distribution.
Local TLS checks cover proxy, redirect and parked hosts with resource-qualified
IDs and all domain aliases; the UI shows consistent status and expandable details.
Do not present TLS handshake observations as actual ACME order status. Give the
native Caddy journal command for exact issuer errors; remote status remains
explicitly unavailable rather than probing the controller for remote hosts.
Run meaningful Go/frontend/browser checks and rebuild the Linux bundle.


Evidence: missing-contract acceptance tests failed before implementation. Passing
checks cover HTTP-only persistence and backup round-trip, exclusion of unused
certificate material, no HTTPS route, preserved HTTPS defaults on migration,
resource-qualified TLS IDs and every alias. Full Go tests/build/vet, frontend
checks and four Chromium smoke tests pass. Live pinned-Caddy HTTP parked traffic,
custom TLS/recovery and two-agent restart acceptance pass. Hugo builds 61 pages.
The Linux archive was rebuilt, static ELF targets and checksums verified, and
all three binary vulnerability scans report no findings. Exact ACME issuer errors
remain in Caddy logs, with the command exposed in the UI and documentation. No
remote TLS probing or fresh VM installation is claimed.
