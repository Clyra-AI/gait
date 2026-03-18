---
title: "Integration Checklist"
description: "Step-by-step checklist for integrating Gait at the tool boundary with the shipped OpenAI Agents lane, official LangChain middleware, and reference adapters."
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
- do not name additional frameworks publicly (for example CrewAI) until the scorecard threshold is met and an in-repo lane exists
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

## Boundary Touchpoints To Wire

- Wrapper or sidecar: call `gait gate eval` immediately before the real tool side effect.
- If you use `gait test`, `gait enforce`, or `gait trace`, the child integration must emit `trace_path=<path>`; wrapper JSON exposes `boundary_contract=explicit_trace_reference` and stable `failure_reason` values when that seam is missing or invalid.
- Context-required policies: pass `--context-envelope <context_envelope.json>` from the local capture boundary; on `gait mcp serve`, either pin that envelope at server startup or explicitly enable same-host request paths before accepting `call.context.context_envelope_path`. Raw intent context fields are not authoritative by themselves.
- SDK or CI automation: use `gait demo --json` for machine-readable smoke checks and handoff metadata.
- Python run-session capture should pass raw normalization data to `gait run record` and let Go compute digest-bearing artifact fields; do not hash portable artifact digests in the wrapper.
- MCP trust snapshots must use unique normalized `server_id` / `server_name` identities; duplicates are invalid and high-risk trust checks fail closed.
- CI regression loop: persist the trace or runpack from that same boundary, then wire `gait regress bootstrap --from ... --json --junit ...`.

## Core Track (First Integration, Required)

Run these first. Stop if expected output is missing.

1. First artifact:
- operator path: `gait demo`
- machine path: `gait demo --json`
- expect either the human `run_id=...` / `ticket_footer=GAIT run_id=...` summary or JSON `ok=true` with `run_id`
2. Verify artifact:
- `gait verify run_demo --json`
- expect `ok=true`
- duplicate ZIP entry names in runpacks or packs are verification failures, not ambiguous soft passes
3. Policy decision shape:
- `gait gate eval ... --json`
- expect deterministic `verdict`, `reason_codes`, `intent_digest`, `policy_digest`, `trace_path`
- for script payloads also expect stable `script_hash`, `step_count`, `step_verdicts`
4. Wrapper allow path:
- run wrapper quickstart allow scenario
- expect `executed=true`
5. Wrapper non-allow path:
- run wrapper block or approval scenario
- expect `executed=false`
- if `gait test` / `gait enforce` / `gait trace` are used around that quickstart, expect `boundary_contract=explicit_trace_reference` and a stable `failure_reason` when no trace seam is present
6. Explicit capture path:
- `gait capture --from run_demo --json`
- `gait regress add --from ./gait-out/capture.json --json`
- expect deterministic fixture created
7. Regress gate run:
- `gait regress run --json --junit ./gait-out/junit.xml`
- expect `status=pass` and exit `0`
8. One-command shortcut:
- `gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml`
- expect the same stable regress contract without a separate capture handoff
9. CI parity:
- wire `.github/workflows/adoption-regress-template.yml`
- confirm local/CI fixture parity
10. Activation timing report:
- run `gait doctor adoption --from ./gait-out/adoption.jsonl --json`
- confirm `activation_timing_ms` and `activation_medians_ms` present
11. Observe->enforce rollout baseline:
- observe: `gait gate eval ... --simulate --json`
- enforce: `gait gate eval ... --json`
12. Approved script registry path (if script automation is used):
- mint entry: `gait approve-script --policy <policy.yaml> --intent <script_intent.json> --registry <registry.json> --approver <id> --key-mode prod --private-key <path> --json`
- inspect entry: `gait list-scripts --registry <registry.json> --json`
- enforce with registry: `gait gate eval ... --approved-script-registry <registry.json> --approved-script-public-key <path> --json`

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
- pass `--context-envelope <context_envelope.json>` on `gait gate eval` or `gait mcp proxy` whenever those constraints are expected to hold at enforcement time
- for `gait mcp serve`, either start the server with `--context-envelope <context_envelope.json>` or, when `--allow-client-artifact-paths` is explicitly enabled, send `call.context.context_envelope_path`
- do not rely on raw `intent.context_set_digest` or `context.auth_context.context_age_seconds` claims to satisfy fail-closed gate checks
- verify deterministic reason codes:
- `context_evidence_missing`
- `context_set_digest_missing`
- `context_evidence_mode_mismatch`
- `context_freshness_exceeded`

Wrkr inventory enrichment (optional, local-file only):

- add `--wrkr-inventory <inventory.json>` on gate eval
- map inventory metadata into explicit policy match keys:
  - `context_tool_names`
  - `context_data_classes`
  - `context_endpoint_classes`
  - `context_autonomy_levels`

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

Official lanes:

- `examples/integrations/openai_agents/`
- `examples/integrations/langchain/` (official middleware with optional callback correlation)

Reference adapters:

- `examples/integrations/autogen/`
- `examples/integrations/openclaw/`
- `examples/integrations/autogpt/`
- `examples/integrations/gastown/`
- `examples/integrations/voice_reference/`
- `examples/integrations/claude_code/` (reference adapter; hook/runtime/input errors fail closed by default, `GAIT_CLAUDE_UNSAFE_FAIL_OPEN=1` is an explicit unsafe override)
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
python3 examples/integrations/langchain/quickstart.py --scenario allow
python3 examples/integrations/langchain/quickstart.py --scenario block
python3 examples/integrations/langchain/quickstart.py --scenario require_approval
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

- same fixture source (`fixtures/run_demo/runpack.zip` + `gait.yaml` or the explicit `gait capture` + `gait regress add` path)
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

Yes. See `examples/integrations/langchain/` for the official middleware lane. The correct wording is middleware with optional callback correlation; callbacks do not decide allow or block outcomes.

### What if I cannot intercept tool calls?

If your agent runtime is fully hosted with no interception point, Gait can still provide observe/report/regress value from exported traces. Full enforcement requires a tool boundary interception point.

### Which integration path should I start with?

The blessed lane is OpenAI Agents (`examples/integrations/openai_agents/`). LangChain is the official middleware lane. Other adapters are reference parity lanes on the same contract.

### Do I need to modify my agent code?

You add a wrapper at the tool dispatch boundary. The wrapper calls `gait gate eval` before executing the tool and records the result. The agent logic itself does not change.
