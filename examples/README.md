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
