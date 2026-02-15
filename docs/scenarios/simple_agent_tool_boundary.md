---
title: "Simple Scenario"
description: "One end-to-end scenario showing a Gait-gated tool call with allow, block, and require_approval outcomes."
---

# Simple Agent Tool-Boundary Scenario

This is the canonical end-to-end scenario showing where Gait sits between an agent runtime and tool execution.

## What This Demonstrates

- intent emitted at tool boundary
- deterministic policy decision (`allow`, `block`, `require_approval`)
- trace artifact emission
- runpack/pack evidence path
- regress fixture + CI guardrail conversion

## Tool Boundary (Canonical Definition)

A tool boundary is the exact call site where your runtime is about to execute a real tool side effect.

- input to boundary: structured `IntentRequest`
- decision surface: `gait gate eval` (or `gait mcp serve`)
- hard rule: non-`allow` means non-execute

Where to inspect this in code:

- wrapper flow: `examples/integrations/openai_agents/quickstart.py`
- gate command wiring: `cmd/gait/gate.go`
- policy evaluation engine: `core/gate/`

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

## Frequently Asked Questions

### What does a Gait-gated tool call look like end to end?

The agent declares an intent, the adapter sends it to `gait gate eval`, the gate returns a verdict (allow/block/require_approval), and only allow executes the real tool. A signed trace is emitted for every outcome.

### What artifacts does Gait produce from a single tool call?

A gate trace (signed JSON), and optionally a runpack or pack if recording is enabled. The trace alone is sufficient for audit and regression.

### What happens on a block verdict?

The tool does not execute. The adapter returns a structured non-execution result to the agent. The signed trace records the block with matched rule and reason codes.

### Can I run this scenario without an agent framework?

Yes. Use `gait gate eval --policy <policy.yaml> --intent <intent.json> --json` directly from the command line to see the full evaluation flow.

### How do I convert this scenario into a CI test?

Run the scenario once, then `gait regress bootstrap --from <run_id>`. The fixture becomes a permanent CI gate that fails on drift.
