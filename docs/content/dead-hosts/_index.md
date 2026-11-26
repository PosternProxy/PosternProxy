---
title: Dead Hosts (Parked Domains)
weight: 50
---

# Dead Hosts

Dead hosts serve a fixed HTML response for a domain instead of proxying to an
upstream. Use them for parked domains, coming-soon pages, and maintenance pages.

## Creating a dead host

1. Open **Parked Domains** in the sidebar and add a parked-domain entry.
2. Enter the domain names and select the target server.
3. Choose **200**, **404**, **410**, or **503** as the response status.
4. Optionally enter a custom HTML body. Leaving it blank uses the bundled parked
   page for the selected status.
5. Select a custom certificate if needed, then create the host.

The bundled parked page includes the PosternProxy logo and follows the visitor's
system light or dark theme automatically. Images and styles are self-contained,
including on remote servers. Custom HTML controls its own appearance.

Select **HTTP only (no certificate)** for plain HTTP without an HTTPS route or
certificate request. Otherwise the host serves both HTTP and HTTPS; choose a
custom certificate or let Caddy obtain one automatically. Existing parked domains
retain HTTPS behavior after upgrading. The HTTP-only choice is included in
configuration backups and applies to the assigned local or remote server.

The **TLS** column shows readiness. Expand its badge for per-domain results,
expiry/check times, and certificate troubleshooting. Live probes cover local
hosts; remote rows direct you to the assigned node. See
[Certificates]({{< relref "/certificates" >}}#status-and-lets-encrypt-errors).

## Access and application status

Select an access list to enforce its IP allow/deny rules on both HTTP and HTTPS
parked-domain routes. The public option applies no IP restriction.

Enable and disable controls save desired configuration and trigger application.
Check [Configuration application status]({{< relref "/multi-server/configuration-status" >}})
when Caddy rejects a change or the assigned remote agent is offline. Disabling a
host removes its managed route after application; other routes on that server
can still match the same request.
