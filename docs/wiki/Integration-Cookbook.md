# Integration Cookbook

Use one boundary pattern only: wrapper or sidecar. Execute tools only on `allow`.

## Universal Recipe

1. Normalize framework payload to `IntentRequest`.
2. Run `gait gate eval --policy ... --intent ... --json`.
3. Execute side effects only if verdict is `allow`.
4. Persist trace and artifact paths under `gait-out/`.

## Framework Recipes

- OpenAI Agents (blessed lane): `examples/integrations/openai_agents/`
- LangChain (official middleware with optional callback correlation): `examples/integrations/langchain/`
- AutoGen: `examples/integrations/autogen/`
- OpenClaw: `examples/integrations/openclaw/`
- AutoGPT: `examples/integrations/autogpt/`
- Gas Town: `examples/integrations/gastown/`

## Required Validation

```bash
make test-adapter-parity
make test-adoption
```

## Skills and Provenance

- Include `skill_provenance` in intents for skill-driven actions.
- Validate trust and signatures with:

```bash
make test-skill-supply-chain
```

## Canonical Contract Docs

- `docs/contracts/primitive_contract.md`
- `docs/contracts/skill_provenance.md`
- `docs/integration_checklist.md`
