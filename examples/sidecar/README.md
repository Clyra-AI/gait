# Gait Sidecar (Canonical Non-Python Integration)

Use this sidecar when your runtime is not using the Python SDK wrapper.

Contract:

- Input: normalized `IntentRequest` (`gait.gate.intent_request` `1.0.0`) from file or stdin
- Execution: `gait gate eval --json`
- Output: JSON envelope containing:
  - `gate_result` (the `gait gate eval` JSON payload)
  - `trace_path`
  - `exit_code`

For MCP boundaries, pair this with `gait mcp verify` when you need a local trust preflight from a scanner or registry snapshot. Keep the boundary split explicit: external system finds, Gait enforces.

## Run from fixture file

```bash
python3 examples/sidecar/gate_sidecar.py \
  --policy examples/policy-test/allow.yaml \
  --intent-file core/schema/testdata/gate_intent_request_valid.json \
  --trace-out ./gait-out/trace_sidecar_allow.json
```

## Run from stdin

```bash
cat core/schema/testdata/gate_intent_request_valid.json | \
python3 examples/sidecar/gate_sidecar.py \
  --policy examples/policy-test/block.yaml \
  --intent-file - \
  --trace-out ./gait-out/trace_sidecar_block.json
```

Expected behavior:

- exit `0`: allow
- exit `4`: require_approval
- exit `3`: block
- non-zero: fail-closed integration path (do not execute side effects)
