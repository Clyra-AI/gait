# Examples (Offline-Safe)

All examples in this folder run without network, secrets, or cloud accounts.

## Included Paths

- `stub-replay/`: deterministic stub replay flow from a demo runpack
- `policy-test/`: allow/block/require-approval policy evaluation examples
- `policy/`: starter low/medium/high risk policy templates + fixture intents
- `regress-run/`: incident-to-regression fixture workflow
- `prompt-injection/`: deterministic prompt-injection style blocking example
- `scenarios/`: reproducible scenario scripts (incident reproduction, injection block, approval flow)
- `python/`: thin Python adapter example (calls local `gait` binary)
- `ci/gait-regress-failing/`: failing regress workflow example with uploaded diff artifacts
- `sidecar/`: canonical non-Python sidecar boundary for `IntentRequest -> gate eval`
- `integrations/openai_agents/`: wrapped tool path with allow/block + trace outputs
- `integrations/langchain/`: wrapped tool path with allow/block + trace outputs
- `integrations/autogen/`: wrapped tool path with allow/block + trace outputs

## Recommended Order

1. `stub-replay`
2. `policy-test`
3. `policy`
4. `regress-run`
5. `prompt-injection`
6. `scenarios`
7. `integrations/openai_agents`
8. `integrations/langchain`
9. `integrations/autogen`
10. `sidecar`

## Contribution Checklist

Before opening a PR that changes `examples/`:

1. Keep every example offline-safe (no cloud dependencies, no secrets).
2. Include copy/paste commands and expected outputs in the example `README.md`.
3. Ensure every policy path documents expected verdict and reason codes.
4. Verify deterministic behavior by running:

```bash
go build -o ./gait ./cmd/gait
bash scripts/policy_compliance_ci.sh
bash examples/scenarios/incident_reproduction.sh
bash examples/scenarios/prompt_injection_block.sh
bash examples/scenarios/approval_flow.sh
```

For full adapter/policy/fixture contribution standards, see `CONTRIBUTING.md`.
