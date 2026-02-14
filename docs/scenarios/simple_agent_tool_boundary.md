# Simple Agent Tool-Boundary Scenario

This is the canonical end-to-end scenario showing where Gait sits between an agent runtime and tool execution.

## What This Demonstrates

- intent emitted at tool boundary
- deterministic policy decision (`allow`, `block`, `require_approval`)
- trace artifact emission
- runpack/pack evidence path
- regress fixture + CI guardrail conversion

## Preconditions

```bash
go build -o ./gait ./cmd/gait
export GAIT_ADOPTION_LOG=./gait-out/adoption.jsonl
```

## Step 1: Run Wrapper Scenarios (Agent Context)

```bash
python3 examples/integrations/openai_agents/quickstart.py --scenario allow
python3 examples/integrations/openai_agents/quickstart.py --scenario block
python3 examples/integrations/openai_agents/quickstart.py --scenario require_approval
```

Expected:

- allow -> `executed=true`
- block -> `executed=false`
- require approval -> `executed=false`

Artifacts:

- `gait-out/integrations/openai_agents/trace_allow.json`
- `gait-out/integrations/openai_agents/trace_block.json`
- `gait-out/integrations/openai_agents/trace_require_approval.json`

## Step 2: Produce and Verify Portable Evidence

```bash
gait demo
gait verify run_demo --json
```

Expected:

- `run_id=run_demo`
- verified runpack under `gait-out/runpack_run_demo.zip`

## Step 3: Convert Incident/Evidence to Regression

```bash
gait regress init --from run_demo --json
gait regress run --json --junit ./gait-out/junit.xml
```

Expected:

- deterministic `status=pass` on known-good fixture
- CI-ready `junit.xml`

## Optional: One-Shot Policy Check From Fixture Intent

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_delete.json \
  --trace-out ./gait-out/trace_delete.json \
  --json
```

Expected:

- deterministic verdict and reason codes
- signed trace at `./gait-out/trace_delete.json`

## Operational Rule

Only execute side effects when verdict is `allow`. All other outcomes are non-executable.

## Related Docs

- `docs/flows.md`
- `docs/integration_checklist.md`
- `docs/demo_output_legend.md`
