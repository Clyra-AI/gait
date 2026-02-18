# Claude Code Quickstart

This integration demonstrates two first-party paths for Claude Code:

- wrapper parity quickstart (`quickstart.py`)
- PreToolUse hook interception (`gait-gate.sh`)

Both paths normalize Claude tool names to Gait generic names (`tool.read`, `tool.write`, `tool.exec`, `tool.delegate`) before policy evaluation.

## Run Wrapper Parity

From repo root:

```bash
go build -o ./gait ./cmd/gait
python3 examples/integrations/claude_code/quickstart.py --scenario allow
python3 examples/integrations/claude_code/quickstart.py --scenario block
python3 examples/integrations/claude_code/quickstart.py --scenario require_approval
```

Expected outputs:

- allow: `verdict=allow`, `executed=true`
- block: `verdict=block`, `executed=false`
- require approval: `verdict=require_approval`, `executed=false`

Trace locations:

- `gait-out/integrations/claude_code/trace_allow.json`
- `gait-out/integrations/claude_code/trace_block.json`
- `gait-out/integrations/claude_code/trace_require_approval.json`

## Run Hook Path

Default policy: `examples/policy/base_high_risk.yaml`

```bash
echo '{"session_id":"abc123","tool_name":"Bash","tool_input":{"command":"npm test"},"hook_event_name":"PreToolUse"}' \
  | examples/integrations/claude_code/gait-gate.sh
```

Expected decision mapping:

- gait `allow` -> Claude `permissionDecision=allow`
- gait `block` -> Claude `permissionDecision=deny`
- gait `require_approval` -> Claude `permissionDecision=ask`

By default hook errors fail open (`allow`). Set `GAIT_CLAUDE_STRICT=1` for fail-closed (`deny`) on hook/runtime errors.
