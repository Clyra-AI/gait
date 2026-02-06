# Hardening Release Checklist

Use this checklist before creating a release tag. Items marked "MANDATORY" are release-blocking.

## 1) Baseline Validation (MANDATORY)

- [ ] `make lint` passes on a clean checkout.
- [ ] `make test` passes with coverage gates:
  - Go coverage >= 85%
  - Python coverage >= 85%
- [ ] `make test-hardening-acceptance` passes.
- [ ] CI `hardening` job is green on the release commit.

## 2) Contract Integrity (MANDATORY)

- [ ] Public CLI exit-code behavior is unchanged or intentionally documented.
- [ ] `--json` error envelope remains stable (`error_code`, `error_category`, `retryable`, `hint`).
- [ ] Schema changes are additive and versioned; no unplanned breaking changes in v1 artifacts.
- [ ] Golden tests for error envelopes and critical outputs are green.

## 3) Security and Privacy (MANDATORY)

- [ ] `gosec` and `govulncheck` pass with no unresolved critical findings.
- [ ] Credential broker safety controls verified:
  - command allowlist behavior
  - timeout/output-size bounds
  - no secret leakage in default CLI outputs
- [ ] Key source configuration checks pass (`doctor` and command-level validation).
- [ ] Unsafe operations retain explicit interlocks and fail-closed defaults.

## 4) Determinism and Artifact Safety (MANDATORY)

- [ ] Deterministic zip generation tests pass.
- [ ] Atomic write and lock contention tests pass.
- [ ] Registry retry/fallback behavior remains deterministic and trust-preserving.
- [ ] Trace/runpack verification passes on regenerated artifacts.

## 5) Supply Chain Integrity (MANDATORY)

- [ ] Release workflow tool versions are pinned.
- [ ] Checksums generated and verified.
- [ ] Signatures/provenance artifacts generated and verifiable.
- [ ] Release workflow integrity verification steps complete successfully.

## 6) Operational Readiness (RECOMMENDED)

- [ ] `gait doctor --json` includes green checks for hooks, cache, lock staleness, temp writeability, and key-source ambiguity.
- [ ] Correlation IDs and operational events are emitted in opt-in logs where enabled.
- [ ] Relevant hardening docs updated:
  - `docs/hardening/contracts.md`
  - `docs/hardening/risk_register.md`
  - framework alignment matrices

## 7) Release Decision

- [ ] Release manager sign-off (engineering owner)
- [ ] Security sign-off (if security-sensitive changes included)
- [ ] Go/No-Go recorded in release notes
