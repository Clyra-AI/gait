# Gait Integration Checklist (Epic A3.3)

This checklist is for application teams integrating Gait at the tool-call boundary.

Target: complete first integration in 30 to 120 minutes (team and framework dependent).

## Scope

This checklist verifies that a repository has the minimum integration needed for deterministic control:

- tool registration boundary control
- fail-closed wrapper or sidecar enforcement
- endpoint-class intent mapping
- skill provenance propagation
- trace persistence
- runpack recording and verification
- CI regression enforcement

## Prerequisites

- `gait` is available in `PATH`
- Python example dependencies are available (`uv`)
- Repo contains the `examples/` fixtures from this project

Optional for reduced flag repetition:

- `.gait/config.yaml` defaults (see `docs/project_defaults.md`)

## Step 1: Tool Boundary Registration

Required outcome:

- Agents can access only wrapped tools, not raw side-effecting executors.

Validation:

- Confirm your tool registry/factory exports wrapped callables only.
- Confirm every side-effecting tool call path flows through `ToolAdapter.execute(...)`.

Evidence to capture:

- Link to adapter or tool registry file in your repo.

## Step 1B: Enforce At The Dispatch Chokepoint

Required outcome:

- The exact code path that executes side effects performs a Gate decision check first.
- The real tool executor is unreachable unless verdict is `allow`.

Implementation guidance:

- Identify the concrete dispatch method/function in your runtime (for example `ToolAdapter.execute`, `dispatch_tool`, worker action handler).
- Insert one pre-execution guard that evaluates intent through Gait (`gait gate eval`, `gait mcp proxy`, or `gait mcp serve`).
- Return non-executable output for `block`, `require_approval`, `dry_run`, invalid payloads, or evaluation failures.

Minimal pattern:

```python
decision = gait_evaluate(tool_call)
if decision["verdict"] != "allow":
    return {"executed": False, "verdict": decision["verdict"]}
return execute_real_tool(tool_call)
```

Validation:

- Unit test: dispatcher does not call executor when verdict is `block`.
- Unit test: dispatcher does not call executor when verdict is `require_approval`.
- Unit test: dispatcher does not call executor when Gate evaluation errors.

Evidence to capture:

- Exact path and line for chokepoint guard insertion (for example `src/agent/dispatch.py:87`).
- Test output proving executor is not reachable on non-`allow` decisions.

## Step 2: Wrapper Enforcement Semantics

Required outcome:

- Execution occurs only on explicit `allow`.
- `block`, `require_approval`, invalid decision, and evaluation failure do not execute side effects.
- `dry_run` does not execute side effects.
- Intents include explicit endpoint metadata (`operation`, `endpoint_class`) for side-effecting paths.
- Intents include `skill_provenance` when execution is skill-triggered.
- Intents SHOULD carry enterprise passthrough context when available:
  - `context.auth_context`
  - `context.credential_scopes`
  - `context.environment_fingerprint`

Canonical wrapper path:

- Use `sdk/python/gait/adapter.py` (`ToolAdapter.execute`) as the reference behavior.
- Treat integration-specific examples (`examples/integrations/*`) as adapter-specific usage only.

Validation command:

```bash
uv run --python 3.13 --directory sdk/python python ../../examples/python/reference_adapter_demo.py
```

Evidence to capture:

- Command output showing `gate verdict=allow executed=True` for allow flow.
- File and line reference where non-`allow` verdicts return non-executable responses.
- Local test output showing fail-closed cases are covered:

```bash
(cd sdk/python && PYTHONPATH=. uv run --python 3.13 --extra dev pytest tests/test_adapter.py tests/test_client.py -q)
```

## Step 2B: Framework Adapter Parity

Required outcome:

- Framework adapters produce the same contract behavior (`verdict`, `executed`, trace paths, fail-closed semantics).

Validation command:

```bash
bash scripts/test_adapter_parity.sh
```

Covered adapters:

- `openai_agents`
- `langchain`
- `autogen`
- `openclaw`
- `autogpt`
- `gastown`

## Step 2C: OpenClaw Installable Skill Path

Required outcome:

- OpenClaw teams can install one official Gait boundary package with one command.

Validation commands:

```bash
bash scripts/install_openclaw_skill.sh --json
bash scripts/test_openclaw_skill_install.sh
```

Evidence to capture:

- Installed path includes `gait_openclaw_gate.py`, `skill_manifest.json`, and `skill_config.json`.
- Skill entrypoint is executable and policy path is configured.

## Step 2D: Gas Town Worker Hook

Required outcome:

- Gas Town worker actions pass through one fail-closed Gait wrapper path.

Canonical artifact path:

- `examples/integrations/gastown/`

Validation commands:

```bash
python3 examples/integrations/gastown/quickstart.py --scenario allow
python3 examples/integrations/gastown/quickstart.py --scenario block
```

Evidence to capture:

- Deterministic traces under `gait-out/integrations/gastown/`.
- `executed=true` only when verdict is `allow`.

## Step 2A: Sidecar Enforcement (Non-Python Runtimes)

Required outcome:

- Non-Python runtimes can route tool intents through one sidecar command with fail-closed behavior.

Canonical sidecar path:

- `examples/sidecar/gate_sidecar.py`

Validation commands:

```bash
python3 examples/sidecar/gate_sidecar.py --policy examples/policy-test/allow.yaml --intent-file core/schema/testdata/gate_intent_request_valid.json --trace-out ./gait-out/trace_sidecar_allow.json
python3 examples/sidecar/gate_sidecar.py --policy examples/policy-test/block.yaml --intent-file core/schema/testdata/gate_intent_request_valid.json --trace-out ./gait-out/trace_sidecar_block.json
```

Evidence to capture:

- JSON output includes `gate_result`, `trace_path`, and `exit_code`.
- Blocked or approval-required decisions are treated as non-executable paths.
- If using `gait mcp serve`, validate your chosen transport endpoint (`/v1/evaluate`, `/v1/evaluate/sse`, or `/v1/evaluate/stream`) returns the same non-`allow` enforcement semantics.

## Step 3: Gate Trace Persistence

Required outcome:

- Gate decisions produce persisted trace artifacts for audit and replay linkage.

Validation command:

```bash
gait gate eval --policy examples/policy-test/block.yaml --intent examples/policy-test/intent.json --trace-out ./gait-out/trace_check.json --json
```

Evidence to capture:

- `./gait-out/trace_check.json` exists and is non-empty.

## Step 4: Runpack Recording And Verification

Required outcome:

- Team can create and verify deterministic run artifacts locally.

Validation commands:

```bash
gait demo
gait verify run_demo --json
```

Evidence to capture:

- `./gait-out/runpack_run_demo.zip`
- Successful `verify` result.

## Step 5: CI Regression Path

Required outcome:

- At least one deterministic regression fixture runs in CI and can emit JUnit.

Canonical CI path:

- `.github/workflows/adoption-regress-template.yml`
- `docs/ci_regress_kit.md`

Validation commands:

```bash
gait regress init --from run_demo --json
gait regress run --json --junit=./gait-out/junit.xml
```

Evidence to capture:

- `gait.yaml` and `fixtures/` created.
- `./gait-out/junit.xml` generated.

## Step 6: Policy Regression Guard

Required outcome:

- Policy behavior changes are detectable before merge.

Validation commands:

```bash
gait policy test examples/policy-test/allow.yaml examples/policy-test/intent.json --json
gait policy test examples/policy-test/block.yaml examples/policy-test/intent.json --json
gait policy test examples/policy-test/require_approval.yaml examples/policy-test/intent.json --json
```

Evidence to capture:

- Exit codes map to expected decisions (`0`, `3`, `4`).

## Step 7: Integration Sign-Off

Mark integration ready only when all are true:

- Wrapped-tools-only registration is in place.
- Wrapper enforces fail-closed execution semantics.
- Trace persistence is configured.
- Runpack record/verify is reproducible.
- CI runs deterministic regress fixtures.
- Policy tests are part of pre-merge checks.

Recommended release gate:

- Block production rollout if any step above fails.
