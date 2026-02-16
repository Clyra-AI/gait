---
title: "MCP Canonical Boundary Demo"
description: "Single-command MCP enforcement demo showing allow, block, and require_approval with emitted trace/runpack/pack evidence."
---

# MCP Canonical Boundary Demo

This is the canonical MCP distribution demo for enforcement + evidence.

It produces, in one run:
- `allow`
- `block`
- `require_approval`

For each case it emits:
- `trace_path`
- `runpack_path`
- `pack_path`
- runpack integrity verification output
- pack integrity verification output

## Run

From repo root:

```bash
go build -o ./gait ./cmd/gait
bash scripts/demo_mcp_canonical.sh
```

Default output root:

- `gait-out/mcp_canonical/`

Key outputs:

- `allow_summary.json`
- `block_summary.json`
- `require_approval_summary.json`
- `mcp_canonical_summary.json`
- `traces/*`
- `runpacks/*`
- `packs/*`

## Validate

```bash
bash scripts/test_mcp_canonical_demo.sh
```

Expected contract:

- allow -> `verdict=allow`, `exit_code=0`
- block -> `verdict=block`, `exit_code=3`
- require approval -> `verdict=require_approval`, `exit_code=4`
- all non-allow outcomes are non-executing at caller boundary
- all emitted runpacks verify via `gait verify`
- all emitted packs verify via `gait pack verify`

## Why this is canonical

- Uses `gait mcp serve` as boundary service.
- Uses deterministic policy fixtures under `examples/integrations/template/`.
- Proves enforcement outcomes and evidence emission in the same flow.
