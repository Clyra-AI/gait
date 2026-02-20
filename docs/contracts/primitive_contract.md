---
title: "Primitive Contract"
description: "Normative contract defining Gait's four deterministic primitives: capture, enforce, regress, and diagnose."
---

# Primitive Contract (Normative)

Status: normative for OSS v1.x producers and consumers.

This document defines the four execution primitives that integrations must use and preserve:

- `IntentRequest`
- `GateResult`
- `TraceRecord`
- `Runpack`

Within a major version, these contracts are backward-compatible.

Intent + receipt continuity conformance profile:

- `docs/contracts/intent_receipt_conformance.md`

## Versioning and compatibility rules

- Producers MUST emit `schema_id` and `schema_version` for every primitive.
- Producers and consumers MUST treat `schema_id` and `schema_version` as contract selectors.
- Required fields for a schema version MUST NOT be removed or renamed without a major version change.
- Semantic changes to required fields MUST NOT occur without a version bump.
- Optional fields MAY be added in minor/patch versions if consumers can ignore unknown fields safely.

## IntentRequest (`gait.gate.intent_request`, `1.0.0`)

Purpose: normalized tool-call intent at the execution boundary.

Required fields:

- `schema_id`
- `schema_version`
- `created_at`
- `producer_version`
- `tool_name`
- `args`
- `targets`
- `context`

Producer obligations:

- MUST normalize and canonicalize values before computing digests.
- MUST provide a non-empty `tool_name`.
- MUST provide `context.identity`, `context.workspace`, and `context.risk_class`.
- MAY include enterprise passthrough context when available:
  - `context.auth_context` (object)
  - `context.credential_scopes` (string array)
  - `context.environment_fingerprint` (string)
- MAY include context-proof linkage fields:
  - `context.context_set_digest` (sha256 hex)
  - `context.context_evidence_mode` (`best_effort|required`)
  - `context.context_refs` (string array of reference ids)
- SHOULD provide `args_digest` and `intent_digest` when available.
- MAY include script intent fields for compound workflows:
  - `script.steps[]` (ordered step list with per-step `tool_name`, `args`, optional `targets`, optional `arg_provenance`)
  - `script_hash` (sha256 hex over canonical script payload)
- SHOULD provide `skill_provenance` when execution originates from a packaged skill.
- SHOULD provide `delegation` when tool execution is delegated across agents:
  - `requester_identity`
  - delegation `chain` links
  - `scope_class`
  - `token_refs`

Consumer obligations:

- MUST fail closed for high-risk paths when intent cannot be evaluated.
- MUST NOT execute side effects on non-`allow` outcomes.

Script mode semantics:

- Producers MUST keep `script.steps[]` order stable because evaluation and digesting are order-sensitive.
- Producers MUST keep step payloads canonicalizable (objects/arrays only, no non-JSON values).
- Consumers MUST evaluate script mode deterministically and preserve stable reason-code ordering.

## GateResult (`gait.gate.result`, `1.0.0`)

Purpose: deterministic policy decision output for one `IntentRequest`.

Required fields:

- `schema_id`
- `schema_version`
- `created_at`
- `producer_version`
- `verdict`
- `reason_codes`
- `violations`

Verdicts:

- `allow`
- `block`
- `dry_run`
- `require_approval`

Producer obligations:

- MUST emit one of the allowed verdicts.
- MUST provide deterministic `reason_codes` ordering.

Consumer obligations:

- MUST execute tool call only when verdict is `allow`.
- MUST block execution on `block`, `require_approval`, `dry_run`, or evaluation error.

## TraceRecord (`gait.gate.trace`, `1.0.0`)

Purpose: signed, auditable decision trace linked to intent and policy digests.

Required fields:

- `schema_id`
- `schema_version`
- `created_at`
- `producer_version`
- `trace_id`
- `tool_name`
- `args_digest`
- `intent_digest`
- `policy_digest`
- `verdict`

Producer obligations:

- MUST bind `verdict` to `intent_digest` and `policy_digest`.
- SHOULD include signature data for tamper-evident traces in production paths.
- SHOULD carry `skill_provenance` through from intent when present.
- SHOULD carry `delegation_ref` when delegated execution evidence is present.
- SHOULD include `observed_at` for runtime wall-clock incident reconstruction.
- SHOULD include `event_id` as a per-emission runtime identity.
- SHOULD include script-governance metadata when script mode is evaluated:
  - `script`, `step_count`, `script_hash`
  - `composite_risk_class`
  - `step_verdicts[]`
  - `pre_approved`, `pattern_id`, `registry_reason`
  - `context_source` when Wrkr enrichment is applied
- SHOULD carry context-proof linkage when present in intent:
  - `context_set_digest`
  - `context_evidence_mode`
  - `context_ref_count`

Consumer obligations:

- MUST verify trace integrity when verification is required by policy/profile.
- MUST treat signature verification failure as non-passing.

Time-field semantics:

- `created_at`: deterministic decision timestamp used in cryptographic binding.
- `observed_at`: operational runtime timestamp for timeline reconstruction only.
- `event_id`: emission identity for retained runtime events; multiple events may share one deterministic `trace_id`.

## Runpack (`gait.runpack.*`, `1.0.0`)

Purpose: deterministic artifact for replay, diff, verify, and regress.

Manifest schema:

- `schema_id`: `gait.runpack.manifest`
- `schema_version`: `1.0.0`
- required fields:
  - `schema_id`
  - `schema_version`
  - `created_at`
  - `producer_version`
  - `run_id`
  - `capture_mode`
  - `files`
  - `manifest_digest`

Archive file contract:

- Runpack zip MUST contain:
  - `manifest.json`
  - `run.json`
  - `intents.jsonl`
  - `results.jsonl`
  - `refs.json`
- Runpack refs MAY include context-proof summary:
  - `context_set_digest`
  - `context_evidence_mode`
  - `context_ref_count`

Producer obligations:

- MUST generate byte-stable artifacts for identical inputs.
- MUST use RFC 8785 (JCS) canonicalization for digest-bearing JSON.
- MUST record `capture_mode` (`reference` default, `raw` explicit).

Consumer obligations:

- MUST verify manifest and file digests before trust.
- MUST treat missing required files or digest mismatches as verification failure.
- MUST treat `context_evidence_mode=required` with missing `context_set_digest` as invalid input.

## PackSpec (`gait.pack.*`, `1.0.0`)

Purpose: unified artifact envelope for `run` and `job` evidence types.

Manifest schema:

- `schema_id`: `gait.pack.manifest`
- `schema_version`: `1.0.0`
- required fields:
  - `schema_id`
  - `schema_version`
  - `created_at`
  - `producer_version`
  - `pack_id`
  - `pack_type` (`run|job`)
  - `source_ref`
  - `contents`

Producer obligations:

- MUST produce deterministic zip bytes for identical inputs.
- MUST include only declared files in `contents`.
- MUST use JCS canonicalization for digest/signature inputs.
- SHOULD include `context_envelope.json` in run packs when context evidence is present.

Consumer obligations:

- MUST verify declared file hashes and reject undeclared files.
- MUST treat schema mismatch and hash mismatch as verification failure.
- SHOULD support legacy runpack/evidence verification during v2.4 compatibility window.
- SHOULD expose deterministic context drift summary in diff outputs when context data exists.

## Command Contract Additions (v2.5)

- `gait run record`:
  - `--context-envelope <path>`
  - `--context-evidence-mode best_effort|required`
  - `--unsafe-context-raw`
- `gait regress run`:
  - `--context-conformance`
  - `--allow-context-runtime-drift`
- `gait pack diff --json`:
  - emits `context_drift_classification`, `context_changed`, `context_runtime_only_changes` when applicable

## Command Contract Additions (Script Governance)

- `gait gate eval`:
  - supports script-intent evaluation with deterministic step rollup metadata in JSON output
  - supports Wrkr enrichment via `--wrkr-inventory <inventory.json>`
  - supports signed pre-approved fast-path via:
    - `--approved-script-registry <registry.json>`
    - `--approved-script-public-key <path>` or `--approved-script-public-key-env <VAR>`
- `gait approve-script`:
  - creates signed approved-script entries bound to policy digest + script hash
- `gait list-scripts`:
  - lists registry entries with expiry/active status

## Session Chain Artifacts (`gait.runpack.session_*`, `1.0.0`)

Purpose: append-only, crash-tolerant long-running capture with verifiable checkpoints.

Artifacts:

- `SessionJournal` (`gait.runpack.session_journal`)
- `SessionCheckpoint` (`gait.runpack.session_checkpoint`)
- `SessionChain` (`gait.runpack.session_chain`)

Producer obligations:

- MUST append session events with monotonic sequence IDs.
- MUST link checkpoints using `prev_checkpoint_digest`.
- MUST preserve deterministic checkpoint runpack emission for identical journal prefixes.

Consumer obligations:

- MUST verify linkage continuity and per-checkpoint runpack integrity for chain-level trust.
- MUST treat missing checkpoints or digest-link mismatch as verification failure.

## Interop contract for wrappers and sidecars

- Python wrappers and non-Python sidecars are transport/adoption layers.
- Go core remains authoritative for:
  - canonicalization
  - hashing/signing
  - policy evaluation
  - artifact verification
- Integrations MUST pass through these primitive contracts unchanged in meaning.

## Enterprise consumer compatibility guard

- Enterprise control-plane consumers are downstream readers of OSS artifacts.
- Enterprise consumers MUST treat OSS primitive contracts as append-only within major `v1.x`.
- Enterprise consumers MUST ignore additive unknown fields.
- OSS MUST remain executable without any enterprise control plane dependency.

CI proof point:

- `scripts/test_ent_consumer_contract.sh` generates live v1.7 artifacts (`runpack`, `trace`, `regress_result`, `signal_report`) and validates deterministic enterprise-style ingestion plus additive-field tolerance.
