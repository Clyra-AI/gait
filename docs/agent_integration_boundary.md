---
title: "Integration Boundary Guide"
description: "Where Gait sits relative to your agent runtime and what interception tiers are supported for enforcement."
---

# Agent Integration Boundary Guide

Use this guide when deciding how Gait fits with your agent runtime, especially managed/preloaded agents.

## Core Rule

Fail-closed enforcement requires an interception point before real tool execution.

If you cannot intercept tool calls, Gait can still add observe/report/regress value, but it cannot block side effects inline.

Concrete boundary touchpoints:

- call `gait gate eval` at the final adapter or middleware dispatch site before execution
- pass `--context-envelope <context_envelope.json>` whenever policy requires context evidence; for MCP server boundaries, either start `gait mcp serve` with `--context-envelope` or explicitly allow same-host request paths with `--allow-client-artifact-paths` and `call.context.context_envelope_path`
- keep `verdict != allow` as non-executing in the adapter response
- use `gait demo --json` for machine-readable wrapper or CI smoke checks

## Integration Tiers

## Tier A: Full Runtime Interception (Best Fit)

Examples:

- wrapper/decorator in your app runtime
- local sidecar calling `gait gate eval`
- local `gait mcp serve` with caller-side enforcement
- `gait test` / `gait enforce` wrapping an explicit Gait-aware quickstart or middleware seam

What Gait can do:

- full policy enforcement (`allow` only executes)
- approval/delegation token checks
- verified context-evidence enforcement when the wrapper, sidecar, or MCP boundary passes `--context-envelope` for context-required policies
- signed trace emission
- runpack/pack capture and regress loop

What remains external:

- model selection/planning/orchestration logic

## Tier B: Middleware/API Boundary Interception

Examples:

- API gateway/middleware for tool execution endpoint
- service boundary where tool payloads can be normalized and evaluated

What Gait can do:

- enforce decisions at middleware boundary
- emit signed traces and artifacts
- provide deterministic CI regression loop
- enforce authenticated context-proof checks when middleware can supply the local context envelope artifact alongside the gate call
- for `gait mcp serve`, keep the default fail-closed boundary by pinning `--context-envelope`, or explicitly opt into same-host request artifact paths before honoring `call.context.context_envelope_path`

Constraints:

- normalization quality depends on payload completeness at boundary
- enforcement reliability depends on strict middleware placement
- `gait test` and `gait enforce` do not auto-instrument arbitrary runtimes; they require emitted Gait trace references from the child integration
- wrapper JSON now makes that boundary explicit with `boundary_contract=explicit_trace_reference`, `trace_reference_required=true`, and stable `failure_reason` values such as `missing_trace_reference` or `invalid_trace_artifact`

## Tier C: Managed/Preloaded Agent Products (Limited Interception)

Examples:

- hosted copilots/preloaded agents where tool execution path is not user-controlled

What Gait can do:

- observe/report if traces/artifacts can be exported
- post-hoc verification and regress generation from exported evidence
- policy simulation and CI policy tests with representative fixtures

What Gait cannot do (without interception):

- guaranteed inline block before side effects
- strict runtime fail-closed enforcement

## Decision Flow

1. Can you intercept every state-changing tool call before execution?
2. If yes: use Tier A and enforce non-`allow` as non-executable.
3. If partially: use Tier B and close bypass paths first.
4. If no: use Tier C and treat controls as observe/report/regress until interception is available.

## Practical Paths

- Tier A quickstart: `examples/integrations/openai_agents/quickstart.py`
- Tier A/B transport path: `gait mcp serve --context-envelope ./context_envelope.json`
- Reference adapter note: `examples/integrations/claude_code/gait-gate.sh` fails closed on hook/runtime/input errors by default; `GAIT_CLAUDE_UNSAFE_FAIL_OPEN=1` is a debugging-only unsafe override, not a promoted lane.
- Tier C fallback: `gait report top`, `gait capture`, `gait regress add`, `gait regress run` (or `gait regress bootstrap` for the one-command path)

## Related Docs

- `docs/flows.md`
- `docs/integration_checklist.md`
- `docs/mcp_capability_matrix.md`
