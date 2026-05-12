# Authorization Bundle Example

Build an authorization bundle from linked evidence files:

```bash
gait pack build \
  --type authorization \
  --from examples/policy/authorization_bundle/authorization_bundle.json \
  --json

gait pack verify ./pack_<id>.zip --json
```

The sample payload links:

- `trace.json`
- `approval_audit.json`
- `credential_evidence.json`
- `outcome_receipt.json`
