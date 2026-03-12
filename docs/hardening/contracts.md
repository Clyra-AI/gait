# Hardening Contracts (Epic H0.1)

This document defines non-functional contracts for Gait reliability hardening.

## Scope

Applies to all commands that read, write, verify, or enforce artifact and policy behavior in `cmd/gait` and `core/*`.

## Non-Functional Requirements (NFRs)

### NFR-01 Startup And Dependency Behavior

- Commands fail fast when required local dependencies are missing.
- Failure outputs include deterministic machine-readable identifiers (`error_code`, `error_category`) under `--json`.
- No network dependency is required for offline core command startup.

Mapped hardening epics: `H1`, `H5`, `H6`.

### NFR-02 Latency Budgets For Critical Paths

Target budgets for local baseline runs on developer hardware:

- `gait verify <run_id>`: median <= 500 ms for demo-sized runpack.
- `gait gate eval --json`: median <= 300 ms for single intent and policy.
- `gait regress run --json`: median <= 1500 ms for demo fixture.

Mapped hardening epics: `H6`, `H11`.

### NFR-03 Failure Classification

- Runtime failures are classified into canonical categories.
- Input validation failures do not mask IO, contention, or transient network failures.
- Exit behavior remains backward-compatible unless explicitly versioned and documented.

Mapped hardening epics: `H1`, `H6`, `H12`.

### NFR-04 Crash Consistency

- Critical writes use atomic write semantics (temp write + sync + rename).
- Partial writes must not produce corrupted state accepted as valid.
- Durable job lifecycle mutations must either roll back to the prior state or leave a deterministic local recovery marker until reconciliation completes.
- Security-sensitive outputs must preserve least-privilege permissions.

Mapped hardening epics: `H2`, `H6`, `H12`.

### NFR-05 Contention Determinism

- Concurrent access to shared state has bounded retry and deterministic timeout behavior.
- Lock contention failures are machine-classifiable and include remediation hints.

Mapped hardening epics: `H3`, `H5`, `H12`.

### NFR-06 Optional Network Resilience

- Remote registry operations retry only transient classes of failure.
- Retry budget is bounded and deterministic.
- Trust checks (pin/signature/allowlist) remain fail-closed.

Mapped hardening epics: `H4`, `H12`.

### NFR-07 Operational Diagnostics

- `gait doctor --json` emits actionable fix guidance for common operational faults.
- `gait doctor --json` detects durable job state divergence or pending recovery markers under `gait-out/jobs`.
- Diagnostics remain deterministic and safe for offline usage.
- Optional operational logs (`GAIT_OPERATIONAL_LOG`) emit stable start/end events with correlation IDs.
- Correlation IDs are present in JSON command outputs and gate traces for deterministic cross-artifact debugging.

Mapped hardening epics: `H5`, `H6`.

### NFR-08 Release Integrity Guardrails

- Release workflows are reproducible and pinned where feasible.
- Integrity artifacts (checksums/signatures/provenance) are verified before release completion.

Mapped hardening epics: `H7`, `H9`, `H12`.
