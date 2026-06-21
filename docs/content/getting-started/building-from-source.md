---
title: Building from Source
weight: 3
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

**Requirements:** Go 1.25+ and Node.js 20+ (with npm).

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

{{< hint info >}}
**Native Windows binaries**
The scripts target Linux by default because PosternProxy runs on Linux (the
installer is Debian/Ubuntu-only). You can produce native Windows binaries for
local testing with `.\scripts\build.ps1 -GoOS windows`, but there is no supported
Windows install path.
{{< /hint >}}

## What the build does

1. Builds the Vue frontend (`npm ci && npm run build`) and embeds it into the
   controller binary via `go:embed`.
2. Compiles the controller and agent as static binaries (`CGO_ENABLED=0`).
3. Builds Caddy with `xcaddy`, adding the `caddy-l4` and `caddy-ratelimit` plugins
   (installing `xcaddy` on the build host if needed).

## Next steps

Once built, follow [Installation]({{< relref "/getting-started/installation" >}})
from **Step 2** — copy the three binaries and `scripts/install.sh` to the server
and run the installer. (Binaries from `scripts/build.sh` are in the repo root;
binaries from `scripts/build.ps1` are in `bin\`.)
