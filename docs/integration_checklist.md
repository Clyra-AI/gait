# Gait Integration Checklist (Tiered v2.6)

This checklist is for OSS teams integrating Gait at the tool-call boundary.

Target: first deterministic wrapper integration in <= 15 minutes from clean checkout.

## Lane Governance (Locked for v2.3+)

Blessed default lane:

- local coding-agent wrapper flow (`examples/integrations/openai_agents/`)
- GitHub Actions CI regress gate (`.github/workflows/adoption-regress-template.yml`)

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

### Context-Proof (v2.5) High-Risk Requirements

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

### Durable Job + Pack Runtime (v2.4) Validation

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
```

Expected contract:

- allow: `verdict=allow`, `executed=true`
- block: `verdict=block`, `executed=false`
- require approval: `verdict=require_approval`, `executed=false`

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

Use `.github/workflows/adoption-regress-template.yml` directly.

Required parity:

- same fixture source (`fixtures/run_demo/runpack.zip` + `gait.yaml` or `gait regress init` fallback)
- same stable exit handling (`0` pass, `5` deterministic failure)
- same retained artifacts (`regress_result.json`, `junit.xml`, fixture files)

See `docs/ci_regress_kit.md` for workflow-call inputs/outputs.
