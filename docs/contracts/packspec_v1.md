---
title: "PackSpec v1"
description: "Unified portable artifact envelope for run, job, and call evidence with Ed25519 signatures and SHA-256 manifest."
---

# PackSpec v1 Contract

Status: normative for v2.4+ producers/consumers.

Version semantics:

- PackSpec versioning lives in schema contracts and compatibility docs.
- Evergreen implementation guides should avoid release-tag titles.
- Release-scoped rollout commentary belongs in release plans/changelog, not this contract page.

PackSpec v1 introduces one portable artifact envelope for run, job, and call evidence:

- file name: `pack_<pack_id>.zip`
- manifest path: `pack_manifest.json`
- manifest schema: `schemas/v1/pack/manifest.schema.json`
- payload schemas:
  - `schemas/v1/pack/run.schema.json`
  - `schemas/v1/pack/job.schema.json`
  - `schemas/v1/pack/call.schema.json`

## Pack Types

- `pack_type=run`
  - payload: `run_payload.json`
  - source attachment: `source/runpack.zip`
- `pack_type=job`
  - payload: `job_payload.json`
  - source attachments: `job_state.json`, `job_events.jsonl`
- `pack_type=call`
  - payload: `call_payload.json`
  - source attachments: `callpack_manifest.json`, `call_events.jsonl`, `commitments.jsonl`, `gate_decisions.jsonl`, `speak_receipts.jsonl`, `reference_digests.json`, `source/runpack.zip`

## Determinism Contract

Producers MUST ensure identical inputs produce identical output bytes:

- deterministic zip entry ordering
- fixed zip timestamp epoch
- stable file modes
- canonical JSON for digest-bearing documents (RFC 8785 / JCS)

## Verify Contract

`gait pack verify` MUST run offline and validate:

- manifest schema identity and version
- declared file presence and hash integrity
- undeclared file rejection
- optional signature requirements (`--profile strict` or `--require-signature`)

Stable exit semantics:

- `0`: verification success
- `2`: verification failure (integrity/schema/signature)
- `6`: invalid input/usage

## Diff Contract

`gait pack diff` emits deterministic JSON conforming to `schemas/v1/pack/diff.schema.json`.

Stable exit semantics:

- `0`: no differences
- `2`: differences detected
- `6`: invalid input/usage

## Export Contract

`gait pack export` emits first-class downstream surfaces from a verified pack:

- OTEL-style JSONL records (`--otel-out`) with:
  - one `gait.pack.verify` span
  - one metadata event
  - stable metrics (`tool_calls_success_rate`, `policy_blocked_total`, `approval_required_total`, `regress_failures_total`)
- PostgreSQL metadata/index SQL (`--postgres-sql-out`) with optional table/index DDL and deterministic upsert statements.

Stable exit semantics:

- `0`: export success
- `6`: invalid input/usage

## Legacy Compatibility (v2.4)

PackSpec v1 readers remain backward-compatible with legacy artifacts:

- runpack: `runpack_<id>.zip`
- guard evidence pack: `evidence_pack_<id>.zip`

Compatibility mapping:

- `gait run record` -> legacy runpack producer (still supported)
- `gait guard pack` -> legacy evidence pack producer (still supported)
- `gait pack verify` -> verifies PackSpec v1 + legacy artifacts

Deprecation posture for v2.4:

- No legacy command or artifact removal in v2.4.
- Migration is additive (`run|guard` surfaces continue to work).

## v1 Stability Contract

PackSpec `1.x` preserves consumer compatibility by default.

Non-breaking changes in `1.x`:
- additive optional fields
- additive enum members when readers are required to ignore unknown values
- additive artifact metadata that does not alter hash/signature semantics for existing files

Breaking changes (not allowed in `1.x`):
- removing or renaming required fields
- changing digest/signature canonicalization rules
- changing required file paths for an existing `pack_type`
- changing stable exit-code semantics for verify/diff

Breaking changes require:
- new major schema version
- explicit migration guidance
- compatibility window called out in release notes and matrix docs

## Deprecation Policy

When a surface is replaced:
- old and new surfaces run in parallel for at least one minor release line
- deprecation notices appear in docs and release notes before removal
- `gait pack verify` retains legacy read compatibility for the published window

Reference:
- producer interop path: `docs/contracts/pack_producer_kit.md`
- public compatibility matrix: `docs/contracts/compatibility_matrix.md`

## Frequently Asked Questions

### What is PackSpec v1?

PackSpec v1 is a unified artifact envelope that supports run, job, and call evidence in a single portable format with Ed25519 signatures and a SHA-256 manifest.

### What is the difference between a runpack and a jobpack?

A runpack captures a single agent run. A jobpack captures a durable job lifecycle including checkpoints, pauses, and approvals. Both use the same PackSpec v1 format.

### Can I verify a pack without the signing key?

You can verify schema and hash integrity without the key. Signature verification requires the corresponding public key.

### What does the manifest contain?

The manifest lists every file in the pack with its SHA-256 digest, the pack type, schema version, producer version, and an optional Ed25519 signature.

### Are packs backward-compatible?

Yes. Schema versioning is additive within major 1.x. Readers must tolerate unknown fields. Breaking changes require a major version bump.
