# Sandbox Policy Examples

These fixtures show sandbox posture enforcement for high-risk `proc.exec`.

```bash
gait gate eval \
  --policy examples/policy/sandbox/allow_sandboxed_exec.yaml \
  --intent examples/policy/sandbox/intent_proc_exec_valid.json \
  --json

gait gate eval \
  --policy examples/policy/sandbox/allow_sandboxed_exec.yaml \
  --intent examples/policy/sandbox/intent_proc_exec_missing_sandbox.json \
  --json

gait gate eval \
  --policy examples/policy/sandbox/allow_sandboxed_exec.yaml \
  --intent examples/policy/sandbox/intent_proc_exec_permissive_network.json \
  --json
```

Expected outcomes:

- `intent_proc_exec_valid.json` => `allow`
- `intent_proc_exec_missing_sandbox.json` => `block`
- `intent_proc_exec_permissive_network.json` => `block`
