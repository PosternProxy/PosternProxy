---
title: TLS Passthrough
weight: 5
---

# TLS Passthrough

TLS Passthrough lets Caddy forward encrypted TLS connections directly to an upstream without terminating (decrypting) them. The upstream handles TLS itself — Caddy never sees the plaintext.

## When to use it

Use TLS Passthrough when:

- The upstream requires **mutual TLS (mTLS)** / client certificates that Caddy cannot forward
- The upstream is a **database** or other service presenting its own TLS certificate
- You need **end-to-end encryption** between client and upstream with no proxy in the middle
- The upstream uses a **private CA** certificate that Caddy cannot validate

## How to enable

On the **Details** tab, toggle **TLS Passthrough** on. When enabled:

- PosternProxy uses Caddy's Layer 4 (`caddy-l4`) module instead of the HTTP server
- The stream is matched by **SNI hostname** (the domain name in the TLS ClientHello)
- Traffic is forwarded as raw TCP bytes — no HTTP inspection, headers, or access lists
- The upstream port must accept TLS connections

<!-- TODO: Screenshot — Details tab with TLS Passthrough toggle enabled -->

## Limitations

Because TLS Passthrough bypasses Caddy's HTTP layer, the following features are **not available** when passthrough is enabled:

| Feature | Available? |
|---|---|
| Automatic Let's Encrypt | ✗ (upstream provides its own cert) |
| HSTS / Force HTTPS | ✗ |
| Access lists (IP rules) | ✗ (client IP is still visible at L4) |
| Rate limiting | ✗ |
| Custom headers | ✗ |
| Custom error pages | ✗ |
| WebSocket support | ✓ (transparent) |

## SNI matching

Caddy uses the SNI field of the TLS ClientHello to match the stream to the correct proxy host. The client **must** send SNI — most modern clients do automatically.

Clients that do not send SNI (e.g. some legacy tooling using bare IP addresses) cannot be matched and will receive a connection reset.

## Layer 4 plugin

TLS Passthrough requires the `github.com/mholt/caddy-l4` plugin. This is included in the Caddy binary built by `scripts/install.sh` — no additional steps are needed.
