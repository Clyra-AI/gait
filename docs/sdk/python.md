# Python SDK Contract (v1)

The Python SDK (`sdk/python/gait`) is a thin subprocess wrapper over the local `gait` binary.

Important boundary:

- Go CLI/Core is authoritative for policy, signing, canonicalization, verification, and exit semantics.
- Python intentionally does not reimplement those behaviors; it shells out to `gait` and returns structured results.

## Scope

Supported primitives:

- intent capture (`capture_intent`)
- gate evaluation (`evaluate_gate`)
- demo capture (`capture_demo_runpack`)
- regress fixture init (`create_regress_fixture`)
- run capture (`record_runpack`)
- trace copy/validation (`write_trace`)

Non-goals:

- no policy parsing/evaluation in Python
- no alternate artifact canonicalization/signing logic
- no hosted transport dependency

## Runtime Model

The SDK executes commands via `subprocess.run(...)` with a bounded timeout.

- default timeout: `30s`
- JSON-decoding is strict for command responses expected to be JSON
- non-zero exits raise `GaitCommandError` with command, exit code, stdout, and stderr

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
