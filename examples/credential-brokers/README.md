# Credential Broker Recipes

These fixtures show provider-style JIT credential receipts that Gait can
normalize without storing raw secrets.

Example command-broker shape:

```bash
gait gate eval \
  --policy ./examples/policy/base_high_risk.yaml \
  --intent ./examples/policy/credentials/intent_aws_sts_allow.json \
  --credential-broker command \
  --credential-command sh \
  --credential-command-args ./examples/credential-brokers/print_receipt.sh,./examples/credential-brokers/aws_sts_receipt.json \
  --json
```

Included provider-style stub receipts:

- `aws_sts_receipt.json`
- `github_oidc_receipt.json`
- `vault_dynamic_receipt.json`
- `gcp_sts_receipt.json`
- `azure_federated_receipt.json`
- `okta_cyberark_receipt.json`

The receipts contain refs and metadata only: provider/source, issuer, scope,
TTL, request/binding evidence, and credential refs or derivable receipt
identifiers.
