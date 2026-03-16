# Gait Contracts

Stable OSS contracts include:

- **PackSpec v1**: Unified portable artifact envelope for run, job, and call evidence with Ed25519 signatures and SHA-256 manifest. Schema: `schemas/v1/pack/manifest.schema.json`.
  - includes first-class export surfaces: `gait pack export --otel-out ...` and `--postgres-sql-out ...` for observability and metadata indexing.
  - `gait pack verify` remains offline-first, but a supplied verify key that produces `signature_status=failed` is a verification failure, not a soft pass.
- **ContextSpec v1**: Deterministic context evidence envelopes with privacy-aware modes and fail-closed enforcement. Required context-proof checks are satisfied through a verified `--context-envelope` input on `gait gate eval`, `gait mcp proxy`, or `gait mcp serve`, rather than raw intent claims.
- **Primitive Contract**: Four deterministic primitives — capture, enforce, regress, diagnose.
- **CLI Meta Contract**: `gait --help` is text-only and exits `0`; machine-readable version discovery uses `gait version --json` or the `--version` / `-v` aliases.
- **Python SDK Demo Contract**: machine-readable SDK/demo capture consumes `gait demo --json` output only; the human text form is non-contractual.
- **Doctor Install Contract**: `gait doctor --json` is truthful for a clean writable binary-install lane, returning `status=pass|warn` there and only surfacing repo-only checks from a Gait repo checkout.
- **Repo Policy Contract**: `gait init` writes `.gait.yaml` and returns `detected_signals`, `generated_rules`, and `unknown_signals`; `gait check` reports the live contract with `default_verdict`, `rule_count`, structured `findings`, compatibility `gap_warnings`, and install-safe `next_commands`.
- **Draft Proposal Migration Contract**: keep the shipped policy DSL (`schema_id`, `schema_version`, `default_verdict`, optional `fail_closed`, optional `mcp_trust`, `rules`); proposal keys like `version`, `name`, `boundaries`, `defaults`, `trust_sources`, and `unknown_server` return deterministic migration guidance instead of enabling a second DSL.
- **CLI Migration Contract**: use `gait mcp verify` rather than `gait mcp-verify`, and `gait capture --out ...` rather than `gait capture --save-as ...`.
- **Equal-Priority Policy Semantics**: when multiple rules at the same priority match one intent, Gait evaluates that priority tier and applies the most restrictive verdict rather than depending on rule names.
- **MCP Trust + Trace Onboarding**: local MCP trust snapshots and observe-only `gait trace` are additive onboarding contracts over the same signed trace and policy surfaces.
  - `mcp_trust.snapshot` must point at a local file; scanners and registries remain complementary inputs.
  - `gait mcp verify --json` reports `trust_model=local_snapshot` and `snapshot_path` when MCP trust is configured.
  - wrapper JSON reports `boundary_contract=explicit_trace_reference`, `trace_reference_required=true`, and stable `failure_reason` values such as `missing_trace_reference` and `invalid_trace_artifact`.
- **Script Governance Contract**: Script intent steps, deterministic `script_hash`, Wrkr-derived context matching fields, and signed approved-script registry entries. Fast-path allow requires a verify key; missing verification prerequisites disable fast-path in standard low-risk mode and fail closed in high-risk / `oss-prod` paths.
- **Delegation Contract**: delegated execution is only authoritative when each claimed delegation hop is backed by signed token evidence; multi-hop chains must stay contiguous and terminate at the requester identity, and policy-required delegation scope must come from the token's signed `scope` or signed `scope_class`.
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
