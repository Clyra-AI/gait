---
title: "Mental Model"
description: "Problem-first model for why Gait exists, where the tool boundary sits, and how deterministic primitives map to incident control."
---

# Gait Mental Model

Use this as the 5-minute bridge between `gait demo` and production integration.

## Problem-First View

If your agent can execute tools, these failures become expensive fast:

- destructive actions happen and teams cannot prove policy context later
- incidents cannot be reproduced because traces are nondeterministic
- long-running runs fail mid-way with unclear state and no portable evidence

Gait exists to make those failure modes deterministic, enforceable, and auditable offline.

## One Sentence Model

Gait is the control and evidence runtime at the tool boundary: capture what happened, enforce policy before side effects, convert incidents into deterministic regressions, and diagnose environment drift.

## Tool Boundary (Canonical Definition)

A tool boundary is the exact call site where your runtime is about to execute a real tool side effect.

- input across the boundary: structured `IntentRequest`
- evaluator: `gait gate eval` (or `gait mcp serve` boundary endpoint)
- hard rule: non-`allow` verdict means non-execute

This is where fail-closed safety is enforced.

## Deterministic Surfaces

- `gate`: evaluate intent against policy and return deterministic `allow|block|require_approval|dry_run` plus signed trace.
- `runpack`/`pack`: emit portable, verifiable evidence artifacts for run, job, and call paths.
- `regress`: turn incidents into fixture-backed CI checks with stable exit behavior.
- `jobs`: run checkpointed long-running work with pause/resume/cancel/approve/inspect lifecycle.
- `doctor`: machine-readable first-run diagnostics and remediation hints.

Contract note: primitive behavior is normative in `docs/contracts/primitive_contract.md`.

## End-To-End Runtime Flow

1. Agent runtime reaches a tool boundary (wrapper or sidecar).
2. Integration emits `IntentRequest` and calls `gait gate eval`.
3. `gate` returns verdict + trace.
4. Tool executes only when verdict is `allow`.
5. Incident or drift uses `runpack` + `regress` to become a deterministic test.

## Where This Happens In Code

- wrapper pattern: `examples/integrations/openai_agents/quickstart.py`
- command entrypoint: `cmd/gait/gate.go`
- policy evaluation engine: `core/gate/`
- artifact verification and contracts: `core/runpack/`, `core/pack/`, `schemas/v1/`

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

## Frequently Asked Questions

### What is a runpack?

A runpack is a tamper-evident ZIP bundle containing intents, results, reference receipts, and a SHA-256 manifest. It is the portable unit of proof for an agent run.

### How is Gait different from observability stacks such as LangSmith or LangFuse?

Gait is a local control and evidence runtime. It enforces execution policy at the tool boundary and emits verifiable artifacts. Observability stacks focus on hosted tracing and analytics.

### Is Gait an agent orchestrator?

No. Gait does not dispatch prompts, manage models, or route conversations. It is the deterministic control and evidence layer at the tool boundary.

### What does offline-first mean?

Core workflows — verify, diff, replay, regress, and policy evaluation — run without network access. No SaaS dependency for critical operations.

### What does fail-closed mean?

When policy evaluation cannot determine a clear allow verdict, the action is blocked. Ambiguity defaults to non-execution.
