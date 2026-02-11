# Cloud Runtime Patterns

This guide defines practical deployment shapes for runtime policy enforcement in cloud environments.

## Core Rule

Gait must run where the execution decision is made.

- Gait returns a decision (`allow`, `block`, `require_approval`, `dry_run`).
- Your runtime enforces execution with one rule: non-`allow` outcomes do not execute side effects.

## Pattern A: VM, Container, Or Kubernetes

Recommended shape:

- Run `gait mcp serve` as a local process or sidecar.
- Agent runtime calls one of:
  - `http://127.0.0.1:<port>/v1/evaluate` (JSON)
  - `http://127.0.0.1:<port>/v1/evaluate/sse` (SSE)
  - `http://127.0.0.1:<port>/v1/evaluate/stream` (NDJSON)
- Runtime executes real tools only when verdict is `allow`.

Example service startup:

```bash
gait mcp serve \
  --policy /etc/gait/policy.yaml \
  --listen 127.0.0.1:8787 \
  --trace-dir /var/lib/gait/traces \
  --runpack-dir /var/lib/gait/runpacks \
  --key-mode prod \
  --private-key /etc/gait/signing.key
```

Caller enforcement pattern:

```python
decision = post_local_evaluate(call_payload)
if decision["verdict"] != "allow":
    return {"executed": False, "verdict": decision["verdict"]}
return execute_real_tool(call_payload)
```

## Pattern B: Serverless (Lambda-Style)

Recommended shape:

- Bundle `gait` binary with the function artifact (or layer).
- Invoke `gait mcp proxy` or `gait gate eval` as a subprocess per request.
- Treat CLI non-zero exits and parse failures as fail-closed non-executable outcomes.

Example invocation shape:

```bash
gait mcp proxy --policy /opt/gait/policy.yaml --call - --adapter mcp --json
```

Use this pattern when long-lived localhost services are not practical.

## Policy, Keys, And Artifact Storage

- Policy: deploy versioned policy files with app config and pin by digest in release metadata.
- Signing keys: use production key sources for production paths; do not use demo keys.
- Traces and runpacks: write to durable storage (persistent volume/object store) and retain by incident policy.

## Fail-Closed Requirements

- If Gait cannot evaluate policy, do not execute tools.
- If decision output is missing or malformed, do not execute tools.
- If approval is required and valid approval is absent, do not execute tools.
- If broker-required credentials cannot be issued, degrade to block.

## Integration Checklist Cross-Reference

- Chokepoint insertion and tests: `docs/integration_checklist.md`
- Runtime flow details: `docs/flows.md`
- Production profile behavior: `docs/slo/runtime_slo.md`
