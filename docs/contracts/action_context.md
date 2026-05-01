# Action Context Contract

High-risk `gait gate eval` intents can now carry explicit execution context in
`context` instead of burying authorization inputs inside ad hoc wrapper state.

Key fields:

- `agent_id`
- `agent_identity`
- `run_id`
- `workflow_id`
- `repo`
- `environment`
- `credential_ref`
- `credential_source`
- `credential_access_type`
- `credential_issuer`
- `credential_ttl_seconds`
- `approval_ref`
- `wrkr_inventory_ref`
- `agent_action_bom_ref`

Fail-closed policy can require these fields with:

```yaml
fail_closed:
  enabled: true
  risk_classes: [high]
  required_high_risk_fields: [agent_id, run_id, repo]
```

Raw credential material is not allowed in intent `args` or `context.auth_context`
when a credential reference or digest should be used instead.

These fields are normalized into the intent digest and emitted on the signed gate
trace so downstream proof and audit tooling can verify the exact authorization
inputs that were evaluated.
