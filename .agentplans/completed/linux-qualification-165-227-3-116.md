# Linux qualification on supplied VM

Status: complete, 2026-11-25 (America/Indianapolis). User supplied SSH access to root@165.227.3.116
for Linux testing following the Windows installer test. Initial inspection found
Ubuntu 24.04.5 x86_64, 458 MiB RAM/no swap, 6.6 GiB free disk, no PosternProxy/Caddy
services/config/data, and only SSH/DNS listeners. First-seen ED25519 host key is
pinned in the workspace task directory; credentials are not recorded here.

Build matching current linux/amd64 controller/agent locally, stage pinned Caddy
and checksums, upload a private bundle and qualification scripts. Test the actual
native installer including required packages, fresh login, retained configuration
and secret on reinstall/restart, native agent enrollment and SSH provisioning,
applied readiness, custom TLS and runtime certificate permissions. Preserve test
evidence and leave services stopped after qualification unless user directs
otherwise. Do not change SSH access or firewall rules. Docker qualification can
use an existing Docker installation; this VM cannot establish native ARM64
qualification. Fix reproduced issues and record the exact scope/results.

User subsequently authorized installing Docker/Compose and temporary swap, and
requested ARM emulation as well. Install packaged Docker/Compose/QEMU, run amd64
container checks and ARM64 emulated executable/container checks within available
disk/memory. Keep emulated results distinct from native ARM hardware evidence.
Remove temporary swap after testing; do not publish images.

Native initial installation, package step, fresh login/private files/bootstrap
cleanup, reinstall/config and credential retention, native agent enrollment and
actual SSH provisioning, applied revisions, verified TLS, Caddy group-read and
unrelated-user denial, agent/Caddy restarts passed. Pinned HTTP/rate/TCP, proxy
trust/upload/header/SSE/WebSocket, UDP/combined transport, passthrough, portable
TLS recovery and two-agent restart convergence passed. SSH authorized_keys and
iptables rules matched their original hashes/content. Services stopped/disabled.

Additional progress:
- All compiled Linux cmd/internal package unit test executables pass natively;
  package-relative testdata had to be staged and each executable run in its source
  directory. No Go toolchain was installed on the VM.
- Docker 29.1.3, Compose 2.40.3, Buildx 0.30.1, QEMU 8.2.2 installed from Ubuntu
  repositories. A private 1 GiB temporary swap file is active during builds.
- ARM64 controller/agent/pinned Caddy run under QEMU binfmt. Actual controller
  install, fresh login, reinstall, bootstrap cleanup, agent application, custom
  TLS, permissions and restarts pass. QEMU JIT needs a temporary runtime-only
  MemoryDenyWriteExecute=no controller drop-in; arm-run.py removes it in finally.
  Production units retain their hardening. This does not qualify native ARM
  systemd hardening/performance. SSH provisioning was tested natively on amd64.
- ARM64 pinned traffic, rate limits, trust/header/upload/SSE/WebSocket checks,
  UDP and passthrough pass. The generic traffic fixture's 5-second startup polling
  was shorter than its existing 30-second context; use that bounded context for
  startup to permit emulation. Payload/routing assertions are unchanged.
- ARM service fixtures hit short deadlines with an emulated test driver during a
  concurrent compiler build. Retesting via native driver with ARM data-plane/agent
  executables and the builder paused isolates emulator overhead from contention.
- Production Docker source build exposed missing shared branding assets in its
  frontend stage. Fixed WORKDIR/COPY paths in docker/Dockerfile; rebuilding.
- ARM64 container runtime checks use production Dockerfiles with explicit named
  contexts supplying verified cross-built ARM binaries, avoiding a compiler under
  QEMU. This tests ARM images/Compose/runtime, not native ARM Docker source builds.

An approval-review usage limit interrupted monitoring once; running remote jobs
continued. User explicitly requested continuation. Do not leave swap/drop-ins or
application services running when qualification concludes.

ARM service recovery/reconciliation passed with a native fixture driver operating
ARM64 Caddy/agent binaries, compiler paused, fixture shim request timeouts raised
from 1/2 to 5 seconds and eventual-state polling bounded at 30 seconds. Production
timeouts and payload assertions are unchanged. Native Windows pinned traffic and
both service fixtures also pass after these changes. Full local Go test/build/vet
passes. The temporary QEMU service drop-in was removed; MemoryDenyWriteExecute=yes
was observed afterwards. Native amd64 binary restoration is running.

Native amd64 binary restoration completed with preserved config hash and ELF
architecture checks. ARM64 Docker runtime images built from the production
Dockerfiles with named cross-build component contexts and passed complete Compose
qualification: version/login, enrollment/applied revision, HTTP, custom TLS,
restart, Caddy failure/container exit, supervisor recovery, controller replacement
and persisted configuration/secrets/certificates. Image identities were retained
locally and remotely; the two test image tags were removed to reclaim disk.

Live Docker fixes: quote the JWT-secret interpolation error text in Compose
(unquoted colon made the YAML invalid), include branding in the frontend stage,
and refresh dynamic published ports in the smoke test after agent restart/start.
A separate throwaway port probe confirmed Docker assigned a new ephemeral host
port on restart. Docker fixture API timeout allows emulation overhead.

Broad default-builder cache pruning was rejected by automatic review and was not
executed. Safer cleanup revalidated and removed only explicit inspected cache IDs
with exact descriptions, parent IDs/timestamps and unused status tied to this
qualification. The native source builder is resumed. Pinned Caddy source build
passes; corrected frontend/app source stages and native container smoke pending.

The corrected frontend resolves branding and passes type-check/bundle after an
explicit build-stage NODE_OPTIONS heap limit (the 458 MiB VM made Node select an
insufficient 256 MiB default). Dockerfile ARG defaults to a 512 MiB heap and stays
out of the runtime image. Failed build cache eviction forced another Caddy source
compile; the source job remains in progress. Temporary swap was safely reduced
from 1 GiB to 512 MiB to reclaim disk. Remove it after the builder has stopped.

## Final result

Both amd64 production Dockerfiles built successfully from source. The complete
native Compose suite passed: version/login, authenticated outbound agent and
applied revision, HTTP and verified custom TLS, restart, Caddy-exit/container-exit
supervision, supervisor restart, replacement controller, and persisted config,
secrets and certificates. The same suite passed with the ARM64 runtime images
under QEMU, using cross-built component contexts as described above.

Node required its explicit 512 MiB build heap. The Go SQLite compiler required
temporary swap on this 458 MiB host; the peak configured swap was 1,344 MiB across
three task-owned files. Independent cleanup guards removed all three after the
source job. No temporary swap remains.

Final independent inspection: native controller/agent/Caddy services inactive and
disabled; no running Docker containers; source builder stopped; no app listeners
on 80/443/81/2019/8443/18181; MemoryDenyWriteExecute=yes and QEMU drop-in absent.
Native amd64 installed executables were restored after emulation. SSH remains
available; Docker/containerd and QEMU remain installed. Retained test data and
native image tags remain; no image/release was published and no commit was made.
Disk free: 4.2 GiB of 8.7 GiB. Native system listeners are SSH/DNS plus containerd
loopback, with no public application listener.

Local artifact metadata and cleanup attestations are under
`tmp/linux-qualification-165-227-3-116/`. Qualified image identities:
- amd64 controller: sha256:52d45e84043cb4cf6185931ad0c7680701a878d6c66827345e50de47379d8eca
- amd64 agent: sha256:6618451f6e83ba783c13ba0c0fade0a84d04d7e7b5d1351701213f6f85739bb1
- ARM64 controller: sha256:a06541cf142abcf98099be4707073203a6debf05b1158b684eb854456a220625
- ARM64 agent: sha256:264a88e76ae47c83c5d9b84af249a9929bc8bb184e63f8a948f2513987c0fbd3

Remaining release evidence is distinct: exact candidate CI (including race and
native ARM runner/source builds), published assets/downloads, and the agreed beta
soak/manual domain/ACME checks. Emulation does not qualify native ARM performance
or the hardened systemd policy. This development qualification is not release
approval.
