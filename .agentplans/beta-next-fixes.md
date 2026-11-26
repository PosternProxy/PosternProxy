# Next beta fixes

Status: implementation and local validation complete; target release qualification remains open. Authorized by the user's request to fix the remaining release
issues after the documentation review. Extends the existing D/E/F/G plans rather
than replacing their release gates. Preserve all current uncommitted work.

## Baseline and acceptance

The user reports that the new Linux controller `install` command succeeded on a
VM. Record this as successful manual installation evidence, not remote-agent,
certificate, upgrade, or recovery qualification. No VM OS/build identifiers or
remote access have been supplied.

1. Certificates: invalid PEM, key mismatch, domain mismatch and invalid validity
   periods fail before DB/files change. Persist private material encrypted using
   the stable controller key. Only certificates referenced by enabled TLS hosts
   on the target server enter its Caddy config/payload. Missing required material
   fails visibly. Stage complete immutable runtime pairs; Caddy receives narrow
   group read access, never controller-data access. Rejected applies preserve
   the previous usable runtime files; successful applies retire stale managed
   files. Agent and controller reuse shared helpers and validate paths/symlinks.
2. Provisioning: verify local agent/Caddy architecture before host changes;
   no placeholders or on-target compilation. Pass identity/credentials through
   protected files, not interpolated shell or logs. Share the native installer
   between controller and agent. Preserve host keys, data, firewall and actual
   SSH access. Successful provisioning requires an authenticated agent and a
   correlated successful desired revision, not script exit. Manual enrollment
   must be explicit, admin-only, audited, and avoid bootstrap-URL placeholders.
3. Settings/audit: runtime settings either apply to the actual services or return
   a clear refusal; privileged operations have attributed, redacted audit events.
   Restrict audit visibility to admins. Validate stream conflicts at the API
   boundary and enforce advertised access controls consistently.
4. Recovery: add a versioned passphrase-encrypted portable format including
   certificate bytes and credential-free server descriptors, explicit safe server
   mapping/re-enrollment, complete validation before destructive changes, and
   recoverable staging/transaction ordering. Preserve legacy partial export as
   explicitly labeled compatibility behavior; no silent credential restoration.
   Restore must distinguish data commit from configuration application.
5. Qualification: focused rejection/security tests first, full Go/frontend
   checks, actual Caddy traffic/TLS and two-agent/restart/restore acceptance,
   rebuild the Linux bundle, and reconcile docs. Fresh VM/container/published
   release checks remain unclaimed until actually exercised.

## Increments

| Increment | Outcome | Dependencies | Status |
|---|---|---|---|
| E1 | Validated encrypted certificate storage and target isolation | none | implemented; regression green |
| E2 | Safe runtime staging/permissions and apply-scoped cleanup | E1 | implemented; regression green |
| D1 | Agent install and safe SSH provisioning, authenticated/applied readiness | E2 | implemented; target VM qualification pending |
| A1 | Runtime settings, audit gaps, boundary validation | E1/E2 contracts | implemented; regression green |
| F1 | Encrypted portable export and clean restore | E1/E2, D1 enrollment | implemented; local live recovery green |
| G1 | Combined acceptance, docs and rebuilt artifacts | all | local checks/build complete; external release gates open |

Certificate storage uses encrypted source files and immutable plaintext runtime
pairs readable only by the service owner and a dedicated Caddy certificate group.
Plaintext runtime PEM is necessary for Caddy's file loader; it is kept outside
controller data, with tight permissions. No license, proxy data-plane, outbound
agent connectivity or existing database migration history changes are authorized.

## Evidence

2026-11-24 local evidence:

- Certificate rejection/isolation tests were independently drafted and failed
  before implementation. They now pass. Review findings on immutable migration,
  PEM framing/EKU, runtime group/umask, directory fsync and expired legacy
  material were corrected and covered by regression tests.
- Provisioning validation tests failed before the shared input contract existed;
  targeted installer/provisioner/API suites now pass. Added readiness, manual
  enrollment/rotation, runtime settings and stream-conflict regression tests.
- Portable recovery missing-interface RED captured before implementation. Tests
  now cover cross-key certificate recovery, explicit server mapping, unenrolled
  creation, wrong passphrase/corruption rejection and legacy format allowlists.
- Full Go tests and go build/go vet passed. Frontend type-check/build/lint passed.
  Hugo builds 61 pages; existing shortcode deprecation warnings remain.
- Actual pinned Caddy serves uploaded custom TLS and restores it from an
  encrypted archive into a fresh database with a different encryption key.
  Existing pinned traffic/rate-limit/TCP and two-agent restart tests pass.
- govulncheck exposed the reachable Gorilla masking advisory; upgraded to its
  exact fixed revision. The rerun reports zero reachable vulnerabilities (the subsequent network/crypto updates also clear the module-only findings).

- Three Chromium smoke tests pass, including one-time manual enrollment and
  active read-only settings. Both installer wrappers and the Caddy shell contract
  tests pass.
- Artifact qualification caught Caddy using Go 1.25.1 despite the application
  toolchain. The contract now pins Go 1.26.11, Caddy 2.11.9 and official patched
  crypto/network/compression/OpenTelemetry dependencies. The exact generated
  Caddy source scan reports no vulnerabilities. App source scan also reports none.
- Version stamping had been overwritten by function-based package initialization;
  both binaries now preserve linker values. Native stamped version commands pass.

The rebuilt development archive contains exactly controller, agent, Caddy and
checksums. All three are static Linux/amd64 executables built with Go 1.26.11;
Caddy core/plugin/security replacement metadata is verified. Fresh extraction
matches the manifest and built files. Source and binary vulnerability scans
report no findings. Artifact hashes and sizes are recorded in
[artifact evidence](beta-next-fixes-artifacts.json).

Fresh Linux agent/systemd permissions,
SSH provisioning, containers, historical upgrades and published downloads remain
external release gates. The user's successful controller VM install is retained
as evidence. Broader beta-exit plans remain active until their own gates close.


Completion boundary: requested code/docs fixes and local build qualification are
finished. No tag, commit, push or publication was performed. Existing broader
D/E/F/G release plans remain active for target-environment acceptance. No Docker
CLI is installed on this host, and no SSH target access was supplied. The user's
successful controller VM installation is not being reclassified as untested.


## Windows build line-ending regression

The user's replacement Linux bundle failed on the VM before installation with
Bash carriage-return errors in the embedded Caddy script. The previous source
write introduced CRLF despite the repository attributes. A regression test
reproduced the failure. The shared Go contract now normalizes CRLF and bare CR
to LF before either native installer executes it, independently of checkout
settings. The shell source is restored to LF. Rebuild both Go binaries and the
archive, retain the verified Caddy binary, and update artifact evidence. The
latest VM run is a failed pre-install verification; the earlier successful
controller install remains separate evidence.


Line-ending fix completion: full Go tests, build and vet pass. Regression coverage
checks the normalized embedded script, CRLF/bare-CR inputs, Bash parsing and
execution of the embedded verifier against a version/module fixture. Both Linux
application binaries were rebuilt, archive checksums regenerated and verified
from a fresh extraction. Binary vulnerability scans report no findings. Updated
archive SHA-256: 5b3a0ab9cab1fa87bc40496d4ff1375dc36c031ae1fa8512e0a99b7dd80027b1.
The corrected bundle still needs the user's Linux VM retry; do not count the
previous failed verification as a successful install.


Initial-login follow-up: native installation now prints the fresh admin login
after readiness using a transient private runtime env file, and preserves existing
passwords on reinstall. See [completed password-output work](completed/install-password-output.md).
The latest archive hashes are in beta-next-fixes-artifacts.json; earlier hashes
above identify previous builds rather than the latest password-output bundle.


Parked-domain follow-up: added HTTP-only mode and local TLS diagnostics across
proxy, redirect and parked host lists. See
[completed parked/TLS work](completed/parked-http-tls-status.md). Current artifact
hashes include this change and the initial-login-output change.

November 24 qualification supersedes the earlier no-SSH-access and VM-retry
limitations above: the user supplied a disposable Ubuntu amd64 target. Current
archive installation, fresh initialization, native agent and actual SSH
provisioning, TLS permission/restart checks and Linux traffic/recovery fixtures
passed. The original controller data was restored and remains healthy. See
[dated evidence and remaining gates](beta-security-qualification-2026-11-24.md)
and the updated artifact JSON for the current archive identity.
