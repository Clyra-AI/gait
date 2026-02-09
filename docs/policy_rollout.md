# Policy Rollout Guide (Epic A4.2)

This guide defines a staged rollout from observe to enforce without service interruption.

## Objective

Move policy controls into production safely:

- start with visibility
- validate deterministic policy behavior in CI
- enforce approvals on high-risk operations
- enforce blocks only after evidence is stable

## Stage 0: Fixture Baseline In CI

Run deterministic policy fixture tests on every PR:

```bash
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_medium_risk.yaml examples/policy/intents/intent_write.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delete.json --json
```

Rollout gate:

- Do not ship policy changes unless fixture tests pass.

## Stage 1: Observe (Simulate Only)

Evaluate policy but do not enforce runtime blocking yet:

```bash
gait gate eval --policy examples/policy/base_medium_risk.yaml --intent examples/policy/intents/intent_write.json --simulate --json
```

Interpretation:

- Exit code remains `0` because simulation is non-enforcing.
- Use `verdict`, `reason_codes`, and trace outputs for tuning.

Rollout gate:

- Move forward only when false positives are at or below your threshold.

## Stage 2: Dry-Run Execution Boundary

Use `dry_run` policy effects for selected high-risk tools and route calls through wrapper execution:

- `ToolAdapter.execute(...)` executes only on `allow`.
- `dry_run` returns decision context with `executed=false`.
- No side effects should occur in this stage.

Rollout gate:

- Move forward only after dry-run telemetry shows expected decisions and zero unsafe bypasses.

## Stage 3: Enforce Approval For High-Risk Tools

Require approvals before selected operations:

```bash
gait gate eval --policy examples/policy/base_high_risk.yaml --intent examples/policy/intents/intent_write.json --json
```

Interpretation:

- Exit `4` means approval is required and execution must not proceed.
- Integrate approval token issuance and retry with approved intent.

Rollout gate:

- Move forward only when approval workflow latency and operability are acceptable.

## Stage 3B: Skill Trust Guardrails

When skills initiate tool calls, add trust conditions:

- `skill_publishers` for known publishers
- `skill_sources` for approved distribution channels (`registry` preferred)

Example checks:

```bash
gait policy test examples/policy/skills/allow_trusted.yaml examples/policy-test/intent.json --json
gait policy test examples/policy/skills/block_untrusted.yaml examples/policy-test/intent.json --json
```

Rollout gate:

- Require explicit approval or block for unknown publisher/source combinations before production enforce.

## Stage 4: Full Enforce Mode

Enforce block/allow decisions at runtime in wrapped tools:

- `allow`: execute tool call
- `require_approval`: block until approved
- `block`: deny execution
- invalid decision/evaluation failure: fail closed

Rollout gate:

- Production rollout is blocked if any high-risk tool path can bypass wrapper enforcement.

## Exit-Code Handling Matrix (CI And Runtime)

Use stable gate exit codes as control signals:

| Exit | Verdict/State | CI Behavior | Runtime Behavior |
| --- | --- | --- | --- |
| `0` | `allow` (or simulated result) | Pass gate step | Execute if wrapper verdict is `allow` |
| `3` | `block` | Fail PR for policy regressions | Deny execution and emit trace |
| `4` | `require_approval` | Fail or mark blocked until approved path is supplied | Deny execution until approval token flow completes |
| `6` | invalid input/schema | Fail fast (treat as broken pipeline) | Fail closed, do not execute |

## Recommended CI Wiring

- PR workflow:
  - `gait policy test` fixture suite
  - targeted `gait gate eval --simulate` checks for changed policies
- nightly workflow:
  - broader intent fixture replay against all baseline policy packs
- deployment gate:
  - block rollout if exit `3`, `4`, or `6` appears for required production intents
