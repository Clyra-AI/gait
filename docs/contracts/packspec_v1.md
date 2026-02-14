# PackSpec v1 Contract

Status: normative for v2.4+ producers/consumers.

PackSpec v1 introduces one portable artifact envelope for both run and job evidence:

- file name: `pack_<pack_id>.zip`
- manifest path: `pack_manifest.json`
- manifest schema: `schemas/v1/pack/manifest.schema.json`
- payload schemas:
  - `schemas/v1/pack/run.schema.json`
  - `schemas/v1/pack/job.schema.json`

## Pack Types

- `pack_type=run`
  - payload: `run_payload.json`
  - source attachment: `source/runpack.zip`
- `pack_type=job`
  - payload: `job_payload.json`
  - source attachments: `job_state.json`, `job_events.jsonl`

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
