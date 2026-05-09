# Gate Explain JSON Contract

`gait gate eval --explain --json` emits a schema-backed machine-readable
explanation for one Gate decision.

Schema:

- `schemas/v1/gate/policy_explain.schema.json`

Example:

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_write.json \
  --json \
  --explain
```

Contract highlights:

- stable `schema_id` and `schema_version`
- deterministic `matched_rules`, `reason_codes`, `violations`, and
  `missing_fields` ordering
- explicit approval, broker, delegation, context evidence, freeze-window,
  kill-switch, and sandbox state
- proof refs for emitted trace and related audit/evidence artifacts when they
  exist

This explain contract is for machine consumers. Plain `--explain` remains
human-facing help text.
