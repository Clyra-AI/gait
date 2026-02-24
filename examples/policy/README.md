# Starter Policy Templates (Epic A4.1)

These templates provide baseline policy packs by risk tier:

- `base_low_risk.yaml`
- `base_medium_risk.yaml`
- `base_high_risk.yaml`
- `baseline-highrisk.yaml` (hyphenated alias)
- `knowledge_worker_safe.yaml` (reversible-first profile)

Scaffold a baseline file directly from the CLI:

```bash
gait policy init baseline-highrisk --out ./gait.policy.yaml --json
```

Each template includes explicit examples for all three verdicts:

- `allow`
- `require_approval`
- `block`
- delegated egress allow/block controls for high-risk write paths
- tainted-data egress blocking for prompt-injection-style payloads

Run from repo root:

```bash
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_write.json --json
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_delete.json --json

gait policy test examples/policy/base_medium_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_medium_risk.yaml examples/policy/intents/intent_write.json --json
gait policy test examples/policy/base_medium_risk.yaml examples/policy/intents/intent_delete.json --json

gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_write.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delete.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_tainted_egress.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delegated_egress_valid.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delegated_egress_invalid.json --json
```

Expected verdict matrix:

- `intent_read.json` => `allow` (exit `0`)
- `intent_write.json` => `require_approval` (exit `4`)
- `intent_delete.json` => `block` (exit `3`)
- `intent_tainted_egress.json` => `block` (exit `3`)
- `intent_delegated_egress_valid.json` => `allow` (exit `0`)
- `intent_delegated_egress_invalid.json` => `block` (exit `3`)

High-risk note:

- `base_high_risk.yaml` marks write actions with `require_broker_credential: true` for least-privilege brokering.
- `base_high_risk.yaml` requires explicit delegation metadata for high-risk egress writes and blocks tainted external payload flow to network destinations.
- `base_high_risk.yaml` and `baseline-highrisk.yaml` include `destructive_budget` defaults to fail-closed once destructive threshold windows are exceeded.
- `knowledge_worker_safe.yaml` defaults unknown tools to block, prefers archive/trash actions, and requires explicit break-glass approval for permanent delete paths.
- For runtime checks in hardened mode, evaluate with `--profile oss-prod` and an explicit broker, for example:

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_write.json \
  --profile oss-prod \
  --key-mode prod \
  --private-key ./prod_signing.key \
  --credential-broker stub \
  --json
```
