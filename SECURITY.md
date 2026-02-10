# Security Policy

## Reporting a Vulnerability

Please report security issues privately.

- Email: dahmann@lumyn.cc
- GitHub Security Advisory: https://github.com/davidahmann/gait/security/advisories/new

Do not open public issues for security vulnerabilities.

Response targets (best effort):

- Initial acknowledgement within 3 business days.
- Triage severity assessment within 7 business days for reproducible reports.
- Coordinated disclosure timeline shared after triage.

## Supported Versions

Only the latest tagged release is supported with security fixes.

## Approval Key Management

Approval and trace signing keys are operational security assets:

- Store private keys in a secret manager or HSM-backed system.
- Do not commit keys, tokens, or customer artifacts to git.
- Rotate signing keys on a fixed schedule and immediately after suspected compromise.
- Publish and distribute public verification keys before key cutover.
- Keep approval signing keys separate from release signing keys.

Operational procedure:

- Follow `docs/approval_runbook.md` for token minting, TTL/scope policy, and incident audit workflow.

## Security Controls In CI

Security checks run in CI and local lint workflows:

- `go vet`, `golangci-lint`, `gosec`, `govulncheck` for Go
- `ruff`, `mypy`, `bandit`, `pytest` for Python wrapper code

Release integrity is validated with signed checksums, SBOM, and provenance artifacts.

## Demo Key Material

Any key material under `examples/scenarios/keys/` is for local walkthroughs only and must not be used for production key management.
