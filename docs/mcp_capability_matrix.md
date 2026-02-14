# MCP Capability Matrix

This page clarifies what `gait mcp proxy`, `gait mcp bridge`, and `gait mcp serve` do and do not do.

## Adapter Definition

In this context, an adapter is the payload translation layer from a framework schema (`mcp`, `openai`, `anthropic`, `langchain`) into Gait's normalized `IntentRequest` shape for policy evaluation.

## Capability Matrix

| Mode | Primary Use | Input | Output | Persistence | Notable Non-Goals |
| --- | --- | --- | --- | --- | --- |
| `gait mcp proxy` | One-shot local evaluation | Tool-call payload file/stdin + policy | JSON decision + optional trace/runpack exports | Optional trace/runpack/log/otel outputs | Not a long-running service |
| `gait mcp bridge` | Alias of proxy for bridge wording/UX | Same as proxy | Same as proxy | Same as proxy | Not a distinct evaluator |
| `gait mcp serve` | Long-running local HTTP decision service | `POST /v1/evaluate*` JSON request | JSON/SSE/NDJSON decision payload with `exit_code` + verdict | Trace/runpack/session retention controls | Does not execute tools for caller |

## Runtime Enforcement Responsibility

All three modes return decisions only. The caller runtime must still enforce:

```text
if verdict != allow: do not execute side effects
```

## Endpoints (`mcp serve`)

- `POST /v1/evaluate` -> JSON
- `POST /v1/evaluate/sse` -> `text/event-stream`
- `POST /v1/evaluate/stream` -> `application/x-ndjson`

## Security and Hardening Notes

- Default bind is loopback.
- Non-loopback bind should use token auth (`--auth-mode token --auth-token-env`).
- Use strict verdict HTTP status when needed (`--http-verdict-status strict`).
- Bound payload size (`--max-request-bytes`) and retention (`--trace-max-*`, `--runpack-max-*`, `--session-max-*`).

## What MCP Modes Do Not Replace

MCP modes do not replace operator/CI workflows such as:

- `gait regress init` / `gait regress run`
- `gait doctor`
- `gait pack inspect` / `gait pack diff`
- release and CI contract gates

## Related Docs

- `docs/flows.md`
- `docs/agent_integration_boundary.md`
- `docs/integration_checklist.md`
