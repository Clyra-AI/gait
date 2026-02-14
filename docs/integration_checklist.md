# Gait Integration Checklist (v2.3 Blessed Lane)

This checklist is for OSS teams integrating Gait at the tool-call boundary.

Target: first deterministic wrapper integration in <= 15 minutes from clean checkout.

## Lane Governance (Locked for v2.3)

Blessed default lane:

- local coding-agent wrapper flow (`examples/integrations/openai_agents/`)
- GitHub Actions CI regress gate (`.github/workflows/adoption-regress-template.yml`)

Expansion policy (v2.3):

- no new official lane is added unless scorecard threshold is met
- scorecard command:

```bash
python3 scripts/check_integration_lane_scorecard.py \
  --input gait-out/adoption_metrics.json \
  --out gait-out/integration_lane_scorecard.json
```

Decision rule (enforced in docs/review): selected lane score >= `0.75` and confidence delta >= `0.03`.

## Prerequisites

- `gait` available in `PATH` or built from source (`go build -o ./gait ./cmd/gait`)
- Python 3 available for integration quickstarts
- repository fixtures present (`examples/`, `schemas/`)

Optional but recommended:

- export adoption telemetry to measure activation milestones:

```bash
export GAIT_ADOPTION_LOG=./gait-out/adoption.jsonl
```

## 15-Minute Stop/Go Checklist

`STOP` if any expected output is missing.

1. `gait demo` emits `run_id=...` and `ticket_footer=GAIT run_id=...`
2. `gait verify <run_id> --json` returns `ok=true`
3. `gait gate eval ... --json` returns deterministic `verdict`, `intent_digest`, `policy_digest`, `trace_path`
4. wrapper quickstart allow flow returns `executed=true`
5. wrapper quickstart block or approval flow returns `executed=false`
6. `gait regress init --from <run_id> --json` creates fixture
7. `gait regress run --json --junit ...` passes (`status=pass` or exit `0`)
8. context-required gate policy blocks missing context evidence with deterministic reason codes
9. `gait pack inspect --json` exposes context summary when context evidence is attached
10. `gait regress run --context-conformance` is green for expected context baseline

## v2.5 Context-Proof Checklist (Required For High-Risk Paths)

1. Capture mode and evidence mode:

- choose one default:
  - `--context-evidence-mode best_effort` for low-risk migration
  - `--context-evidence-mode required` for fail-closed high-risk paths
- attach context envelope at capture:

```bash
gait run record \
  --input ./run_record.json \
  --context-envelope ./context_envelope.json \
  --context-evidence-mode required \
  --json
```

2. Gate policy wiring:

- include rule constraints:
  - `require_context_evidence: true`
  - `required_context_evidence_mode: required`
  - optional `max_context_age_seconds`
- verify missing context returns deterministic reason codes:
  - `context_evidence_missing`
  - `context_set_digest_missing`

3. Trace verification expectations:

- emit trace for decision events
- verify trace signatures before treating context linkage as evidence:

```bash
gait trace verify ./gait-out/trace.json --public-key ./trace_public.key --json
```

4. Regress context conformance:

- bootstrap fixture from context-bearing runpack
- enforce context drift rules in CI:

```bash
gait regress bootstrap --from <runpack_or_pack> --name context_guard --json
gait regress run --context-conformance --allow-context-runtime-drift --json
```

## Canonical Execution Path (Primary)

1. Emit `IntentRequest`
2. Evaluate Gate
3. Enforce fail-closed for non-`allow`
4. Persist trace
5. Emit runpack
6. Convert runpack into regress fixture

### Minimal Commands (Blessed Path)

From repo root:

```bash
go build -o ./gait ./cmd/gait
python3 examples/integrations/openai_agents/quickstart.py --scenario allow
python3 examples/integrations/openai_agents/quickstart.py --scenario block
python3 examples/integrations/openai_agents/quickstart.py --scenario require_approval
```

Expected behavioral contract:

- allow: `verdict=allow`, `executed=true`
- block: `verdict=block`, `executed=false`
- require approval: `verdict=require_approval`, `executed=false`

Then convert captured run into CI baseline:

```bash
gait demo
gait regress init --from run_demo --json
gait regress run --json --junit ./gait-out/junit.xml
```

## Fail-Closed Chokepoint Rules (Required)

Your dispatch path MUST enforce:

- only `verdict=allow` executes side effects
- `block`, `require_approval`, `dry_run`, invalid payload, or Gate evaluation error return non-executable output

Minimal pattern:

```python
decision = gait_evaluate(tool_call)
if decision["verdict"] != "allow":
    return {"executed": False, "verdict": decision["verdict"]}
return execute_real_tool(tool_call)
```

## CI Mapping (Local -> GitHub Actions)

Use `.github/workflows/adoption-regress-template.yml` directly.

Required parity between local and CI:

- same fixture source (`fixtures/run_demo/runpack.zip` + `gait.yaml` or `gait regress init` fallback)
- same stable exit handling (`0` pass, `5` deterministic failure)
- same retained artifacts (`regress_result.json`, `junit.xml`, fixture files)

See `docs/ci_regress_kit.md` for workflow-call inputs/outputs and downstream reuse.

## Activation Timing Metrics (M1/M2)

Collect local activation report:

```bash
GAIT_ADOPTION_LOG=./gait-out/adoption.jsonl ./gait demo
GAIT_ADOPTION_LOG=./gait-out/adoption.jsonl ./gait verify run_demo --json
GAIT_ADOPTION_LOG=./gait-out/adoption.jsonl ./gait regress init --from run_demo --json
GAIT_ADOPTION_LOG=./gait-out/adoption.jsonl ./gait regress run --json
./gait doctor adoption --from ./gait-out/adoption.jsonl --json
```

Use report fields:

- `activation_timing_ms` for milestone timing deltas (`A1..A4`)
- `activation_medians_ms` for stable command timing medians (M1/M2 calculations)
- `skill_workflows` for skill workflow fitness signals

## Intent + Receipt Conformance

Before calling an integration complete, run contract checks:

```bash
bash scripts/test_intent_receipt_conformance.sh ./gait
```

Contract doc:

- `docs/contracts/intent_receipt_conformance.md`

## Advanced / Secondary Lanes (Reference Only)

These remain supported parity references in v2.3 but are not top-of-funnel:

- `examples/integrations/langchain/`
- `examples/integrations/autogen/`
- `examples/integrations/openclaw/`
- `examples/integrations/autogpt/`
- `examples/integrations/gastown/`
- sidecar path: `examples/sidecar/gate_sidecar.py`
- MCP proxy/serve path: `gait mcp proxy`, `gait mcp serve`

For parity checks across official adapters:

```bash
bash scripts/test_adapter_parity.sh
```

For full adoption smoke (wrapper + sidecar + scenarios):

```bash
bash scripts/test_adoption_smoke.sh
```
