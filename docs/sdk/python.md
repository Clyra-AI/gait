---
title: "Python SDK"
description: "Thin subprocess wrapper over the Gait CLI for Python integrations with zero external dependencies."
---

# Python SDK Contract (v1)

The Python SDK (`sdk/python/gait`) is a thin subprocess wrapper over the local `gait` binary.

Important boundary:

- Go CLI/Core is authoritative for policy, signing, canonicalization, verification, and exit semantics.
- Python intentionally does not reimplement those behaviors; it shells out to `gait` and returns structured results.

## Scope

Supported primitives:

- intent capture (`capture_intent`)
- gate evaluation (`evaluate_gate`)
- demo capture (`capture_demo_runpack`, via `gait demo --json`)
- regress fixture init (`create_regress_fixture`)
- run capture (`record_runpack`)
- trace copy/validation (`write_trace`)
- optional LangChain middleware (`GaitLangChainMiddleware`)

Non-goals:

- no policy parsing/evaluation in Python
- no alternate artifact canonicalization/signing logic
- no hosted transport dependency

## Runtime Model

The SDK executes commands via `subprocess.run(...)` with a bounded timeout.

- default timeout: `30s`
- JSON-decoding is strict for command responses expected to be JSON
- demo capture consumes machine-readable `gait demo --json` output only
- non-zero exits raise `GaitCommandError` with command, exit code, stdout, and stderr

## Migration Note

If existing wrapper automation scraped the human `gait demo` text output, switch it to `capture_demo_runpack(...)` or `gait demo --json`. The human text form is not a stable SDK contract.

## Binary Resolution And Errors

The SDK expects `gait` to be available on `PATH` by default.

Override binary:

- `gait_bin="gait"` (default)
- `gait_bin="/absolute/path/to/gait"`
- `gait_bin=[sys.executable, "wrapper.py"]` (test harnesses)

Missing binary behavior:

- raises `GaitCommandError` with actionable message:
  - install `gait`
  - ensure `PATH` contains `gait`
  - or pass `gait_bin` explicitly

## Minimal Example

```python
from gait import IntentContext, capture_intent, evaluate_gate

intent = capture_intent(
    tool_name="tool.write",
    args={"path": "/tmp/out.txt"},
    context=IntentContext(identity="alice", workspace="/repo", risk_class="high"),
)

decision = evaluate_gate(
    policy_path="examples/policy/base_high_risk.yaml",
    intent=intent,
)

if decision.verdict != "allow":
    raise RuntimeError(f"blocked: {decision.verdict} {decision.reason_codes}")
```

## Rollout Pattern (Observe -> Enforce)

Recommended sequence:

1. run fixture policy tests in CI (`gait policy test`, `gait policy simulate`)
2. evaluate in observe mode (`--simulate`) through integration wrappers
3. enforce non-`allow` as non-executable at the tool boundary

Reference docs:

- `docs/policy_rollout.md`
- `docs/integration_checklist.md`

## Frequently Asked Questions

### Does the Python SDK require Go?

The SDK is a thin subprocess wrapper that calls the local `gait` binary. The Go binary must be installed and on PATH.

### Does the SDK have external dependencies?

No. The Python SDK has zero external runtime dependencies. Dev dependencies (pytest, ruff, mypy) are for development only.

LangChain is opt-in:

- install with `uv sync --extra langchain --extra dev` when you want the middleware surface
- base SDK users do not need LangChain installed

### What Python version is required?

Python 3.11 or higher.

### How do I wrap a tool function with Gait?

Use the `@gate_tool` decorator from the SDK. It automatically evaluates gate policy before executing the tool and records the result.

### How does the official LangChain integration work?

The official surface is `GaitLangChainMiddleware`, and enforcement only happens in `wrap_tool_call`.

- tool name and args are captured into a normal `IntentRequest`
- execution still routes through `ToolAdapter.execute`
- optional `GaitLangChainCallbackHandler` receives additive correlation metadata such as `run_id`, `request_id`, `trace_path`, `policy_digest`, and `intent_digest`
- callback handlers never decide allow or block behavior
- the official example lane is `examples/integrations/langchain/quickstart.py`

Minimal snippet:

```python
from gait import ToolAdapter
from gait.langchain import GaitLangChainMiddleware, GaitLangChainCallbackHandler

adapter = ToolAdapter(policy_path="examples/integrations/langchain/policy_allow.yaml")

middleware = [
    GaitLangChainMiddleware(
        adapter=adapter,
        callback_handler=GaitLangChainCallbackHandler(),
    )
]
```

### What happens if the Gait binary is not found?

The SDK raises a clear error with install instructions and PATH guidance instead of an opaque FileNotFoundError.
