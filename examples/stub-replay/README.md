# Stub Replay Example

This example shows deterministic stub replay from a local runpack.

Run from repo root:

```bash
gait demo
gait run replay run_demo --json
gait run replay run_demo --json
```

Expected behavior:

- Both replay calls return `mode: "stub"`.
- Both replay outputs are identical for the same runpack.
- No network or external tools are required.
