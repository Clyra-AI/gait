---
title: "Integration Checklist"
description: "Step-by-step checklist for integrating Gait with agent frameworks including OpenAI Agents, LangChain, and AutoGen."
---

# Gait Integration Checklist

This checklist is for OSS teams integrating Gait at the tool-call boundary.

Target: first deterministic wrapper integration in <= 15 minutes from clean checkout.

Fast path scenario:

- `docs/scenarios/simple_agent_tool_boundary.md`

## Version Semantics

This checklist is evergreen guidance. Release-specific rollouts belong in plan/changelog docs.

- Pack and verifier compatibility: `docs/contracts/compatibility_matrix.md`
- Pack contract evolution: `docs/contracts/packspec_v1.md`

## Lane Governance (Locked)

Blessed default lane:

- local coding-agent wrapper flow (`examples/integrations/openai_agents/`)
- GitHub Actions CI regress gate (`.github/workflows/adoption-regress-template.yml`)
- one-PR CI adoption target via reusable workflow or composite action

Expansion policy:

- no new official lane is added unless scorecard threshold is met
- scorecard command:

```bash
python3 scripts/check_integration_lane_scorecard.py \
  --input gait-out/adoption_metrics.json \
  --out gait-out/integration_lane_scorecard.json
```

Decision rule: selected lane score >= `0.75` and confidence delta >= `0.03`.

## Prerequisites

- `gait` available on `PATH` or built from source (`go build -o ./gait ./cmd/gait`)
- Python 3 available for wrapper quickstarts
- repository fixtures present (`examples/`, `schemas/`)

Recommended metrics opt-in before first run:

```bash
export GAIT_ADOPTION_LOG=./gait-out/adoption.jsonl
```

## Integration Boundary Decision (Before Core Track)

Choose the highest tier you can support:

1. Full interception at tool boundary (best): wrapper/sidecar/middleware or `gait mcp serve`.
2. Partial interception: enforce where possible and treat uncovered paths as observe-only.
3. No interception (some managed/preloaded agent products): use observe/report/regress workflows; inline fail-closed enforcement is not guaranteed.

Reference: `docs/agent_integration_boundary.md`.

## Core Track (First Integration, Required)

Run these first. Stop if expected output is missing.

1. First artifact:
- `gait demo`
- expect `run_id=...` and `ticket_footer=GAIT run_id=...`
2. Verify artifact:
- `gait verify run_demo --json`
- expect `ok=true`
3. Policy decision shape:
- `gait gate eval ... --json`
- expect deterministic `verdict`, `reason_codes`, `intent_digest`, `policy_digest`, `trace_path`
4. Wrapper allow path:
- run wrapper quickstart allow scenario
- expect `executed=true`
5. Wrapper non-allow path:
- run wrapper block or approval scenario
- expect `executed=false`
6. Regress fixture init:
- `gait regress init --from run_demo --json`
- expect fixture created
7. Regress gate run:
- `gait regress run --json --junit ./gait-out/junit.xml`
- expect `status=pass` and exit `0`
8. CI parity:
- wire `.github/workflows/adoption-regress-template.yml`
- confirm local/CI fixture parity
9. Activation timing report:
- run `gait doctor adoption --from ./gait-out/adoption.jsonl --json`
- confirm `activation_timing_ms` and `activation_medians_ms` present
10. Observe->enforce rollout baseline:
- observe: `gait gate eval ... --simulate --json`
- enforce: `gait gate eval ... --json`

## Advanced Track (Hardening and Scale)

Run after Core Track is green.

### Context-Proof High-Risk Requirements

1. Capture mode and evidence mode:

```bash
gait run record \
  --input ./run_record.json \
  --context-envelope ./context_envelope.json \
  --context-evidence-mode required \
  --json
```

2. Gate policy constraints:
- include `require_context_evidence: true`
- include `required_context_evidence_mode: required`
- optional `max_context_age_seconds`
- verify deterministic reason codes:
- `context_evidence_missing`
- `context_set_digest_missing`

3. Trace signature verification:

```bash
gait trace verify ./gait-out/trace.json --public-key ./trace_public.key --json
```

4. Regress context conformance:

```bash
gait regress bootstrap --from <runpack_or_pack> --name context_guard --json
gait regress run --context-conformance --allow-context-runtime-drift --json
```

### Durable Job + Pack Runtime Validation

1. `gait demo --durable --json` should end with `job_status=completed`.
2. `gait pack inspect ./gait-out/pack_job_demo_durable.zip --json` should parse successfully.
3. Execute full lifecycle manually for production wiring:
- `gait job submit`, `gait job checkpoint add`, `gait job approve`, `gait job resume`, `gait job inspect`

### Conformance And Contract Gates

Run these before declaring an integration complete:

```bash
bash scripts/test_intent_receipt_conformance.sh ./gait
make test-packspec-tck
make test-context-conformance
```

Contract docs:

- `docs/contracts/intent_receipt_conformance.md`
- `docs/contracts/packspec_tck.md`

### Adapter Parity / Secondary Lanes

Supported references:

- `examples/integrations/langchain/`
- `examples/integrations/autogen/`
- `examples/integrations/openclaw/`
- `examples/integrations/autogpt/`
- `examples/integrations/gastown/`
- `examples/integrations/voice_reference/`
- sidecar path: `examples/sidecar/gate_sidecar.py`
- MCP proxy/serve: `gait mcp proxy`, `gait mcp serve`

Parity checks:

```bash
bash scripts/test_adapter_parity.sh
bash scripts/test_adoption_smoke.sh
```

## Canonical Execution Path

1. Emit `IntentRequest`
2. Evaluate Gate
3. Enforce fail-closed for non-`allow`
4. Persist trace
5. Emit runpack
6. Convert runpack into regress fixture

### Minimal Commands (Blessed Path)

```bash
go build -o ./gait ./cmd/gait
python3 examples/integrations/openai_agents/quickstart.py --scenario allow
python3 examples/integrations/openai_agents/quickstart.py --scenario block
python3 examples/integrations/openai_agents/quickstart.py --scenario require_approval
python3 examples/integrations/voice_reference/quickstart.py --scenario allow
python3 examples/integrations/voice_reference/quickstart.py --scenario block
python3 examples/integrations/voice_reference/quickstart.py --scenario require_approval
```

Expected contract:

- allow: `verdict=allow`, `executed=true`
- block: `verdict=block`, `executed=false`
- require approval: `verdict=require_approval`, `executed=false`

Voice boundary addendum:

- allow: `speak_emitted=true` and `callpack_path` emitted
- block/require approval: `speak_emitted=false` and no gated speech side effects

## Fail-Closed Chokepoint Rules (Required)

Your dispatch path MUST enforce:

- only `verdict=allow` executes side effects
- `block`, `require_approval`, `dry_run`, invalid payload, or evaluation error returns non-executable output

Minimal pattern:

```python
decision = gait_evaluate(tool_call)
if decision["verdict"] != "allow":
    return {"executed": False, "verdict": decision["verdict"]}
return execute_real_tool(tool_call)
```

## CI Mapping (Local -> GitHub Actions)

Default path (GitHub Actions):

- use `.github/workflows/adoption-regress-template.yml` directly for one-PR adoption
- optional step-level control: `.github/actions/gait-regress/action.yml`

Required parity:

- same fixture source (`fixtures/run_demo/runpack.zip` + `gait.yaml` or `gait regress init` fallback)
- same stable exit handling (`0` pass, `5` deterministic failure)
- same retained artifacts (`regress_result.json`, `junit.xml`, fixture files)

Non-GitHub portability path:

- run `bash scripts/ci_regress_contract.sh` as the provider-agnostic CI contract
- copy provider template from:
  - `examples/ci/portability/gitlab/.gitlab-ci.yml`
  - `examples/ci/portability/jenkins/Jenkinsfile`
  - `examples/ci/portability/circleci/config.yml`

See `docs/ci_regress_kit.md` for workflow/action contracts and portability guidance.

## Frequently Asked Questions

### How long does integration take?

First deterministic wrapper integration targets 15 minutes. Full production rollout with policy, CI, and delegation typically takes 30-120 minutes.

### Does Gait work with LangChain?

Yes. See `examples/integrations/langchain/` for a maintained adapter that wraps tool calls through gate evaluation.

### What if I cannot intercept tool calls?

If your agent runtime is fully hosted with no interception point, Gait can still provide observe/report/regress value from exported traces. Full enforcement requires a tool boundary interception point.

### Which integration path should I start with?

The blessed lane is OpenAI Agents (`examples/integrations/openai_agents/`). All adapters follow the same wrapper pattern and emit the same artifacts.

### Do I need to modify my agent code?

You add a wrapper at the tool dispatch boundary. The wrapper calls `gait gate eval` before executing the tool and records the result. The agent logic itself does not change.
