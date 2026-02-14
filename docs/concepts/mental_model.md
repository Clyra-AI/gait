# Gait Mental Model

Use this as the 5-minute bridge between `gait demo` and production integration.

## Plain-Language Summary

When agents can execute tools, teams need to record actions, control execution, debug failures, and prove what happened.  
Gait is the deterministic layer that sits at the tool boundary to provide those four outcomes.

## One Sentence Model

Gait makes agent tool calls controllable by turning execution into four deterministic primitives: capture (`runpack`), enforce (`gate`), regress (`regress`), and diagnose (`doctor`).

## What Each Primitive Does

- `runpack`: records a deterministic artifact (`runpack_<run_id>.zip`) with manifest, intents, results, and receipts.
- `gate`: evaluates one structured `IntentRequest` against policy and returns a deterministic verdict (`allow`, `block`, `dry_run`, `require_approval`) plus signed trace.
- `regress`: converts an incident runpack into repeatable CI checks and JUnit-compatible output.
- `doctor`: validates local environment and produces machine-readable fixes.

## End-To-End Runtime Flow

1. Agent runtime reaches a tool boundary (wrapper or sidecar).
2. Integration emits `IntentRequest` and calls `gait gate eval`.
3. `gate` returns verdict + trace.
4. Tool executes only when verdict is `allow`.
5. Incident or drift uses `runpack` + `regress` to become a deterministic test.

## Sync Versus Async

- Synchronous path:
  - `gait gate eval`
  - `gait demo`
  - `gait verify`
  - `gait regress run`
- Asynchronous/operational path:
  - CI workflows and nightly suites
  - optional adoption and operational logs

## Where State Lives

- Local artifacts: `./gait-out/`
- Optional runpack cache: `~/.gait/runpacks`
- Optional registry cache and pins: `~/.gait/registry`
- Deterministic contracts: JSON schemas under `schemas/v1/` and Go types under `core/schema/v1/`

## Failure Behavior (Default-Safe)

- Core workflows are offline-first.
- Non-`allow` gate verdicts do not execute side effects.
- High-risk `oss-prod` paths enforce fail-closed requirements.
- Artifact verification failures are explicit and machine-readable (`--json` with stable exit codes).

## What This Is Not

- Not an agent orchestrator.
- Not a hosted dashboard dependency.
- Not a prompt-only filter.
- Not an automatic framework interceptor; enforcement requires a wrapper/sidecar/proxy hook.

## Visual References

- System architecture: `docs/architecture.md`
- Runtime and operational flows: `docs/flows.md`
- Managed/preloaded agent integration boundaries: `docs/agent_integration_boundary.md`
- Documentation ownership map: `docs/README.md`

For implementation details and exact integration checks, use `docs/integration_checklist.md`.
