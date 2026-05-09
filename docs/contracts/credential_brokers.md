# Credential Broker Recipes

Gait stays provider-neutral in its authorization logic, but the credential
layer can normalize provider-style JIT receipt metadata into the shared broker
response contract.

Supported provider-style receipt families:

- AWS STS
- GitHub OIDC
- Vault dynamic credentials
- GCP STS / workload identity federation
- Azure federated credentials
- Okta/CyberArk-style brokering
- generic `command` and `file` style receipts

Contract guarantees:

- normalize provider/source, issuer, scope, TTL, request digest, and
  target/run/job bindings into the same broker response shape
- reject secret-like raw credential values in provider-style fields
- derive a stable credential ref from provider metadata when the receipt omits
  one
- keep tests and examples offline-first with deterministic stubs

Examples:

- `examples/credential-brokers/README.md`
- `examples/credential-brokers/aws_sts_receipt.json`
- `examples/credential-brokers/github_oidc_receipt.json`
- `examples/credential-brokers/vault_dynamic_receipt.json`
- `examples/credential-brokers/gcp_sts_receipt.json`
- `examples/credential-brokers/azure_federated_receipt.json`
- `examples/credential-brokers/okta_cyberark_receipt.json`
