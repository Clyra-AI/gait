# Regress Run Example

This example shows the deterministic incident-to-regression flow.

Run from repo root:

```bash
gait demo
gait regress init --from run_demo --json
gait regress run --json
```

Expected behavior:

- `gait regress init` writes:
  - `gait.yaml`
  - `fixtures/run_demo/runpack.zip`
- `gait regress run` writes `regress_result.json`.
- Exit code is `0` for pass, `5` for regression failure.
