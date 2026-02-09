# Primitive Contract (Normative)

Status: normative for OSS v1.x producers and consumers.

This document defines the four execution primitives that integrations must use and preserve:

- `IntentRequest`
- `GateResult`
- `TraceRecord`
- `Runpack`

Within a major version, these contracts are backward-compatible.

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
- SHOULD provide `args_digest` and `intent_digest` when available.
- SHOULD provide `skill_provenance` when execution originates from a packaged skill.

Consumer obligations:

- MUST fail closed for high-risk paths when intent cannot be evaluated.
- MUST NOT execute side effects on non-`allow` outcomes.

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

Consumer obligations:

- MUST verify trace integrity when verification is required by policy/profile.
- MUST treat signature verification failure as non-passing.

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

Producer obligations:

- MUST generate byte-stable artifacts for identical inputs.
- MUST use RFC 8785 (JCS) canonicalization for digest-bearing JSON.
- MUST record `capture_mode` (`reference` default, `raw` explicit).

Consumer obligations:

- MUST verify manifest and file digests before trust.
- MUST treat missing required files or digest mismatches as verification failure.

## Interop contract for wrappers and sidecars

- Python wrappers and non-Python sidecars are transport/adoption layers.
- Go core remains authoritative for:
  - canonicalization
  - hashing/signing
  - policy evaluation
  - artifact verification
- Integrations MUST pass through these primitive contracts unchanged in meaning.
