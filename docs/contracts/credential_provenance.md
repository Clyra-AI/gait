# Credential Provenance Contract

High-risk intents can now carry structured credential posture instead of opaque
wrapper-local hints.

Supported intent context fields include:

- `credential_ref`
- `credential_source`
- `credential_access_type`
- `credential_issuer`
- `credential_subject`
- `credential_owner`
- `credential_scopes`
- `credential_ttl_seconds`
- `credential_target_binding`
- `credential_run_binding`
- `credential_job_binding`

Normalized credential sources currently include:

- `github_pat`
- `github_oidc`
- `aws_iam_user`
- `aws_sts`
- `vault_dynamic`
- `gcp_sts`
- `azure_federated`
- `kubernetes_service_account`
- `env`
- `command`
- `stub`
- `unknown`

Policy rules can enforce:

- `block_standing_credentials`
- `allowed_credential_sources`
- `allowed_credential_issuers`
- `allowed_credential_access_types`
- `max_credential_ttl_seconds`
- `require_jit_credential`

The high-risk baseline templates now use those fields to make covered write and
deploy paths JIT-only by default:

- accepted sources: `aws_sts`, `github_oidc`, `vault_dynamic`
- accepted issuers: `sts.amazonaws.com`,
  `token.actions.githubusercontent.com`, `vault.example`
- accepted access type: `jit`
- default TTL ceiling: `900` seconds

Representative fixtures live under `examples/policy/credentials/` for:

- AWS STS allow
- GitHub OIDC allow
- Vault dynamic allow
- GitHub PAT/static block
- AWS IAM user/cloud-admin block
- inherited environment credential block
- unknown provenance block

Representative block reasons:

- `standing_credential_disallowed`
- `credential_not_jit`
- `credential_source_disallowed`
- `credential_access_type_disallowed`
- `credential_issuer_disallowed`
- `credential_ttl_exceeded`
- `credential_target_binding_mismatch`
- `credential_run_binding_mismatch`
- `credential_job_binding_mismatch`

The gate trace and broker credential evidence record keep references and
provenance only. Raw credential bytes remain out of traces and evidence by
default.
