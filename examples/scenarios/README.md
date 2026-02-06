# Scenario Pack (Epic A7.1)

This pack provides reproducible, offline-safe scenario checks.

Run from repo root:

```bash
bash examples/scenarios/incident_reproduction.sh
bash examples/scenarios/prompt_injection_block.sh
bash examples/scenarios/approval_flow.sh
```

Each script enforces deterministic expectations:

- incident reproduction: `demo -> regress init -> regress run` succeeds and writes expected artifacts
- prompt injection: policy test returns exit `3` with `blocked_prompt_injection`
- approval flow:
  - missing approval path returns exit `4`
  - scope mismatch path returns exit `4`
  - valid approval path returns exit `0` with `approval_granted`

The key under `examples/scenarios/keys/` is for demo/testing only.
