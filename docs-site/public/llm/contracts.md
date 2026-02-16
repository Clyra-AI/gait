# Gait Contracts

Stable OSS contracts include:

- **PackSpec v1**: Unified portable artifact envelope for run, job, and call evidence with Ed25519 signatures and SHA-256 manifest. Schema: `schemas/v1/pack/manifest.schema.json`.
  - includes first-class export surfaces: `gait pack export --otel-out ...` and `--postgres-sql-out ...` for observability and metadata indexing.
- **ContextSpec v1**: Deterministic context evidence envelopes with privacy-aware modes and fail-closed enforcement.
- **Primitive Contract**: Four deterministic primitives — capture, enforce, regress, diagnose.
- **Intent+Receipt Spec**: Structured tool-call intent with deterministic receipt generation.
- **Endpoint Action Model**: Maps tool-call intent to policy-evaluated action outcomes.
- Artifact schemas (`schemas/v1/*`)
- Stable CLI exit codes (`0` success, `1` internal/runtime failure, `2` verification failure, `3` policy block, `4` approval required, `5` regression drift, `6` invalid input, `7` dependency missing, `8` unsafe operation blocked)
- Backward-compatible readers within major version
- Deterministic zip entry ordering, fixed timestamps, canonical JSON (RFC 8785 / JCS)

Version semantics:

- Contract versioning lives in schema and compatibility documents.
- Evergreen guides should avoid release tags in titles.
- Release-lane rollout notes belong in release plans/changelog docs.

References:

- `docs/contracts/packspec_v1.md`
- `docs/contracts/compatibility_matrix.md`
- `docs/contracts/pack_producer_kit.md`
- `docs/contracts/contextspec_v1.md`
- `docs/contracts/primitive_contract.md`
- `docs/contracts/intent_receipt_spec.md`
- `docs/contracts/endpoint_action_model.md`
- `docs/failure_taxonomy_exit_codes.md`
