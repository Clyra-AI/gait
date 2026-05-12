# Kill Switch Examples

These examples show generalized kill-switch state applied to Gate and MCP
surfaces.

```bash
gait kill-switch add \
  --state ./.gait-out/kill_switch_state.json \
  --identity alice \
  --tool-name tool.exec \
  --reason "incident freeze" \
  --actor secops \
  --json

gait gate eval \
  --policy examples/policy/kill_switch/allow_exec.yaml \
  --intent examples/policy/kill_switch/intent_exec.json \
  --kill-switch-state examples/policy/kill_switch/kill_switch_state_identity.json \
  --json
```

Expected outcome:

- matching kill-switch entries force `block`
