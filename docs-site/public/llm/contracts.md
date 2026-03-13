# Gait Contracts

Stable OSS contracts include:

- **PackSpec v1**: Unified portable artifact envelope for run, job, and call evidence with Ed25519 signatures and SHA-256 manifest. Schema: `schemas/v1/pack/manifest.schema.json`.
  - includes first-class export surfaces: `gait pack export --otel-out ...` and `--postgres-sql-out ...` for observability and metadata indexing.
- **ContextSpec v1**: Deterministic context evidence envelopes with privacy-aware modes and fail-closed enforcement. For `gait gate eval`, required context-proof checks are satisfied through a verified `--context-envelope` input rather than raw intent claims.
- **Primitive Contract**: Four deterministic primitives — capture, enforce, regress, diagnose.
- **Repo Policy Contract**: `gait init` writes `.gait.yaml`; `gait check` reports the live contract (`default_verdict`, `rule_count`, `gap_warnings`).
- **Equal-Priority Policy Semantics**: when multiple rules at the same priority match one intent, Gait evaluates that priority tier and applies the most restrictive verdict rather than depending on rule names.
- **MCP Trust + Trace Onboarding**: local MCP trust snapshots and observe-only `gait trace` are additive onboarding contracts over the same signed trace and policy surfaces.
  - `mcp_trust.snapshot` must point at a local file; scanners and registries remain complementary inputs.
- **Script Governance Contract**: Script intent steps, deterministic `script_hash`, Wrkr-derived context matching fields, and signed approved-script registry entries.
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
