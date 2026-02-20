---
title: "Policy Rollout"
description: "Roll out policy changes from observe to require_approval to enforce with simulation and impact analysis."
---

# Policy Rollout Guide (Epic A4.2)

This guide defines a staged rollout from observe to enforce without service interruption.

## Objective

Move policy controls into production safely:

- start with visibility
- validate deterministic policy behavior in CI
- enforce approvals on high-risk operations
- enforce blocks only after evidence is stable

Default rollout sequence:

1. `gait demo --policy`
2. `gait gate eval --policy <policy.yaml> --intent <intent.json> --simulate --json`
3. `gait gate eval --policy <policy.yaml> --intent <intent.json> --json`

## Stage 0: Fixture Baseline In CI

Run deterministic policy fixture tests on every PR:

```bash
gait policy validate examples/policy/base_low_risk.yaml --json
gait policy validate examples/policy/base_medium_risk.yaml --json
gait policy validate examples/policy/base_high_risk.yaml --json
gait policy fmt examples/policy/base_medium_risk.yaml --write --json
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_medium_risk.yaml examples/policy/intents/intent_write.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delete.json --json
gait policy simulate --baseline examples/policy/base_medium_risk.yaml --policy examples/policy/base_high_risk.yaml --fixtures examples/policy/intents --json
```

Rollout gate:

- Do not ship policy changes unless fixture tests pass.
- Use strict-parse failures (`exit 6`) as fail-closed pre-merge blockers.

## Stage 1: Observe (Simulate Only)

Evaluate policy but do not enforce runtime blocking yet:

```bash
gait gate eval --policy examples/policy/base_medium_risk.yaml --intent examples/policy/intents/intent_write.json --simulate --json
```

Interpretation:

- Exit code remains `0` because simulation is non-enforcing.
- Use `verdict`, `reason_codes`, `matched_rule`, and trace outputs for tuning.

Rollout gate:

- Move forward only when false positives are at or below your threshold.
- Use `gait policy simulate` with baseline and candidate policy versions to quantify changed fixture verdicts before switching stages.

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
- Validate both runtime and CI behavior:
  - runtime must block execution until approval token flow completes
  - CI should treat `require_approval` as a blocked promotion signal unless an approved path is part of release criteria

## Stage 3D: Script Governance, Wrkr Context, and Approved Registry

For multi-step scripts, evaluate script payloads directly and wire deterministic context enrichment:

```bash
gait gate eval \
  --policy ./policy.yaml \
  --intent ./script_intent.json \
  --wrkr-inventory ./wrkr_inventory.json \
  --json
```

For explicitly approved script patterns, mint signed entries and verify fast-path allow behavior:

```bash
gait approve-script \
  --policy ./policy.yaml \
  --intent ./script_intent.json \
  --registry ./approved_scripts.json \
  --approver secops \
  --key-mode prod \
  --private-key ./approval_private.key \
  --json

gait list-scripts --registry ./approved_scripts.json --json

gait gate eval \
  --policy ./policy.yaml \
  --intent ./script_intent.json \
  --approved-script-registry ./approved_scripts.json \
  --approved-script-public-key ./approval_public.key \
  --json
```

Rollout gate:

- approved-script entries must be policy-digest bound and signature verified.
- missing/invalid registry state must fail closed in high-risk/oss-prod paths.
- monitor `pre_approved`, `pattern_id`, and `registry_reason` in gate JSON and trace artifacts.

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

## Stage 3C: Delegation Guardrails

For multi-agent execution paths, enforce delegation constraints before full block/allow rollout:

- Require delegation only on scoped high-risk tool classes first.
- Constrain delegator/delegate identities and delegation scope (`write`, `admin`, etc.).
- Fail closed on invalid/missing delegation token evidence for constrained rules.

Suggested fixture gates:

```bash
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delegated_egress_valid.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delegated_egress_invalid.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_tainted_egress.json --json
```

Rollout gate:

- Do not advance to full enforce until delegated-valid, delegated-invalid, and tainted-egress fixtures are stable and deterministic in CI.

Identity boundary note:

- Gait validates signed delegation evidence (identity/scope/digest bindings) presented at eval time.
- Enterprise IdP/OIDC token exchange and identity lifecycle remain external to Gait.
- Integration teams should map IdP-issued identities/claims into delegation token issuance before `gait gate eval`.

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
  - `gait policy validate` on changed policy files
  - `gait policy fmt --write` idempotence check on policy fixtures
  - `gait policy test` fixture suite
  - targeted `gait gate eval --simulate` checks for changed policies
- nightly workflow:
  - broader intent fixture replay against all baseline policy packs
- deployment gate:
  - block rollout if exit `3`, `4`, or `6` appears for required production intents
