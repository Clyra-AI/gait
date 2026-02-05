# Gait

Gait makes production AI agents controllable and debuggable by default with **replayable runpacks**, **deterministic regressions**, and **policy-gated tool calls**.

## Start here (offline, < 60 seconds)

1. Install:
   - (placeholder) `brew install gait` or download a single static binary
2. Run the demo:
   - `gait demo`
3. Verify the run:
   - `gait verify <run_id>`

You will get a ticket footer like:

```
GAIT run_id=<run_id> manifest=sha256:<manifest_hash> verify="gait verify <run_id>"
```

## What you can do next

- **Replay**: `gait run replay <run_id>` (stubbed by default)
- **Diff**: `gait run diff <run_id_A> <run_id_B>`
- **Regress**: `gait regress init --from <run_id>` then `gait regress run`
- **Gate**: `gait policy test <policy.yaml> <intent_fixture.json>`

## Scope (v1)

Runpack, Regress, Gate, Doctor, and a minimal adapter surface. No hosted UI.

