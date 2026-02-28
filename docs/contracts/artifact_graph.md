# Artifact Graph Contract (Normative)

Status: normative for OSS v1.x producers and consumers.

This contract defines how Gait artifacts compose into a verifiable graph over time.

## Scope

Applies to:

- Runpack artifacts (`gait.runpack.*`)
- Context evidence artifacts (`gait.context.envelope`, `gait.context.reference_record`, `gait.context.budget_report`)
- Session artifacts (`gait.runpack.session_journal`, `gait.runpack.session_checkpoint`, `gait.runpack.session_chain`)
- Gate traces (`gait.gate.trace`)
- Delegation artifacts (`gait.gate.delegation_token`, `gait.gate.delegation_audit_record`)
- Regress results (`gait.regress.result`)
- Evidence packs (`gait.guard.pack`)

## Contract Rules

- Artifacts MUST be schema-versioned and self-describing (`schema_id`, `schema_version`).
- Digests MUST be computed from canonical JSON (JCS/RFC 8785) for any signed or hashed content.
- Cross-artifact references SHOULD use immutable identifiers (digest, run_id, trace_id) and MUST NOT rely on mutable display names.
- Producers MUST preserve deterministic serialization for stable verification and diff behavior.
- Consumers MUST treat unknown additive fields as non-breaking within major `v1.x`.
- Producers MAY include a standardized optional `relationship` envelope to emit graph-ready topology with:
  - `parent_ref`
  - `entity_refs[]`
  - `policy_ref`
  - `agent_chain[]`
  - `edges[]`
- Relationship envelope values MUST be deterministic when present:
  - lowercase digest identifiers
  - deduplicated arrays
  - stable sort order for refs and edges
  - UTC/RFC3339 for timestamps already carried in parent artifacts

## Graph Integrity Expectations

- A `TraceRecord` MUST bind verdict context (`intent_digest`, `policy_digest`) for one tool decision.
- A `TraceRecord` SHOULD carry `context_set_digest` when context evidence is present in the evaluated intent path.
- A `Runpack` MUST include the deterministic run timeline and manifest digests for replay and verification.
- A context-enabled `Runpack` SHOULD preserve `refs.context_set_digest` continuity with any bundled `context_envelope.json`.
- A `SessionCheckpoint` MUST bind checkpoint index/range to runpack `manifest_digest` and `checkpoint_digest`.
- A `SessionChain` MUST preserve `prev_checkpoint_digest` continuity across checkpoint sequence.
- A `RegressResult` SHOULD reference fixture/run identity so failures can map back to captured artifacts.
- Evidence bundles SHOULD include pointers back to the exact runpack/trace/regress artifacts they summarize.
- Delegation audits SHOULD reference the trace and delegation token IDs used for allow/block outcomes.
- Trace and audit producers SHOULD include `relationship` envelopes when enough local context exists to bind:
  - actor -> tool (`calls`)
  - tool -> policy (`governed_by`)
  - delegator -> delegate (`delegates_to`)
- Consumer projections SHOULD preserve intent/receipt digest continuity (`intent_digest`, `policy_digest`, `refs.context_set_digest`, `refs.receipts[*].{query_digest,content_digest}`) when deriving audit views.

## Compatibility Model

- Within `v1.x`, artifact schemas are append-only for required behavior.
- Breaking field removals/renames require a major version increment.
- New optional fields are allowed when existing readers can ignore them safely.

## Why This Exists

The artifact graph is the durable system of record for:

- incident reconstruction
- policy drift comparisons
- regression baselines
- audit evidence continuity

The contract above keeps that graph verifiable and stable across tool versions.
