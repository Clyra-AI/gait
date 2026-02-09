# Awesome Gait Ecosystem

This index is the public discovery surface for adapters, skills, and policy packs that follow the Gait primitive contract.

Source of truth:

- `docs/ecosystem/community_index.json`

Validation:

- `python3 scripts/validate_community_index.py`

## Official Integrations

- OpenAI Agents: `examples/integrations/openai_agents/`
- LangChain: `examples/integrations/langchain/`
- AutoGen: `examples/integrations/autogen/`
- OpenClaw: `examples/integrations/openclaw/`
- AutoGPT: `examples/integrations/autogpt/`

## Official Skills

- `gait-capture-runpack`
- `gait-incident-to-regression`
- `gait-policy-test-rollout`

## Contribution Rules

- Every entry must be deterministic, offline-safe by default, and execution-boundary enforced.
- Every adapter entry must pass `bash scripts/test_adapter_parity.sh` behavior.
- Every skill entry must declare provenance and avoid direct policy logic in non-Go layers.
- Every entry must include a public GitHub repo URL and a stable summary.

See `docs/ecosystem/contribute.md` for the full submission workflow.
