# Kill Switch Contract

Gate kill switches provide local state-backed emergency stop coverage for
matching agents, identities, tools, targets, paths, workspaces, and
environments.

State schema:

- `schemas/v1/gate/kill_switch_state.schema.json`

CLI surfaces:

```bash
gait kill-switch add --state ./.gait-out/kill_switch_state.json --identity alice --tool-name tool.exec --reason "incident freeze" --actor secops --json
gait kill-switch list --state ./.gait-out/kill_switch_state.json --json
gait kill-switch disable --state ./.gait-out/kill_switch_state.json --entry-id <id> --json
gait kill-switch expire --state ./.gait-out/kill_switch_state.json --entry-id <id> --expires-at 2026-05-09T15:00:00Z --json
gait gate eval --policy <policy.yaml> --intent <intent.json> --kill-switch-state ./.gait-out/kill_switch_state.json --json
gait mcp proxy --policy <policy.yaml> --call <tool_call.json> --kill-switch-state ./.gait-out/kill_switch_state.json --json
```

Matching selectors:

- `agent_id`
- `identity`
- `tool_name`
- `target_kind` plus `target_value`
- `path_prefixes`
- `workspace_prefixes`
- `environment`

Reason-code contract:

- `kill_switch_active`
- `kill_switch_agent_id_active`
- `kill_switch_identity_active`
- `kill_switch_tool_name_active`
- `kill_switch_target_active`
- `kill_switch_path_active`
- `kill_switch_workspace_active`
- `kill_switch_environment_active`
- `kill_switch_state_unavailable`

Fail-closed behavior:

- when a kill-switch state file is configured and unavailable in `oss-prod`, or
  on configured high-risk paths, Gate blocks with
  `kill_switch_state_unavailable`
- expired or disabled entries do not block
- matching entries are journaled to `kill_switch_journal.jsonl` next to the
  state file

Notes:

- job-level emergency stop remains authoritative and backward-compatible
- broader kill-switch scopes are additive and schema-backed
