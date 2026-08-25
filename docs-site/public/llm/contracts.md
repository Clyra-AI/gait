# Gait Contracts

Stable OSS contracts include:

- **PackSpec v1**: Unified portable artifact envelope for run, job, and call evidence with Ed25519 signatures and SHA-256 manifest. Schema: `schemas/v1/pack/manifest.schema.json`.
  - includes first-class export surfaces: `gait pack export --otel-out ...` and `--postgres-sql-out ...` for observability and metadata indexing.
  - `gait pack verify` remains offline-first, but a supplied verify key that produces `signature_status=failed` is a verification failure, not a soft pass.
  - duplicate ZIP entry names are verification failures, even if one duplicate would otherwise hash-match.
- **ContextSpec v1**: Deterministic context evidence envelopes with privacy-aware modes and fail-closed enforcement. Required context-proof checks are satisfied through a verified `--context-envelope` input on `gait gate eval`, `gait mcp proxy`, or `gait mcp serve`, rather than raw intent claims.
- **Primitive Contract**: Four deterministic primitives — capture, enforce, regress, diagnose.
- **CLI Meta Contract**: `gait --help` is text-only and exits `0`; machine-readable version discovery uses `gait version --json` or the `--version` / `-v` aliases.
- **Python SDK Demo Contract**: machine-readable SDK/demo capture consumes `gait demo --json` output only; the human text form is non-contractual.
  - `run_session(...)` delegates digest-bearing runpack fields to `gait run record` in Go rather than hashing them in Python.
  - `gait run record` defaults to `capture_mode=reference`, strips raw `intents[].args` and `results[].result` after digesting, and warns when explicit raw capture is selected.
  - unsupported `set` values and other non-JSON payloads are rejected deterministically.
  - `sdk/python` version metadata is repo-local dev metadata; release/install verification uses `gait version --json`.
- **Doctor Install Contract**: `gait doctor --json` is truthful for a clean writable binary-install lane, returning `status=pass|warn` there and only surfacing repo-only checks from a Gait repo checkout.
- **Repo Policy Contract**: `gait init` writes `.gait.yaml` and returns `detected_signals`, `generated_rules`, and `unknown_signals`; `gait check` reports the live contract with `default_verdict`, `rule_count`, structured `findings`, compatibility `gap_warnings`, and install-safe `next_commands`.
- **Strict `oss-prod` Policy Contract**: `gait gate eval`, `gait mcp proxy`, and `gait mcp serve` reject policies that set `default_verdict: allow`; strict profiles must use `block` or `require_approval` defaults plus explicit allow rules.
- **Draft Proposal Migration Contract**: keep the shipped policy DSL (`schema_id`, `schema_version`, `default_verdict`, optional `fail_closed`, optional `mcp_trust`, `rules`); proposal keys like `version`, `name`, `boundaries`, `defaults`, `trust_sources`, and `unknown_server` return deterministic migration guidance instead of enabling a second DSL.
- **CLI Migration Contract**: use `gait mcp verify` rather than `gait mcp-verify`, and `gait capture --out ...` rather than `gait capture --save-as ...`.
- **Equal-Priority Policy Semantics**: when multiple rules at the same priority match one intent, Gait evaluates that priority tier and applies the most restrictive verdict rather than depending on rule names.
- **MCP Trust + Trace Onboarding**: local MCP trust snapshots and observe-only `gait trace` are additive onboarding contracts over the same signed trace and policy surfaces.
  - `mcp_trust.snapshot` must point at a local file; scanners and registries remain complementary inputs.
  - `gait mcp verify --json` reports `trust_model=local_snapshot` and `snapshot_path` when MCP trust is configured.
  - duplicate normalized MCP identities invalidate the snapshot, and required high-risk trust checks fail closed.
  - wrapper JSON reports `boundary_contract=explicit_trace_reference`, `trace_reference_required=true`, and stable `failure_reason` values such as `missing_trace_reference` and `invalid_trace_artifact`.
- **Script Governance Contract**: Script intent steps, deterministic `script_hash`, Wrkr-derived context matching fields, and signed approved-script registry entries. Fast-path allow requires a verify key; missing verification prerequisites disable fast-path in standard low-risk mode and fail closed in high-risk / `oss-prod` paths.
- **Delegation Contract**: delegated execution is only authoritative when each claimed delegation hop is backed by signed token evidence; multi-hop chains must stay contiguous and terminate at the requester identity, and policy-required delegation scope must come from the token's signed `scope` or signed `scope_class`.
- **Credential Provenance Contract**: high-risk baseline templates can require JIT-only credential provenance for covered write and deploy paths, accepting AWS STS, GitHub OIDC, and Vault-style dynamic credentials while blocking standing PATs, IAM users, inherited env credentials, and unknown provenance with stable reason codes.
- **Freeze Window Contract**: Gate rules can carry policy-owned IANA timezone freeze windows that deterministically `block` or `require_approval` for production-impacting actions, with replayable `--evaluation-time <rfc3339>` support and signed trace output.
- **Sandbox Posture Contract**: high-risk `proc.exec` and generated-code paths can carry schema-backed sandbox metadata so Gate can validate bounded network, filesystem, timeout, env exposure, privilege mode, and sandbox evidence refs/digests without storing raw environment contents.
- **Kill Switch Contract**: Gate and MCP can consume schema-backed generalized kill-switch state for additive emergency stop scopes across matching agents, identities, tools, targets, paths, workspaces, and environments, while preserving job-level emergency stop behavior.
- **Gate Explain JSON Contract**: `gait gate eval --explain --json` returns a schema-backed explanation object with deterministic matched-rule, reason, missing-field, approval, sandbox, freeze-window, and kill-switch status data for machine consumers.
- **Credential Broker Recipe Contract**: provider-style JIT receipts for AWS STS, GitHub OIDC, Vault, GCP, Azure, and Okta/CyberArk-style flows normalize into the shared broker response contract without storing raw secrets.
- **Trust Graduation Contract**: named staged rollout policies can move from observe and dry-run through read-only allow, approval-gated write, brokered write, and blocked destructive defaults while approved-script promotion remains scope- and expiry-bound.
- **Authorization Bundle Contract**: PackSpec now supports an `authorization` subtype that links a Gate trace to approval, credential, delegation, context, sandbox, and outcome evidence with offline verification through `gait pack verify`.
- **Action Contract Boundary**: Gait validates one explicit Wrkr `proposed_action_contract` artifact (artifact schema `1`, contract schema `3`), binds activation to a current-selection manifest and explicit policy/principal/authority/key inputs, and emits a distinct signed `activated_action_contract` plus deterministic consumer receipt. Production signing requires an Ed25519 private key; development signing is test-only and marked. New Wrkr revisions require reactivation.
- **Runtime readiness boundary**: `gait contract classify|readiness|explain` produces a versioned pre-execution projection. Action/boundary/outcome classifications are deterministic and monotonic; resource lifecycle actions are independent of `risk_class`; readiness requires an explicit evaluation time and policy-bound validator public key with a signature over the normalized typed claim digest plus signed, fresh, digest-bound evidence; and no command claims tool execution or observed effects.
- `--action` consumes the strict versioned runtime-action artifact emitted as `classification.action`; raw heuristic input is a separate versioned `--input` schema and cannot be mixed with `--action` or `--proposal`.
- Typed execution/effect/containment/compensation evidence is digest-bound to the Wrkr contract family/revision, Gait activation, runtime/readiness/decision, policy, target, environment, and Proof correlation refs. Lifecycle reduction is deterministic and fail-closed for execution before verified activation/readiness, effects without successful execution, unscoped containment, replay, cross-lineage mismatches, and incomplete required compensation. Gait records/verifies caller evidence and never executes tools.
- The checked-in full-lineage fixture pack is non-authoritative test data. `VerifyLifecycleConformance` verifies proposal and activation signatures, revalidates readiness signatures/freshness at the decision time with caller-supplied trusted validator keys, binds activation policy/target/environment, and checks exact digest-bound lineage, typed evidence signatures, causal ordering, and scenario expectations; missing, stale, replayed, identifier-only, or mismatched links fail closed.
- Consumers must require both `valid` and `authoritative_success` for a success gate. Verified blocked, failed, partial-containment, and unresolved-containment scenarios remain non-success outcomes.
- **Migration:** existing v1 lifecycle records require no conversion and remain valid. New typed fields are required only for their matching new event kinds. Consumers should update to the current v1 schema to process those kinds; older consumers must report unknown kinds as unsupported or inconclusive, never as successful. Auxiliary identifier-only Proof refs remain compatible when the profile content digest supplies the binding, and no persisted artifact is automatically rewritten.
- **Effect Evidence and Contract Boundary**: Gait carries signed versioned `effect_snapshot` before/after evidence for bounded Postgres, filesystem, HTTP, and resource lifecycles and grades typed `effect_contract` `expect`, `forbid`, and `invariant` predicates purely. A trusted collector public key is required for an authoritative pass; `pass`, `fail`, and `inconclusive` are deterministic, and grading never executes an external effect.
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
- `docs/contracts/action_contract_activation.md`
- `schemas/v1/action-contract/README.md`
- `docs/failure_taxonomy_exit_codes.md`
## Action Contract evidence additions

- `gait effects observe` captures one bounded observation; paired `gait effects capture` consumes `--before-observation` and `--after-observation` to produce a complete signed snapshot.
- `gait action-contract advisory evaluate|verify` is advisory-only and offline by default.
- `gait action-contract otel` requires `--trusted-key` and `--source-version`; local signed evidence remains authoritative if export fails.
- `gait regress add --from lifecycle-receipt.json` requires receipt verification, a runpack `--private-key`, and optional explicit `--verify-at`.
- `gait action-contract chain evaluate` advances state only on allow; `gait action-contract circuit evaluate` exits fail-closed for denied chains, non-authoritative effects, unresolved containment, stop/revocation, invalidation, and out-of-scope boundaries.
- Chain policy/state/candidate evaluation is deterministic and pre-execution. Contract-bound approval/delegation/JIT fields are exact-binding extensions; legacy unbound flows remain compatible.
- v1.5.0 released fixtures are distinct from unreleased synthetic stop/revocation/invalidation/out-of-scope control extensions, which are quarantine-only and non-authoritative.
