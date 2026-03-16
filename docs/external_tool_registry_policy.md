# External Tool Registry To Policy Workflow

Use this workflow when your organization already maintains an approved tool registry and you want Gait policy to enforce it at execution time.

For MCP server admission, use the same boundary principle with a local trust snapshot. External scanners or registries produce local files; `gait mcp verify`, `gait mcp proxy`, and `gait mcp serve` consume the local file and enforce policy. Gait does not become the scanner.

## Goal

Convert external allowlist data into deterministic `gait.gate.policy` YAML and validate it before rollout.

Gait does not own the tool registry. It consumes registry outputs as policy input.

## MCP Trust Snapshot Workflow

Use this when an external source such as Snyk or an internal registry produces trust data for MCP servers:

1. Export a local trust file from the external system.
2. Render a local `gait.mcp.trust_snapshot` JSON file.
3. Preflight with `gait mcp verify`.
4. Enforce the same trust policy through `gait mcp proxy` or `gait mcp serve`.

`gait mcp verify --json` reports that contract explicitly with `trust_model=local_snapshot` and `snapshot_path=<local file>`. The evaluator does not fetch hosted registry data at decision time.

Example policy contract:

```yaml
mcp_trust:
  enabled: true
  snapshot: ./examples/integrations/mcp_trust/trust_snapshot.json
  action: block
  required_risk_classes: [high, critical]
  min_score: 0.8
  max_age: 168h
  require_registry: true
```

```bash
python3 scripts/render_mcp_trust_snapshot.py \
  --input examples/integrations/mcp_trust/snyk_mcp_report.json \
  --output examples/integrations/mcp_trust/trust_snapshot.json

gait mcp verify \
  --policy examples/integrations/mcp_trust/policy.yaml \
  --server examples/integrations/mcp_trust/server_github.json \
  --json
```

Positioning rule:

- scanner or registry finds
- Gait enforces

## Supported Allowlist Input Shapes

`scripts/render_tool_allowlist_policy.py` accepts:

1. Object with tools list:
```json
{ "tools": ["tool.read", "tool.write"] }
```

2. Array of tool names:
```json
["tool.read", "tool.write"]
```

3. Array of objects:
```json
[{ "tool_name": "tool.read" }, { "tool_name": "tool.write" }]
```

## Recipe

1. Export allowlist JSON from your registry system.
2. Render policy YAML from the allowlist.
3. Validate with `gait policy test`.
4. Promote using your normal rollout stages (`simulate` -> `dry_run` -> `enforce`).

```bash
python3 scripts/render_tool_allowlist_policy.py \
  --input ./examples/policy/external_tool_allowlist.json \
  --output ./gait-out/policy_external_allowlist.yaml

gait policy test ./gait-out/policy_external_allowlist.yaml examples/policy/intents/intent_read.json --json
gait policy test ./gait-out/policy_external_allowlist.yaml examples/policy/intents/intent_delete.json --json
```

Equivalent Make target:

```bash
make tool-allowlist-policy \
  INPUT=examples/policy/external_tool_allowlist.json \
  OUTPUT=gait-out/policy_external_allowlist.yaml
```

## Example Source File

Use this local fixture as a template:

- `examples/policy/external_tool_allowlist.json`

## Why This Is First-Class

- Keeps registry ownership with your platform/security tooling.
- Keeps enforcement deterministic at the action boundary.
- Produces explicit policy artifacts that can be reviewed, versioned, and tested in CI.
