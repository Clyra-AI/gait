# Localhost UI (`gait ui`)

The localhost UI is an optional adoption shell over existing CLI commands. It does not replace CLI contracts and it does not add hosted dependencies.

Runtime boundary reminder:

- The UI is not your agent runtime.
- The UI is a local shell over fixed CLI actions.
- Runtime enforcement for real tool calls still requires a wrapper/sidecar/middleware path (or `gait mcp serve`) in front of tool execution.

## Start

```bash
gait ui
```

Defaults:

- bind: `127.0.0.1:7980`
- opens browser automatically
- runs commands in the current working directory

## Common flags

```bash
gait ui --listen 127.0.0.1:7999
gait ui --open-browser=false
```

Non-loopback requires explicit opt-in:

```bash
gait ui --listen 0.0.0.0:7980 --allow-non-loopback
```

## What the UI runs

The UI calls existing commands with `--json` and renders results:

- `gait demo --json`
- `gait verify run_demo --json`
- `gait run receipt --from run_demo --json`
- `gait regress init --from run_demo --json`
- `gait regress run --json --junit ./gait-out/junit.xml`
- `gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delete.json --json`

## Controlled inputs

The UI supports controlled arguments without enabling arbitrary commands:

- `regress_init` accepts `run_id` (validated by pattern).
- `policy_block_test` accepts selected `policy_path` and `intent_path` from allowlisted fixtures.

The command surface remains fixed to the predefined action set.

## Per-click deltas

Each run updates an operator summary:

- last action
- last run timestamp
- changed artifact indicators based on `exists`/`modified_at` deltas

Artifact metadata is exposed in `/api/state` under the `artifacts` field.

## Safety model

- The UI is local-only by default.
- Gate, verify, signing, and policy logic remain in Go CLI/core.
- Non-`allow` outcomes remain non-executable in the UI presentation.
- UI interactions do not automatically instrument third-party hosted agent runtimes.

For a concrete runtime example, use:

- `docs/scenarios/simple_agent_tool_boundary.md`

## Troubleshooting

- If UI does not open automatically, copy the printed URL into a browser.
- If a command fails, inspect the `stderr` field in the command output panel.
- If embedded assets are stale, run:

```bash
bash scripts/ui_sync_assets.sh
```
