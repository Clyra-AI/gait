# Python Reference Adapter (Epic 9.2)

This example shows the recommended high-risk execution path:

1. Build a typed `IntentRequest` from model output.
2. Evaluate policy with `gait gate eval` through the Python SDK.
3. Execute side effects only after an `allow` verdict.
4. Capture a runpack and create a regress fixture for CI.

Non-negotiable wrapper rules:

- Only wrapped tools are registered with the agent.
- Any non-`allow` decision blocks execution (fail-closed).
- Approval tokens and keys stay outside prompt/model context.

Run from repo root:

```bash
uv run --python 3.13 --directory sdk/python python ../../examples/python/reference_adapter_demo.py
```
