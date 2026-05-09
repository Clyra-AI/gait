# Broker Receipt Contract

Brokered credential issuance uses a typed request/receipt contract for `stub`,
`env`, and `command` providers.

Request schema:

- `schemas/v1/gate/broker_request.schema.json`

Broker request fields include:

- `tool_name`
- `identity`
- `workspace`
- `session_id`
- `request_id`
- `run_id`
- `job_id`
- `reference`
- `scope`
- `target_binding`

Broker credential evidence records can carry:

- broker name and request reference
- credential source, access type, issuer, subject, and owner
- scope
- credential ref
- target, run, and job binding
- request digest
- issued/expires timestamps and TTL

`gait gate eval` now blocks broker-required paths when receipt validation shows:

- missing or malformed broker output
- scope mismatch
- credential ref mismatch
- request digest mismatch
- target binding mismatch
- run binding mismatch
- job binding mismatch
- TTL above policy

The high-risk baseline templates pair receipt validation with JIT-only provider
allowlists for covered write and deploy paths. Operator examples and fixtures
use AWS STS, GitHub OIDC, and Vault-style dynamic credentials, while static
PATs, IAM user credentials, inherited environment credentials, and unknown
provenance remain blocked.

These receipt details are emitted into the broker credential evidence JSON and
referenced from the signed gate trace so later proof-bundle work can link the
authorization decision to the brokered JIT credential evidence.
