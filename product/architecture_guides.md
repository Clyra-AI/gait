# Gait Architecture Guide

Version: 1.0
Status: Normative
Scope: Gait only (`gait/` repository root).

This document is the repo-local architecture overlay for Gait. It defines the
boundaries, review expectations, and failure-mode rules that planning and
implementation work must preserve.

## 1) Purpose

Use this guide to keep Gait's runtime-boundary architecture explicit,
testable, and safe.

Every architecture-impacting change must answer:

1. What public or internal contract changes?
2. What failure mode changes?
3. What test proves normal behavior?
4. What test proves degraded or unsafe behavior?
5. What migration or compatibility expectation applies?

## 2) Architecture Baseline

Gait remains an offline-first, fail-closed, evidence-producing CLI and runtime
boundary for AI agent tool calls.

Required boundaries:

- CLI command wiring and user-facing output
- Gate policy evaluation
- Pack, runpack, callpack, and trace artifacts
- Job runtime lifecycle and checkpoints
- MCP proxy, bridge, and server boundary adapters
- Registry, skill trust, and install verification
- Context evidence and credential handling
- Voice token minting and verification
- Scout drift, adoption, and operational signals
- Guard and incident evidence handling
- Python SDK adoption layer
- Docs site and local UI shell

Do not collapse enforcement logic into wrappers, examples, docs-site code, or
Python SDK convenience layers.

## 3) Current Package Map

| Boundary | Primary paths |
|---|---|
| CLI wiring | `cmd/gait/` |
| Gate and verdicts | `core/gate/` |
| Packs and runpacks | `core/pack/`, `core/runpack/`, `core/zipx/` |
| Jobs | `core/jobruntime/` |
| MCP | `core/mcp/`, `cmd/gait/mcp*.go` |
| Registry and trust | `core/registry/`, `cmd/gait/registry.go` |
| Context evidence | `core/contextproof/` |
| Credentials | `core/credential/` |
| Doctor and readiness | `core/doctor/` |
| Guard and incidents | `core/guard/` |
| Regressions | `core/regress/`, `core/policytest/` |
| Scout and telemetry | `core/scout/` |
| Schemas | `schemas/v1/`, `core/schema/` |
| Python adoption | `sdk/python/` |
| Docs and examples | `docs/`, `docs-site/`, `examples/` |

## 4) TDD and Contract Discipline

For behavior changes:

1. Add or update the failing test first where practical.
2. Implement the minimal code to pass.
3. Refactor while keeping tests green.

Minimum test additions by change type:

| Change type | Minimum required tests |
|---|---|
| CLI output or exits | command tests for help, JSON, stderr/stdout split, exit code |
| Gate policy behavior | allow/block/require-approval fixtures and fail-closed tests |
| Artifact schemas | schema validation, golden fixtures, migration/compatibility tests |
| Zip or signing behavior | byte-stability, canonicalization, digest, signature tests |
| Job runtime | lifecycle, checkpoint, pause/resume/cancel, contention tests |
| MCP boundary | transport and policy routing tests |
| SDK wrapper | Python tests plus Go command contract coverage |
| Docs/examples | docs checks and runnable example smoke where behavior changes |

## 5) Fail-Closed and Safety Rules

- `verdict != allow` means the side effect must not execute.
- Missing policy, undecidable policy, missing required approval, missing context
  evidence, or invalid high-risk credentials must block in production/high-risk
  modes.
- Unsafe real replay or raw capture requires explicit opt-in flags.
- Adapters and SDKs must call authoritative Go evaluation paths instead of
  reimplementing decisions.
- Prompt text and retrieved content are not policy engines; structured intent is
  the evaluation input.

## 6) Deterministic Artifacts

Any JSON participating in digest, signature, cache key, or diff must be JCS
canonicalized before hashing/signing.

Zip artifacts must preserve:

- deterministic file ordering
- stable timestamps
- stable modes
- stable ownership metadata
- explicit compression behavior

Never hash pretty-printed JSON or platform-dependent encodings.

## 7) Failure and Degradation Matrix

| Condition | Expected behavior |
|---|---|
| Policy cannot be parsed | fail closed with stable error |
| Tool intent is malformed | fail closed with invalid-input or policy/schema error |
| Approval token missing or expired | block or require approval; do not execute |
| Pack signature invalid | verification failure |
| Replay asks for real tools without unsafe flag | unsafe operation blocked |
| Context evidence required but absent | fail closed for high-risk action |
| MCP transport error before verdict | no side effect; explicit transport failure |
| Python wrapper cannot call Go CLI | no local fallback decision logic |

When modifying any row behavior, update tests and docs in the same change.

## 8) Architecture Review Triggers

Require explicit architecture notes or ADR-style PR content when changing:

- verdict semantics
- schema or artifact compatibility
- signing, hashing, canonicalization, or zip layout
- SDK public API
- MCP transport behavior
- job lifecycle semantics
- fail-closed behavior
- release or provenance behavior

Minimum ADR-style content:

```text
Context:
Decision:
Alternatives:
Compatibility impact:
Failure-mode impact:
Validation:
Rollback:
```

## 9) Command Matrix

| Change scope | Required command set |
|---|---|
| Standard behavior change | `make prepush` |
| Architecture, policy, MCP, voice, context, or failure change | `make prepush-full` plus targeted risk lane |
| Contract/schema/artifact change | `make test-contracts` plus targeted package/e2e tests |
| Reliability/fault-tolerance change | `make test-hardening` and relevant chaos target |
| Runtime SLO/perf-sensitive change | `make test-runtime-slo` or relevant budget target |
| Docs/examples integration change | docs checks plus example smoke for changed path |

## 10) Non-Goals

- This guide does not authorize hosted-dashboard scope in core runtime work.
- This guide does not move policy decisions into Python or examples.
- This guide does not permit fail-open behavior for high-risk tool execution.
- This guide does not replace `AGENTS.md`; it specializes architecture execution
  for planning and implementation workflows.

