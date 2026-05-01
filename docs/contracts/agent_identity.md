# Agent Identity Contract

Agent identity is now a first-class policy input for high-risk authorization.

Intent context can include:

- `context.agent_id`
- `context.agent_identity.lifecycle_states`
- `context.agent_identity.owner`
- `context.agent_identity.manifest_digest`
- `context.agent_identity.publisher`
- `context.agent_identity.source`
- `context.agent_identity.issued_at`
- `context.agent_identity.approved_at`
- `context.agent_identity.expires_at`
- `context.agent_identity.revoked`

Policy rules can enforce:

- `require_declared_agent`
- `allowed_agent_ids`
- `denied_agent_ids`
- `required_agent_manifest_digest`
- `allowed_agent_manifest_publishers`
- `allowed_agent_manifest_sources`
- `required_agent_lifecycle_states`
- `require_agent_owner`
- `require_unexpired_agent`

Representative block reasons:

- `agent_unknown`
- `agent_revoked`
- `agent_expired`
- `agent_manifest_mismatch`
- `agent_owner_missing`
- `agent_lifecycle_state_invalid`

The signed gate trace carries the normalized agent identity evidence alongside
the rest of the execution context so offline proof verification can explain which
agent lifecycle inputs were used.
