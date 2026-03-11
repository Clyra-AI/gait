# MCP Trust Example

This example shows the bounded MCP trust path:

- external tool produces a local file
- a deterministic trust snapshot is rendered from that file
- `gait mcp verify` and `gait mcp proxy` consume the local snapshot
- Gait enforces; the scanner or registry remains the evidence source

## Files

- `server_github.json`: local MCP server description
- `snyk_mcp_report.json`: sample third-party trust input
- `trust_snapshot.json`: rendered local trust snapshot used by Gait

## Render Snapshot

```bash
python3 scripts/render_mcp_trust_snapshot.py \
  --input examples/integrations/mcp_trust/snyk_mcp_report.json \
  --output examples/integrations/mcp_trust/trust_snapshot.json
```

## Verify Server Trust

```bash
gait mcp verify \
  --policy examples/integrations/mcp_trust/policy.yaml \
  --server examples/integrations/mcp_trust/server_github.json \
  --json
```

## Proxy Evaluation

```bash
gait mcp proxy \
  --policy examples/integrations/mcp_trust/policy.yaml \
  --call examples/integrations/mcp_trust/tool_call.json \
  --json
```

`tool_call.json` must include the same `server.server_id` as the trust snapshot entry.
