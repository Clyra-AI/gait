---
title: "ContextSpec v1"
description: "Deterministic context evidence envelopes for run capture, gate enforcement, pack diff, and regression conformance."
---

# ContextSpec v1 Contract

Status: normative for v2.5+ producers and consumers.

ContextSpec v1 defines deterministic context evidence for run capture, gate enforcement, pack inspect/diff, and regress conformance.

Schemas:

- `schemas/v1/context/envelope.schema.json`
- `schemas/v1/context/reference_record.schema.json`
- `schemas/v1/context/budget_report.schema.json`

## Envelope Contract

Schema identity:

- `schema_id`: `gait.context.envelope`
- `schema_version`: `1.0.0`

Required fields:

- `schema_id`
- `schema_version`
- `created_at`
- `producer_version`
- `context_set_id`
- `context_set_digest`
- `evidence_mode` (`best_effort|required`)
- `records[]`

Required record fields:

- `ref_id`
- `source_type`
- `source_locator`
- `query_digest` (sha256 hex)
- `content_digest` (sha256 hex)
- `retrieved_at`
- `redaction_mode`
- `immutability` (`unknown|mutable|immutable`)

Optional record fields:

- `freshness_sla_seconds`
- `sensitivity_label`
- `retrieval_params`

## Determinism Rules

- Producers MUST canonicalize digest-bearing JSON with RFC 8785 / JCS.
- `context_set_digest` MUST be deterministic for equivalent normalized records.
- Record ordering MUST NOT change digest outcome.
- Envelope validation MUST fail on digest mismatch.

## Safety and Enforcement Rules

- `evidence_mode=required` means missing/invalid context evidence blocks high-risk execution paths.
- Raw context evidence requires explicit unsafe operator intent:
  - `gait run record --unsafe-context-raw`
- Gate policies may enforce:
  - `require_context_evidence`
  - `required_context_evidence_mode: required`
  - `max_context_age_seconds`

Stable reason-code surface:

- `context_evidence_missing`
- `context_set_digest_missing`
- `context_evidence_mode_mismatch`
- `context_freshness_exceeded`

## Trace Binding Rules

Signed trace records may include:

- `context_set_digest`
- `context_evidence_mode`
- `context_ref_count`

Tampering with context linkage fields MUST fail signature verification.

## Wrkr Context Enrichment Contract

Wrkr enrichment is an optional local-file context source for gate matching.

Accepted inventory keys per entry:

- `tool_name`
- `data_class`
- `endpoint_class`
- `autonomy_level`

Normalization and matching rules:

- tool names and class values are normalized to lowercase during load.
- inventory is local-file only and does not require network.
- policy matching keys are explicit:
  - `context_tool_names`
  - `context_data_classes`
  - `context_endpoint_classes`
  - `context_autonomy_levels`

Fail-closed expectations:

- missing/invalid inventory blocks in high-risk contexts (`risk_class` high/critical) and oss-prod profile.
- lower-risk contexts may continue without enrichment and should emit an explicit warning.

## CLI Contract Examples

Capture with required context evidence:

```bash
gait run record \
  --input ./run_record.json \
  --context-envelope ./context_envelope.json \
  --context-evidence-mode required \
  --json
```

Fail-closed context policy evaluation:

```bash
gait gate eval --policy ./policy.yaml --intent ./intent.json --json
```

Pack inspect context summary:

```bash
gait pack inspect ./pack_<id>.zip --json
```

Deterministic context drift signal:

```bash
gait pack diff ./pack_a.zip ./pack_b.zip --json
```

Regression context conformance gate:

```bash
gait regress run --context-conformance --allow-context-runtime-drift --json
```

## Compatibility Policy

- ContextSpec v1 fields are additive to existing v1 contracts.
- v1 consumers MUST ignore unknown optional fields.
- v2.5 producers MUST remain backward-compatible with v1.0.0 envelope and record schemas.

## Frequently Asked Questions

### What is a context envelope?

A context envelope is a deterministic JSON artifact that captures what context material the model was working from, with configurable privacy modes (metadata, hashes, or raw).

### When does context evidence fail-closed?

When policy requires context evidence for a high-risk action class and the evidence is missing or stale, the gate blocks execution with an explicit reason code.

### What privacy modes are available?

Three modes: metadata-only (file names), hashes (SHA-256 digests), and raw (full content). Raw requires explicit unsafe flags.

### How does context drift classification work in pack diff?

Pack diff separates semantic drift (different context inputs) from runtime-only drift (same context, different execution). This reduces noise in CI regressions.

### Does context evidence add latency?

Context envelopes are computed during capture, not gate evaluation. The overhead is bounded by the configured evidence mode and budget limits.
