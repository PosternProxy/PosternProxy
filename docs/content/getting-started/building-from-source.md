---
title: Building from Source
weight: 5
---

# Building from Source

Pre-built binaries are published on the
[GitHub releases page](https://github.com/PosternProxy/PosternProxy/releases/) and
are the recommended way to install — see [Installation]({{< relref "/getting-started/installation" >}}).
Build from source only if you want to run unreleased changes or target an
architecture that isn't published.

Build on a development machine or CI, **not** on the production server — this keeps
the Go toolchain off production. The build produces three **Linux** binaries:

| Binary | Purpose |
|---|---|
| `posternproxy` | The controller (management UI + API, with the frontend embedded) |
| `posternproxy-agent` | The remote agent |
| `caddy` | A custom Caddy build with the L4 and rate-limit plugins |

**Requirements:** Go 1.26+ and Node.js 20.19+ or 22.12+ (with npm).

Both build scripts cross-compile for `linux/amd64` by default, so the output is
ready to copy straight to a Linux server regardless of which OS you build on. For
an ARM server (arm64 VPS, Raspberry Pi, etc.), target `arm64` as shown.

## On Linux or macOS

```bash
bash scripts/build.sh
# ARM target instead:
GOARCH=arm64 bash scripts/build.sh
```

Binaries are written to the **repository root**: `./posternproxy`,
`./posternproxy-agent`, `./caddy`.
The same directory contains `checksums.txt` for local installation.

## On Windows

Use PowerShell 7+:

```powershell
.\scripts\build.ps1
# ARM target instead:
.\scripts\build.ps1 -GoArch arm64
```

Binaries are written to the **`bin\` directory**: `bin\posternproxy`,
`bin\posternproxy-agent`, `bin\caddy`. They are Linux binaries (no `.exe` suffix,
because the target is Linux).
The directory also contains `checksums.txt`.

{{% hint info %}}
**Native Windows binaries**
The scripts target Linux by default because PosternProxy runs on Linux (the
installer is Debian/Ubuntu-only). You can produce native Windows binaries for
local testing with `.\scripts\build.ps1 -GoOS windows`, but there is no supported
Windows install path.
{{% /hint %}}

## What the build does

1. Builds the Vue frontend (`npm ci && npm run build`) and embeds it into the
   controller binary via `go:embed`.
2. Compiles the controller and agent as static binaries (`CGO_ENABLED=0`).
3. Builds Caddy with `xcaddy`, adding the `caddy-l4` and `caddy-ratelimit` plugins
   (installing `xcaddy` on the build host if needed).
4. Writes SHA-256 checksums for the three binaries.

The agent is included in every full build as a separate executable. It is not
embedded in the controller and does not need a second build for a remote node of
the same architecture. Use the same source version with a different `GoArch` /
`GOARCH` if that node needs a different CPU target.

The scripts write loose files; a separately supplied archive packages those
outputs together. To create a bundle from an output directory:

```bash
tar -czf posternproxy-linux-amd64.tar.gz posternproxy posternproxy-agent caddy checksums.txt
```

Use the architecture matching those binaries in the archive name. The archive
name does not change the target of the binaries inside.

## Next steps

If you received `posternproxy-linux-amd64.tar.gz`, it already contains the three
binaries and `checksums.txt`. Follow the bundle extraction steps in
[Installation]({{< relref "/getting-started/installation" >}}); the extracted
controller is named `posternproxy`.

Copy the three binaries and `checksums.txt` from the output directory to one
directory on the server. No source files or scripts are required:

```bash
chmod +x posternproxy
sudo ./posternproxy install --assets-dir .
```

See [Installation]({{< relref "/getting-started/installation" >}}) for firewall,
offline-package, and rerun options. The source-build path uses local assets so
unreleased code does not silently download dependencies from another release.

Tagged release builds use `.github/workflows/release.yml` to build the Windows
amd64, native Linux amd64/arm64, and Docker amd64/arm64 assets with a combined
`checksums.txt`. Both native Linux architectures receive controller, agent and
Caddy bundles. The workflow creates a draft release for owner review;
the owner must publish it before automatic installation can download those assets.
