# ADR 0007: Generalized Kill Switch State

- Status: Accepted
- Date: 2026-05-09
- Related epic: `GSC2.1`

## Context

Job-level emergency stop already blocks MCP dispatches for stopped jobs, but
operators also need fast additive stop scopes for risky agents, tools, targets,
paths, workspaces, and environments.

## Decision

Add a schema-backed generalized kill-switch state file owned by Go core:

- additive selectors for `agent_id`, `identity`, `tool_name`, `target`,
  `path_prefixes`, `workspace_prefixes`, and `environment`
- shared matcher used by `gait gate eval`, `gait mcp proxy`, and `gait mcp serve`
- atomic state writes through `gait kill-switch add|list|disable|expire`
- fail-closed blocking with `kill_switch_state_unavailable` when configured
  state is unavailable in strict profiles/high-risk paths
- append-only kill-switch journal entries for blocked dispatch decisions

## Alternatives Considered

1. Keep job emergency stop only.
   - Rejected: insufficient scope for broader operational freezes.
2. Put stop logic in wrappers only.
   - Rejected: weakens the Go-authoritative execution boundary.

## Consequences

- Operators can stop covered execution paths without editing policy files.
- Job emergency stop remains backward-compatible.
- Future explain and proof surfaces can consume one shared kill-switch decision
  object.
