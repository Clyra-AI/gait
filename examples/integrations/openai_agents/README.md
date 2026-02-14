# OpenAI Agents Quickstart (Blessed v2.3 Lane)

This is the top-of-funnel integration lane for v2.3:

- local wrapper enforcement for OpenAI-style tool calls
- direct mapping to GitHub Actions regress gate

Use this when you want the clearest runtime-boundary example of where Gait sits between agent and tool execution.

Related onboarding:

- `docs/scenarios/simple_agent_tool_boundary.md`
- `docs/agent_integration_boundary.md`

## Wrapper Contract

1. normalize framework tool call into `IntentRequest`
2. evaluate with `gait gate eval`
3. execute tool only on `allow`
4. persist deterministic trace path

## Run

From repo root:

```bash
go build -o ./gait ./cmd/gait
python3 examples/integrations/openai_agents/quickstart.py --scenario allow
python3 examples/integrations/openai_agents/quickstart.py --scenario block
python3 examples/integrations/openai_agents/quickstart.py --scenario require_approval
```

Expected outputs:

- allow: `verdict=allow`, `executed=true`
- block: `verdict=block`, `executed=false`
- require approval: `verdict=require_approval`, `executed=false`

Trace locations:

- `gait-out/integrations/openai_agents/trace_allow.json`
- `gait-out/integrations/openai_agents/trace_block.json`
- `gait-out/integrations/openai_agents/trace_require_approval.json`

## Local -> CI Regress Mapping

```bash
gait demo
gait regress init --from run_demo --json
gait regress run --json --junit ./gait-out/junit.xml
```

Then use `.github/workflows/adoption-regress-template.yml` in CI.

Detailed CI contract: `docs/ci_regress_kit.md`.
