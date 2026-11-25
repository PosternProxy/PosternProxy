---
title: Certificates
weight: 60
---

# Certificates

PosternProxy handles TLS in two ways: **automatic** certificates managed by Caddy,
and **custom** certificates you upload. The Certificates page manages the custom
certificates that proxy and redirection hosts can then reference.

## Automatic (Let's Encrypt HTTP-01)

For most public hosts you do not create a certificate here at all. Turn on
**Force HTTPS** for a proxy host without an assigned custom certificate and Caddy
obtains and renews a Let's Encrypt certificate automatically using the HTTP-01
challenge.

**Requirements:**
- The domain must have a public DNS A/AAAA record pointing to this server
- Port 80 must be reachable from the internet for the ACME challenge

Automatic certificates are managed entirely by Caddy. They do **not** appear on
the Certificates page. If provisioning can't complete yet (DNS not pointed, port
80 blocked), Caddy keeps retrying on its own; the Proxy Hosts list shows the live
TLS readiness status for each HTTPS host, and flips to OK on its own
once a certificate is obtained.

If a host does not have Force HTTPS on and no certificate assigned, it is served
over plain HTTP and no certificate is requested.

## Custom (upload your own)

Upload your own certificate and private key when you have:
- A certificate from a private/internal CA
- A wildcard certificate obtained outside of PosternProxy
- Short-lived certificates managed by your own tooling

### Adding a custom certificate

1. Go to **Custom Certs → Upload Certificate**
2. Enter a display name and domain names, then upload the PEM certificate and key
3. Click **Upload Certificate**

Then select it on a proxy or redirection host's **SSL** tab, or on a dead host.

## Certificate list

The Certificates page shows uploaded custom certificates with:

| Column | Description |
|---|---|
| Name | Display name supplied during upload |
| Domains | Domain names entered in the form |
| Expires | Expiry metadata, when present |
| Status | Upload/file-storage state; `active` is not proof of valid TLS |

## Expiry alerts

The Dashboard shows custom certificates expiring within 30 days using parsed leaf
expiry metadata. Upload validates PEM framing, key pairing, server usage,
domains and validity before changing storage or the database. Custom certificates
do not auto-renew.

## Storage and distribution

Private source material is encrypted at rest using the controller's stable
encryption key. Preserve that key with offline recovery copies. Only custom
certificates referenced by enabled TLS hosts on a target server are included in
its configuration and agent payload. Missing required material fails application.

Caddy needs plaintext PEM while serving TLS. The native installers provide a
dedicated `posternproxy-certs` group and runtime directory; complete immutable
pairs are readable by the application and Caddy without exposing controller data.
A failed apply preserves the previous pair. Successful applies retire unused
managed runtime files. Do not make keys world-readable to fix permissions.

Legacy plaintext sources are migrated to immutable encrypted copies. Old managed
numeric-directory files are retired only after a successful local application
and only when no certificate row still references them. Unreferenced encrypted
source pairs are currently retained for recovery; automatic cleanup targets
managed plaintext runtime files. Missing legacy files
produce a startup error log while management remains available for repair.

Check configuration status and an actual TLS connection after assigning a
certificate. An active upload is storage state, not proof that every target has
applied it. Encrypted recovery archives include custom certificate bytes; see
[Backup & Restore]({{< relref "/backup-restore" >}}).

## Not yet available

The following are planned but not implemented:

- **Let's Encrypt DNS-01 challenge** (wildcard / private domains via a DNS
  provider API). Today, wildcard or non-public domains require a custom upload.
- **Per-certificate renew / download** buttons.

## Notes

- Certificate private keys are stored on disk at `POSTERNPROXY_CERT_DIR` (`data/certs`
  by default; `/var/lib/posternproxy/certs` in installed configuration), as encrypted files with mode `0600`. Runtime PEM uses mode `0640` in a group-restricted directory.
- Automatic (HTTP-01) certificates are stored in Caddy's own certificate store and
  do not appear on this page.


## Status and Let's Encrypt errors

Proxy, redirect and parked host lists have expandable TLS status badges:

- **TLS ready**: every checked domain has a matching certificate within its
  validity dates. The check does not validate the certificate trust chain.
- **Certificate mismatch**: Caddy presented a certificate for another name.
- **TLS unavailable**: no usable TLS handshake, or invalid certificate dates.
- **HTTP only**: the source host does not request or serve a certificate.
- **Check TLS on node**: local-controller probes do not describe a remote server.

Local checks run every minute and include all configured domain aliases. Expand
the badge to see each result and its check time. Also check configuration
application status: a saved configuration can still be queued or rejected.

These checks describe the serving certificate, not a Let's Encrypt order. A TLS
failure can be temporary while issuance is in progress. For the actual issuance
or renewal error, inspect Caddy on the assigned server:

```bash
sudo journalctl -u caddy -n 100 --no-pager
# Follow retries live:
sudo journalctl -u caddy -f
```

Look for `tls.obtain`, `tls.renew`, challenge errors, DNS/address problems,
connection failures or rate-limit responses. In Docker, use `docker logs` for the
controller or agent container that serves the host. Forwarding a redirect to an
HTTPS destination does not enable HTTPS on its source domain.
