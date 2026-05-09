# Trust Graduation Examples

These examples show a staged Gate rollout from observe through brokered write
and blocked destructive defaults.

Suggested path:

1. `observe.yaml` with `gait gate eval --simulate`
2. `dry_run.yaml`
3. `read_only_allow.yaml`
4. `approval_gated_write.yaml`
5. `brokered_write.yaml`
6. `blocked_destructive.yaml`

Approved-script promotion example:

```bash
gait approve-script \
  --policy examples/policy/trust_graduation/approval_gated_write.yaml \
  --intent examples/policy/trust_graduation/script_write_intent.json \
  --registry ./approved_scripts.json \
  --approver secops \
  --scope risk:low,workspace_prefix:/repo \
  --key-mode prod \
  --private-key ./examples/scenarios/keys/approval_private.key \
  --json
```
