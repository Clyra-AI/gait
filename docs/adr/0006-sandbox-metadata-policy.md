# ADR 0006: Sandbox Metadata Policy Enforcement

- Status: Accepted
- Date: 2026-05-09
- Related epic: `GSC1.3`

## Context

Gate can classify `proc.exec` as a high-risk execution boundary, but it
previously lacked a first-class contract for bounded network, filesystem,
environment, timeout, and privilege posture.

## Decision

Add schema-backed sandbox metadata on intent context plus a rule-level
`sandbox` policy block:

- normalized sandbox metadata with network mode, writable paths, read-only
  roots, env exposure mode, timeout, filesystem isolation, user mode, and
  evidence ref/digest
- fail-closed blocking when required sandbox metadata is missing or too
  permissive
- signed traces and JSON output record sandbox decision state and evidence
  digest/ref, not raw environment contents
- valid sandbox posture can satisfy the `proc.exec` path without relying on the
  older blanket destructive auto-approval path

## Alternatives Considered

1. Wrapper-local sandbox checks.
   - Rejected: breaks the Go-authoritative policy boundary.
2. Trace raw environment or writable path details.
   - Rejected: unnecessary privacy and leakage risk.

## Consequences

- Operators get a stable machine-readable sandbox posture contract.
- High-risk execution paths fail closed when callers cannot prove bounded
  runtime posture.
- Proof and explain surfaces can reference sandbox evidence without storing raw
  env data.
