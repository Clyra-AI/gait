# Freeze Window Examples

These fixtures show deterministic freeze-window evaluation for production deploy
paths.

```bash
gait gate eval \
  --policy examples/policy/freeze_windows/production_block.yaml \
  --intent examples/policy/freeze_windows/intent_prod_deploy.json \
  --evaluation-time 2026-03-10T14:30:00Z \
  --json

gait gate eval \
  --policy examples/policy/freeze_windows/production_require_approval.yaml \
  --intent examples/policy/freeze_windows/intent_prod_deploy.json \
  --evaluation-time 2026-03-10T14:30:00Z \
  --json
```

Expected outcomes:

- `production_block.yaml` => `block`
- `production_require_approval.yaml` => `require_approval`
