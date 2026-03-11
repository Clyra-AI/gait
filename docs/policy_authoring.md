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
