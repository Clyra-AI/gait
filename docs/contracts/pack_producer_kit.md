---
title: "Pack Producer Kit"
description: "Minimal producer path for emitting PackSpec v1 artifacts without adopting the full Gait runtime."
---

# Pack Producer Kit (Minimal)

This kit is for maintainers who want to emit PackSpec v1 artifacts without adopting Gait runtime dispatch.

## Scope

The producer kit covers:
- deterministic zip emission
- PackSpec v1 manifest + payload shape
- hash integrity compatibility with `gait pack verify`

It does not cover:
- policy evaluation
- runtime orchestration
- regression execution

## Reference Emitter

Use the minimal producer script:

```bash
python3 scripts/pack_producer_kit.py \
  --out ./gait-out/producer_kit/pack_run_minimal.zip \
  --run-id run_producer_kit_demo \
  --created-at 2026-01-01T00:00:00Z
```

Then verify with Gait consumer contract:

```bash
gait pack verify ./gait-out/producer_kit/pack_run_minimal.zip --json
```

Expected: `ok=true`.

Determinism check:

```bash
bash scripts/test_pack_producer_kit.sh
```

## Determinism Rules

Producers MUST:
- sort zip entries deterministically
- use stable zip timestamps
- use stable file modes
- hash bytes exactly as written into archive
- compute `pack_id` from canonicalized manifest with empty `pack_id` and no signatures
- avoid duplicate ZIP entry names; `gait pack verify` treats duplicate names as verification failure

## Required Files (`pack_type=run`)

- `pack_manifest.json`
- `run_payload.json`
- `source/runpack.zip`

## Interop Contract

If a producer emits the required schema and hash contract, Gait consumers verify it with:

- `gait pack verify`
- `gait pack inspect`
- `gait pack diff`

Duplicate entry names are not a valid interop variant; consumers fail closed on ambiguous archives.

This enables artifact-format adoption without runtime lock-in.
