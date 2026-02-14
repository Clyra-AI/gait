# Agent Integration Boundary Guide

Use this guide when deciding how Gait fits with your agent runtime, especially managed/preloaded agents.

## Core Rule

Fail-closed enforcement requires an interception point before real tool execution.

If you cannot intercept tool calls, Gait can still add observe/report/regress value, but it cannot block side effects inline.

## Integration Tiers

## Tier A: Full Runtime Interception (Best Fit)

Examples:

- wrapper/decorator in your app runtime
- local sidecar calling `gait gate eval`
- local `gait mcp serve` with caller-side enforcement

What Gait can do:

- full policy enforcement (`allow` only executes)
- approval/delegation token checks
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

Constraints:

- normalization quality depends on payload completeness at boundary
- enforcement reliability depends on strict middleware placement

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
- Tier A/B transport path: `gait mcp serve`
- Tier C fallback: `gait report top`, `gait regress init`, `gait regress run`

## Related Docs

- `docs/flows.md`
- `docs/integration_checklist.md`
- `docs/mcp_capability_matrix.md`
