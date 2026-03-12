# ADR 0002: Atomic Write Strategy

- Status: Accepted
- Date: 2026-02-06
- Related epic: `H2`

## Context

Critical runtime files are written by multiple flows. Partial writes can corrupt state and break deterministic behavior.

## Decision

Use a shared atomic write utility for critical files:

1. write to temp file in destination directory
2. flush and sync file contents
3. apply explicit file mode
4. atomically rename into final path

Durable job lifecycle mutations add a local pending-mutation marker beside `state.json` and `events.jsonl`:

1. write `pending_mutation.json` with the previous state, intended next state, and event payload
2. atomically write `state.json`
3. append the event to `events.jsonl`
4. remove the marker on success

Recovery semantics are deterministic:

- if the marker exists and the event is missing, restore the previous state
- if the marker exists and the event is already durable, materialize the intended next state
- least-privilege file modes remain `0600` for job state, event logs, and recovery markers

## Alternatives Considered

1. Keep direct `os.WriteFile` usage in all call sites.
   - Rejected: partial-write corruption risk.
2. Add per-package custom atomic helpers.
   - Rejected: duplicated logic and inconsistent semantics.

## Consequences

- Improved crash consistency for state and artifact writes.
- Requires migration of existing critical write paths and targeted failure tests.
