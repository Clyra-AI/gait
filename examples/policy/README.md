# Starter Policy Templates (Epic A4.1)

These templates provide baseline policy packs by risk tier:

- `base_low_risk.yaml`
- `base_medium_risk.yaml`
- `base_high_risk.yaml`

Each template includes explicit examples for all three verdicts:

- `allow`
- `require_approval`
- `block`

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
```

Expected verdict matrix:

- `intent_read.json` => `allow` (exit `0`)
- `intent_write.json` => `require_approval` (exit `4`)
- `intent_delete.json` => `block` (exit `3`)

High-risk note:

- `base_high_risk.yaml` marks write actions with `require_broker_credential: true` for least-privilege brokering.
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
