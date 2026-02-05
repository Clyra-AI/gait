# Prompt Injection Blocking Example

This example demonstrates deterministic blocking at the tool boundary.

Scenario:

- The model output includes a prompt-injection style instruction to exfiltrate data.
- Intent targets a network host.
- Gate policy blocks host egress for this tool.

Run from repo root:

```bash
gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json
```

Expected behavior:

- Exit code is `3`.
- Verdict is `block`.
- Reason code includes `blocked_prompt_injection`.
