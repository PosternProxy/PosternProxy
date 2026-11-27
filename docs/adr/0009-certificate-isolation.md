# 0009. Certificates are isolated per server, with key-safe delivery and a defined access contract

- **Status:** Accepted
- **Date:** 2026-11-27 (records a decision made during the beta-exit program)
- **Deciders:** Michael (owner)
- **Topics:** security, certificates, agent

## Context

Custom certificates and keys must reach the servers that need them without leaking key material to unrelated servers, and the Caddy runtime user must be able to read them. Earlier code bundled all custom PEM/key bytes to every remote server and used directories incompatible with the Caddy service user.

## Decision

1. Certificates are **selected and delivered per server**, not bundled to every server.
2. Private key material is handled with **restricted ownership/permissions** and a defined **agent cert-writer contract** compatible with the Caddy service user.
3. Certificate **activation and retirement** are coordinated with the apply model (ADR 0008).

## Consequences

- Easier: keys do not fan out to servers that should not hold them.
- Harder: installers and service units must honour the certificate access contract across users/sandboxes.
- Now a rule: no broadcasting of key material; certificate changes flow through reconciliation.

## Evidence

`internal/caddy/tls.go`, the certificate service/handlers and agent cert writer; beta-exit certificates (blockers 9/10). See [../evidence/0009-certificate-isolation.md](../evidence/0009-certificate-isolation.md).
