# SIEM Ingestion Quick Recipes (JSONL And OTEL)

Gait OSS does not ship a dashboard. It emits deterministic artifacts and structured exports so teams can use their existing monitoring stack.

Use `gait mcp proxy` export options:

- `--export-log-out <events.jsonl>` for normalized JSONL event stream
- `--export-otel-out <otel.jsonl>` for OTEL-style JSONL events

## Generate Export Files

```bash
gait mcp proxy \
  --policy examples/policy-test/allow.yaml \
  --call examples/mcp/openai_function_call.json \
  --adapter openai \
  --trace-out ./gait-out/trace_mcp.json \
  --export-log-out ./gait-out/mcp_events.jsonl \
  --export-otel-out ./gait-out/mcp_otel.jsonl \
  --json
```

## Splunk (File Monitor)

Use file monitor on `gait-out/mcp_events.jsonl` and parse as JSON.

Minimal `inputs.conf` example:

```ini
[monitor:///var/data/gait/gait-out/mcp_events.jsonl]
index = gait
sourcetype = gait:events
disabled = false
```

## Datadog (Agent Log Collection)

Configure Datadog agent log source for JSON file:

```yaml
logs:
  - type: file
    path: /var/data/gait/gait-out/mcp_events.jsonl
    service: gait
    source: gait
```

## Elastic (Filebeat)

Minimal Filebeat input:

```yaml
filebeat.inputs:
  - type: filestream
    id: gait-events
    paths:
      - /var/data/gait/gait-out/mcp_events.jsonl
    parsers:
      - ndjson:
          add_error_key: true
```

## Mapping Guidance

Recommended indexed keys:

- `trace_id`
- `run_id`
- `session_id`
- `job_id`
- `phase`
- `tool_name`
- `verdict`
- `reason_codes`
- `policy_digest`
- `intent_digest`
- `decision_latency_ms`
- `delegation_ref`
- `delegation_depth`

Operational alerts worth pinning:

- emergency stop preemption (`reason_codes` includes `emergency_stop_preempted`)
- destructive budget breach (`reason_codes` includes `destructive_budget_exceeded`)

This keeps SIEM queries aligned with Gait artifacts and proofs.
