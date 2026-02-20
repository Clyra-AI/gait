# Gait Scenario Fixtures

These scenarios validate Gate and Pack behavior from externally-authored fixtures.

Each scenario directory contains:

- `README.md` for intent and rationale
- input artifacts (`policy.yaml`, `intent.json`, `intents.jsonl`, token files, etc.)
- expected artifacts (`expected.yaml` or `expected-verdicts.jsonl`)
- optional `flags.yaml` for execution options

The suite includes baseline policy/pack/delegation/approval scenarios plus script-governance fixtures for:

- script threshold approvals and deterministic script metadata
- script max-step and mixed-risk policy controls
- Wrkr inventory fail-closed behavior in high-risk contexts
- approved-script registry signature mismatch fail-closed behavior
