---
title: Windows services
weight: 3
---

# Windows service installation

The native Windows amd64 bundle contains `posternproxy.exe`,
`posternproxy-agent.exe`, `caddy.exe` and `checksums.txt`. Use an elevated
PowerShell terminal on a dedicated proxy machine. The services use separate
virtual accounts; no service-account passwords or Bash installation are needed.

The development bundle passed an authorized Windows workstation lifecycle test:
controller and agent SCM installation, login, reinstall, service identities,
encrypted enrollment, custom-certificate HTTPS, restart and data-preserving
removal. The Windows release workflow repeats these checks against its candidate
bundle; a local development run does not qualify untested release assets.

## Controller

Download the Windows zip from the selected release and verify its hash against
that release's `checksums.txt`, then extract and install:

```powershell
Expand-Archive .\posternproxy-windows-amd64.zip .\posternproxy-install
Set-Location .\posternproxy-install
.\posternproxy.exe install --assets-dir .
```

Fresh installs print the administrator login after services become ready.
Reinstalling preserves the database, encryption identity and existing login.
The management UI defaults to port 81. Port 8443 is reserved for the internal
TLS-passthrough bridge; management also cannot use 80, 443 or 2019.

The controller service is `posternproxy`; its Caddy service is
`posternproxy-caddy`. Binaries live under `%ProgramFiles%\PosternProxy` and data
under `%ProgramData%\PosternProxy`. The Caddy account can read staged runtime
certificates, not the controller's private certificate source directory.
Windows Firewall is unchanged. Restrict management to your administration
network, and configure the public HTTP/TLS firewall rules explicitly.

## Service settings

For a TLS management frontend or remotely enrolled agents, supply a JSON file
with the required settings. This file is merged during installation/reinstallation;
settings not supplied are preserved:

```json
{
  "POSTERNPROXY_CONTROLLER_URL": "https://proxy.example.com",
  "POSTERNPROXY_BEHIND_TLS_PROXY": "true"
}
```

```powershell
.\posternproxy.exe install --assets-dir . --config-file .\service-settings.json
```

Use `BEHIND_TLS_PROXY=true` only when management is actually accessed through
HTTPS. Other supported keys are `POSTERNPROXY_LISTEN`,
`POSTERNPROXY_TRUSTED_PROXIES`, `POSTERNPROXY_CADDY_TRUSTED_PROXIES` and
`POSTERNPROXY_ALLOWED_ORIGINS`. Storage paths and the encryption identity are
managed by the installer. The installed service definitions are encrypted with
machine-scoped Windows DPAPI and restricted NTFS ACLs. Tokens never appear in
the service command line. Do not hand-edit encrypted `.dat` files.

## Remote agent

Create a manual server enrollment in the controller. Save its JSON as a protected
`agent.json` on the remote Windows machine, then run from the matching bundle:

```powershell
.\posternproxy-agent.exe install --assets-dir . --config-file .\agent.json
```

The installer verifies the controller accepts this enrollment before installing.
The agent runs as `posternproxy-agent` and connects outbound to the controller.
Check the controller's applied revision as well as Online status; installed
services alone do not prove configuration has applied. Controller and agent
service installations on the same Windows machine are refused because both
would own its Caddy configuration. Delete the input enrollment file after a
successful install and keep enrollment tokens private.

A released Windows controller can also provision native Linux agents over SSH.
It downloads the matching release's Linux agent and Caddy assets and verifies
their checksums, architecture and Caddy build contract before transferring them.
This requires outbound HTTPS access to the public release assets. Development
builds need matching Linux assets supplied locally; native release bundles target
Linux amd64 and arm64.

## Operations and recovery

```powershell
Get-Service posternproxy,posternproxy-caddy
Restart-Service posternproxy
Get-Content "$env:ProgramData\PosternProxy\posternproxy\service.log" -Tail 100
.\posternproxy.exe uninstall
```

Use the corresponding agent executable/service/log directory on agent machines.
`uninstall` stops and unregisters the managed services while retaining binaries,
configuration and data. It does not remove firewall rules or delete backups.
Failed binary replacements retain `.previous` files for operator recovery.

To reset an installed controller administrator password from an elevated terminal:

```powershell
$env:POSTERNPROXY_DATA_DIR = "$env:ProgramData\PosternProxy\posternproxy"
$env:POSTERNPROXY_DB_PATH = "$env:POSTERNPROXY_DATA_DIR\posternproxy.db"
.\posternproxy.exe reset-password
```

Retain the original data, `secret.key` and protected configuration for recovery.
Machine-scoped DPAPI service definitions are tied to that Windows machine. For
recovery onto another machine, use an encrypted portable configuration archive
on a fresh installation and reenroll agents; copying `.dat` files alone is not
a cross-machine recovery method.
