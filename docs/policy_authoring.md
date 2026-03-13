---
title: "Policy Authoring"
description: "Write, validate, format, test, and simulate YAML policies for agent tool-call enforcement."
---

# Policy Authoring Workflow

Use this workflow to reduce policy rollout mistakes before runtime enforcement.

## Objective

Make policy changes deterministic, reviewable, and low-friction for developers:

- scaffold from a baseline template
- validate syntax and semantics early
- normalize formatting deterministically
- test verdict behavior against intent fixtures

## Recommended Authoring Loop

```bash
gait init --template baseline-mediumrisk --json
gait check --json
gait policy validate .gait.yaml --json
gait policy fmt .gait.yaml --write --json
gait policy test .gait.yaml examples/policy/intents/intent_write.json --json
gait policy simulate --baseline examples/policy/base_medium_risk.yaml --policy .gait.yaml --fixtures examples/policy/intents --json
```

Interpretation:

- `policy validate` checks strict YAML parsing + policy semantics only.
- `policy fmt` rewrites normalized YAML deterministically.
- `policy test` evaluates one intent fixture and returns verdict, reason codes, and `matched_rule`.
- `policy simulate` compares baseline vs candidate verdicts over fixture corpora and recommends rollout stage (`observe`, `require_approval`, `enforce`).

## Equal-Priority Contract

When multiple rules at the same priority match one intent, Gait evaluates the entire matching priority tier and applies the most restrictive verdict from that tier.

- verdict precedence is `allow < dry_run < require_approval < block`
- renaming same-priority rules must not change the verdict
- `matched_rule` remains deterministic and may include a comma-separated set of same-priority matches when more than one rule is visible

If you need one rule to win unconditionally, give it a strictly lower numeric `priority` instead of relying on rule names.

## Repo-Root Policy Contract

The default onboarding contract is the repo-root file `.gait.yaml`.

`gait init --json` writes that file and returns:

- `policy_path`
- `template`
- `next_commands`

`gait check --json` reads `.gait.yaml` and reports the live contract, including:

- `default_verdict`
- `rule_count`
- `gap_warnings`

Use `gait policy init --out gait.policy.yaml --json` only when you intentionally want a non-default path.

## Common Top-Level Fields

The schema for `.gait.yaml` is `schemas/v1/gate/policy.schema.json`.

Most policies start with these fields:

- `schema_id`: must be `gait.gate.policy`
- `schema_version`: currently `1.0.0`
- `default_verdict`: one of `allow`, `block`, `dry_run`, `require_approval`
- `fail_closed`: optional high-risk missing-data rules
- `mcp_trust`: optional local trust-snapshot contract for MCP server admission
- `rules`: ordered rule list

Example:

```yaml
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: block
fail_closed:
  enabled: true
  risk_classes: [critical]
  required_fields: [targets, arg_provenance]
mcp_trust:
  enabled: true
  snapshot: ./examples/integrations/mcp_trust/trust_snapshot.json
  action: block
  required_risk_classes: [high, critical]
  min_score: 0.8
rules:
  - name: require-approval-tool-write
    priority: 20
    effect: require_approval
    min_approvals: 2
    match:
      tool_names: [tool.write]
    reason_codes: [approval_required_for_write]
```

## Common Rule Fields

Rules usually combine:

- `name`, `priority`, and `effect`
- `match.tool_name` or `match.tool_names`
- `match.risk_classes`, `match.target_kinds`, `match.identities`, or other structured selectors
- `reason_codes` and optional `violations`

Additional rule features are available when needed:

- `endpoint` for path/domain and destructive endpoint controls
- `destructive_budget` and `rate_limit` for bounded execution
- `require_context_evidence` for context-proof gating
- `require_broker_credential` for broker-backed approval flows
- script-specific controls via `approved-script-registry` on `gait gate eval`

## Failure Semantics

- exit `0`: valid / allow path
- exit `3`: policy test verdict `block`
- exit `4`: policy test verdict `require_approval`
- exit `6`: invalid input, parse error, unknown field, or invalid schema/value

Treat exit `6` as fail-closed in CI and production rollout lanes.

## IDE Schema Wiring (YAML Language Server)

For editors that support YAML schema mapping, point policy files at:

- `schemas/v1/gate/policy.schema.json`

Example VS Code workspace settings:

```json
{
  "yaml.schemas": {
    "./schemas/v1/gate/policy.schema.json": [
      ".gait.yaml",
      "gait.policy.yaml",
      "examples/policy/**/*.yaml"
    ]
  }
}
```

This gives fast feedback for enum values and unknown keys before runtime.

## Team Workflow Recommendation

- Require `policy validate` + fixture `policy test` in pre-merge CI.
- Run `policy simulate` against representative fixture sets before changing rollout stage.
- Keep policy files formatted by `policy fmt --write` before review.
- Review policy changes with fixture deltas and matched-rule evidence, not raw YAML diff alone.
- Include equal-priority overlap fixtures in CI when multiple rules intentionally target the same tool surface.
- Keep the repo-default contract truthful: if docs say `.gait.yaml`, examples should use `.gait.yaml` unless a custom path is the point of the example.

## Signing Key Lifecycle (Local)

For production verification profiles and trace signing workflows, manage keys with CLI primitives:

```bash
gait keys init --out-dir ./gait-out/keys --prefix prod --json
gait keys rotate --out-dir ./gait-out/keys --prefix prod --json
gait keys verify --private-key ./gait-out/keys/prod_private.key --public-key ./gait-out/keys/prod_public.key --json
```

## Example Policy Packs

- baseline templates: `examples/policy/base_low_risk.yaml`, `examples/policy/base_medium_risk.yaml`, `examples/policy/base_high_risk.yaml`
- endpoint controls: `examples/policy/endpoint/*`
- skill trust controls: `examples/policy/skills/*`
- simple guard fixtures: `examples/policy-test/*`

## Frequently Asked Questions

### What happens if no rule matches a tool call?

The default action applies. In fail-closed mode (`oss-prod` profile), the default is block. In standard mode, the default is configurable per policy.

### Can I test a policy without affecting production?

Yes. Use `gait policy test` against fixture intents, or `gait policy simulate` to compare candidate vs baseline policy across fixtures before deploying.

### What policy formats are supported?

Gait uses YAML policy files with structured rules. Use `gait init` for the additive repo-root onboarding path (`.gait.yaml`), or `gait policy init --out ...` when you want an explicit custom scaffold path such as `gait.policy.yaml`.

### Can I set different rules per tool class?

Yes. Rules can match on tool name, tool class (read, write, delete, admin), endpoint patterns, risk class, and actor identity.

### How do I roll out a policy change safely?

Start with observe mode (dry_run), then require_approval for high-risk actions, then enforce. Use `gait policy simulate` to see verdict deltas before each step.
