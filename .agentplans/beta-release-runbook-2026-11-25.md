# Release v0.1.5-beta -- owner runbook

Updated 2026-11-26. Target is **v0.1.5-beta**. The earlier v0.1.4-beta tag
remains on its original candidate; its prepared, unpublished changes are included
in v0.1.5-beta. Do not move the old tag.

The owner reports the updated manual Release candidates run is all green;
Package was correctly skipped because that run was manual. Commit and push this
metadata update before tagging. The tag-triggered run must qualify the exact
version-stamped artifacts and create the draft. No release has been published
by this preparation.
Use the steps in order. Stop on a failed command or failed required Actions job.

## What is complete

- Native Linux amd64 retained; Linux arm64, Windows amd64 services and Docker
  amd64/arm64 implemented and documented. Every native bundle includes agent and Caddy.
- Development qualification: Windows SCM lifecycle/custom TLS/removal; Ubuntu
  amd64 native install/reinstall/actual SSH provisioning/custom TLS; source-built
  amd64 Docker/Compose; ARM64 native installer and container runtime under QEMU.
- Fresh source reviews of authority, lifecycle, installer and release changes.
  Two new findings fixed and independently reviewed: ongoing agent credential
  revocation and consistent backup snapshots.
- Full Windows Go tests, build and vet; frontend type/lint/build; all 15 browser
  tests including HTTPS; actionlint; Hugo (63 pages); Go vulnerability scan clean.
- Final affected API/services suites pass natively on the supplied Ubuntu VM.
  Database migration/baseline suites also pass with their checked-in fixture.
- Release draft creation now depends on complete quality CI and native platform/
  Docker qualification. CI can also run on beta-exit, codex branches and manually.

Prior platform evidence predates the final small authority/backup fixes; final
candidate workflows rebuild and requalify the actual release source. Native ARM
hardware/source image and Linux race checks remain mandatory in those workflows.

## 1. Review and commit the prepared tree

From PowerShell in `C:\Users\Michael\Repos\PosternProxy`:

```powershell
git branch --show-current
git status --short
git diff --check
git diff --stat
```

Review the changes, including the new files shown by status. The tree contains the
previous Windows/Linux/Docker work as well as these final fixes. `tmp/`, binaries,
node_modules and generated output are ignored; do not force-add them. Keep secrets
and private qualification logs out of the commit. Review `.agentplans/` as project
working evidence. The changelog is already prepared under
`## [0.1.5-beta] 2026-11-26`, with an empty Unreleased section.

```powershell
git add -A
git diff --cached --check
git diff --cached --stat
git commit -m "Prepare v0.1.5-beta platform support and release qualification"
git push
```

If your release date changes, use the actual publication date in the changelog.

## 2. Merge the reviewed candidate and require green CI

The implementation is already merged to master. Commit and push the release
metadata on the intended release branch. If that is a separate branch, open a
pull request to master and merge after review and green CI. Otherwise wait for
the master CI run on the metadata commit. Do not bypass failing checks or force
push.

```powershell
git fetch origin master --tags
git switch master
git pull --ff-only origin master
git status --short
git log -1 --format="%H %s"
```

The tree must be clean and HEAD must be the reviewed merge/candidate. If a local
master cannot fast-forward (for example a squash merge), switch to a clean branch
at `origin/master`; do not reset or discard local work. Optional: on GitHub Actions,
run **Release candidates** manually on master for pre-tag qualification. Manual
runs qualify and upload artifacts but do not create a release. The workflow must
first be on the default branch to expose its Run workflow button.

## 3. Create the exact beta tag

```powershell
git ls-remote --tags origin refs/tags/v0.1.5-beta
git tag -a v0.1.5-beta -m "PosternProxy v0.1.5-beta"
git push origin v0.1.5-beta
```

The first command should show no existing tag. If the name already exists, choose
a new version and update notes/changelog; do not move an existing published tag.
The tag push starts **Release candidates**. Wait for all jobs:

- Quality: Go test/race/vet/vulnerability/module checks, frontend/browser/audit,
  pinned Caddy traffic/recovery and documentation.
- Windows: actual services install/reinstall/agent/TLS/removal.
- Native Linux **amd64 and arm64**: tests/race/vet/security, source builds,
  traffic/recovery and controller/agent installer harnesses.
- Docker amd64 and arm64: production source images and Compose qualification.
- Package: succeeds only after all above, writes checksums and creates a **draft**.

If a runner is unavailable or a job fails, fix it and qualify a new immutable
candidate/tag. Do not publish a partial draft or treat QEMU as native ARM CI.

## 4. Review the draft and its exact downloadable bundles

On GitHub Releases open the generated draft. Confirm tag/commit and **prerelease**;
do not mark it latest stable. Replace generated notes with the prepared content in
`beta-v0.1.5-release-notes.md` beside this runbook. Link the installation and beta
limitations pages at your documentation URL.

Required assets:

This list describes the assets attached to the GitHub release draft, not the
local `bin/` directory. The local PowerShell build produces one target at a time
in `bin/`. The tagged Release candidates workflow builds all targets, packages
them in runner-local `release/` directories, and attaches the combined assets to
the draft. Do not upload stale development files from `bin/` as release assets.

- `posternproxy-linux-amd64.tar.gz`, `posternproxy-linux-arm64.tar.gz`.
- `posternproxy-windows-amd64.zip`.
- Loose controller, agent and Caddy executables for each of those three targets.
- Release-level `checksums.txt` covering every downloadable executable/archive.

Use your signed-in browser to download the draft bundle and manifest. Verify the
archive hash, extract into an empty directory, then verify the archive's enclosed
binary manifest. Draft URLs require authentication; this is distinct from public
installation. Confirm version output and clean install from the exact bundle on
a disposable target. Candidate CI already covers each native target's bundle
components; also inspect archive contents/names/manifest and try the documented
extract/install commands. Do not run destructive qualification scripts on a real
host with data you need.

Linux example in the directory containing the downloaded archive/manifest:

```bash
sha256sum --ignore-missing -c checksums.txt
mkdir posternproxy-install
tar -xzf posternproxy-linux-amd64.tar.gz -C posternproxy-install
cd posternproxy-install
sha256sum -c checksums.txt
chmod +x posternproxy posternproxy-agent caddy
./posternproxy version
sudo ./posternproxy install --assets-dir .
```

Use the arm64 archive on ARM. Follow the printed bootstrap login privately,
change the password, and confirm a host works. Limit management access to your
trusted network/SSH tunnel; ports 80/443 are for public traffic. Do not paste
bootstrap passwords into public logs or release notes.

Windows archive verification (PowerShell, before extracting):

```powershell
$asset = 'posternproxy-windows-amd64.zip'
$line = Get-Content ./checksums.txt | Where-Object { ($_ -split '\s+', 2)[1] -eq $asset }
if (@($line).Count -ne 1) { throw 'Archive is missing from checksum manifest' }
$expected = ($line -split '\s+', 2)[0]
if ((Get-FileHash $asset -Algorithm SHA256).Hash -ne $expected) { throw 'Checksum mismatch' }
Expand-Archive -LiteralPath $asset -DestinationPath ./posternproxy-install
```

Verify each enclosed binary against its enclosed manifest the same way, then
follow the Windows guide's elevated `./posternproxy.exe install --assets-dir .`.
If a draft smoke fails, leave it unpublished and resolve the failure.

## 5. Publish the GitHub prerelease, then finish Docker publication

Publish the reviewed **prerelease through the GitHub UI**. This starts
**Publish qualified Docker images**. Wait for both architecture jobs and the
manifest job to succeed before announcing Docker availability. This workflow
rebuilds and tests the tagged source before pushing. A workflow publishing with
its own GITHUB_TOKEN would not trigger this downstream release event, so retain
the owner UI publication step.

In GitHub Packages, ensure **both** `posternproxy` and `posternproxy-agent` are
public and grant this repository Actions access if required. Normal publication
uses GITHUB_TOKEN; do not add a personal token unnecessarily. Public pulls must
work without a login. Do not label `latest` for this beta; workflows update `beta`
and `v0.1.5-beta` tags.

## 6. Check public download/install and anonymous Docker pulls

From a logged-out browser or clean Linux directory:

```bash
curl -fLO https://github.com/PosternProxy/PosternProxy/releases/download/v0.1.5-beta/posternproxy-linux-amd64.tar.gz
curl -fLO https://github.com/PosternProxy/PosternProxy/releases/download/v0.1.5-beta/checksums.txt
sha256sum --ignore-missing -c checksums.txt
```

Repeat the extraction/install smoke from step 4 against the public bundle. Test
all three public archive links; check that the individual-binary installer can
fetch its matching agent/Caddy from this same tag. Archive installation always
uses `--assets-dir .`. Confirm controller and agent both report v0.1.5-beta.

Anonymous Docker tests on a Docker host (use an empty temporary auth directory):

```bash
docker_auth=$(mktemp -d)
DOCKER_CONFIG="$docker_auth" docker pull ghcr.io/posternproxy/posternproxy:v0.1.5-beta
DOCKER_CONFIG="$docker_auth" docker pull ghcr.io/posternproxy/posternproxy-agent:v0.1.5-beta
DOCKER_CONFIG="$docker_auth" docker manifest inspect ghcr.io/posternproxy/posternproxy:v0.1.5-beta
DOCKER_CONFIG="$docker_auth" docker manifest inspect ghcr.io/posternproxy/posternproxy-agent:v0.1.5-beta
```

Both manifests must contain linux/amd64 and linux/arm64. Follow the Docker guide
for a Compose start/login/outbound-agent/restart smoke using these versioned
images. If publication fails, leave the announcement pending and rerun the failed
job after fixing permissions; do not claim container availability prematurely.

## 7. Your real-domain check, then announce

Use a disposable real DNS name pointing at the test server, with inbound 80/443
available. Create an automatic HTTPS host in the UI; verify a trusted certificate
in a browser and with `curl -v https://YOUR_TEST_DOMAIN/` without disabling TLS
verification. Confirm the TLS status/diagnostics and inspect Caddy logs if issuance
fails. Try an HTTP-only parked domain and a remote-agent host too. Check login,
host edit/apply status, restart persistence and one encrypted backup/restore.
Custom TLS and emulated-domain tests do not prove public Let's Encrypt issuance.
Long renewal/outage/multi-server workload soak belongs during this beta before
production. Do not wait for that entire soak to gather beta feedback.

After these pass, publish/refresh docs for this commit and announce v0.1.5-beta.
Record the release URL, commit, candidate Actions URLs, artifact hashes, Docker
digests and manual-check results in this directory. Reconcile the blocker ledger
only after exact-candidate Linux CI is green. If any critical gate fails, keep
release/announcement on hold rather than relabeling it as tested.
