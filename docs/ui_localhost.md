# Localhost UI (`gait ui`)

The localhost UI is an optional adoption shell over existing CLI commands. It does not replace CLI contracts and it does not add hosted dependencies.

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

## Safety model

- The UI is local-only by default.
- Gate, verify, signing, and policy logic remain in Go CLI/core.
- Non-`allow` outcomes remain non-executable in the UI presentation.

## Troubleshooting

- If UI does not open automatically, copy the printed URL into a browser.
- If a command fails, inspect the `stderr` field in the command output panel.
- If embedded assets are stale, run:

```bash
bash scripts/ui_sync_assets.sh
```
