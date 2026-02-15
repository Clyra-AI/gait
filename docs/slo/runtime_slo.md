# Runtime SLO Contract

This document defines the measurable runtime contract for Gate evaluation in OSS runtime lanes.

Version semantics: this page is evergreen guidance. Release-specific rollout notes belong in plan/changelog docs; compatibility details belong in `docs/contracts/compatibility_matrix.md`.

## Scope

Applies to local, offline Gate execution and related safety checks in the default OSS lane:

- `gait demo`
- `gait gate eval`
- `gait verify`
- `gait regress run`
- `gait guard pack`
- `gait run session checkpoint`
- `gait verify session-chain`
- `gait gate eval` with delegation token verification
- context-proof operations:
  - envelope capture + verify
  - context-required gate eval
  - context-aware pack diff
  - context conformance regress run

## Measurement Command

Use the canonical command budget harness:

```bash
make bench-budgets
make context-budgets
```

Report output:

- `perf/command_budget_report.json`
- `perf/context_budget_report.json`

The report includes:

- p50/p95/p99 latency
- per-command error rate
- pass/fail status for each budgeted command

## Latency SLOs

The enforced budgets are configured in:

- `perf/runtime_slo_budgets.json`

Gate endpoint-class budgets are evaluated for:

- `fs.read`
- `fs.write`
- `fs.delete`
- `proc.exec`
- `net.http`
- `net.dns`

Session/delegation governance budgets are also evaluated for:

- `session_checkpoint_emit`
- `session_chain_verify`
- `gate_eval_delegation_verify`

Budget checks are enforced on all of: p50, p95, and p99.

## Error-Budget Envelope

For each budgeted command in the runtime harness:

- `max_error_rate` is enforced (default `0.0` for all v1.7 lanes)
- any command above budget or above error-rate threshold fails the check

This makes latency and reliability a release gate, not an observational metric only.

## Fail-Closed Contract

Safety posture must remain fail-closed in protected paths:

- Non-`allow` Gate verdicts must not execute side effects.
- Invalid intent payloads must not produce execution permission.
- High-risk `oss-prod` profiles must reject unsafe policy/broker preconditions.
- Skill verification failures in registry trust checks must fail verification.
- Broker credential failures for broker-required policies must degrade to block.

These behaviors are asserted in:

- `internal/e2e/` fail-closed matrix tests
- unit tests under `core/gate/` and `cmd/gait/`

## CI Enforcement

The runtime SLO check is enforced in CI through:

- `make bench-budgets`
- v1.7 acceptance gate (`scripts/test_v1_7_acceptance.sh`)
- nightly perf profile (`.github/workflows/perf-nightly.yml`)

Any SLO or fail-closed regression should block merge.

## Context Budget Inputs

Context runtime budget thresholds are configured in:

- `perf/context_budgets.json`

The checker:

- `scripts/check_context_budgets.py`

is enforced in:

- `make test-runtime-slo`
- `make bench-check`
- `.github/workflows/perf-nightly.yml`
