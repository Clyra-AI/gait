# ADR 0005: Freeze Window Policy Evaluation

- Status: Accepted
- Date: 2026-05-08
- Related epic: `GSC1.2`

## Context

Operators need deterministic policy answers for production freeze periods such
as release freezes, incident freezes, and quarter-end change holds.

## Decision

Add a first-class `freeze_window` rule block inside Gate policy evaluation:

- policy-owned IANA timezone
- local wall-clock start/end windows in policy
- additive `block` or `require_approval` effect
- optional environment and risk-class narrowing
- explicit `--evaluation-time <rfc3339>` on `gait gate eval` for replay and
  deterministic fixtures
- signed trace and JSON output carry the selected freeze-window decision

## Alternatives Considered

1. Wrapper-local freeze checks.
   - Rejected: breaks the Go-authoritative policy boundary.
2. UTC-only window config.
   - Rejected: harder for operators to author and review around local change
     windows and DST.

## Consequences

- Production freeze behavior is deterministic and replayable in tests.
- Invalid timezone or window configuration fails closed instead of silently
  skipping the freeze.
- Traces and follow-on explain/proof surfaces can consume one shared decision
  object.
