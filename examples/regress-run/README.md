# Regress Run Example

This example shows the deterministic incident-to-regression flow.

Run from repo root:

```bash
gait demo
gait capture --from run_demo --json
gait regress add --from ./gait-out/capture.json --json
gait regress run --json
```

Expected behavior:

- `gait regress add` writes:
  - `gait.yaml`
  - `fixtures/run_demo/runpack.zip`
- `gait regress run` writes `regress_result.json`.
- Exit code is `0` for pass, `5` for regression failure.
