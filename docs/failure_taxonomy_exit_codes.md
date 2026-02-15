---
title: "Failure Taxonomy And Exit Codes"
description: "Operator-facing mapping of Gait exit codes, error categories, and deterministic JSON error envelope fields."
---

# Failure Taxonomy And Exit Codes

This page is the practical operator reference for non-zero outcomes.

Canonical constants live in:

- `cmd/gait/verify.go` (stable exit code constants)
- `cmd/gait/error_output.go` (error envelope + category mapping)
- `core/errors/errors.go` (error categories)
- `docs/adr/0001-error-taxonomy-and-exit-mapping.md` (taxonomy decision record)

## Stable Exit Codes

| Exit code | Meaning | Typical producer commands |
| --- | --- | --- |
| `0` | success | all commands |
| `1` | internal/runtime failure | IO, contention, transient/permanent network, uncategorized internal faults |
| `2` | verification failure | `gait verify`, `gait pack verify`, diff/trace verify mismatch paths |
| `3` | policy blocked | `gait gate eval`, `gait policy test` |
| `4` | approval required | `gait gate eval`, `gait policy test` |
| `5` | deterministic regression failure | `gait regress run`, `gait regress bootstrap` |
| `6` | invalid input/usage | malformed flags, missing required args, invalid schema input |
| `7` | missing dependency | dependency checks, doctor readiness failures |
| `8` | unsafe operation blocked | explicit unsafe replay guardrails |

## JSON Error Envelope

When `--json` is used and a command emits an error, outputs include these fields:

- `error`
- `error_code`
- `error_category`
- `retryable`
- `hint`
- `correlation_id` (when available)

This envelope is emitted by `writeJSONOutput`/`marshalOutputWithErrorEnvelope` in `cmd/gait/error_output.go`.

## Category To Exit Mapping

`exitCodeForError` maps categorized errors to exit behavior:

| Error category (`core/errors`) | Exit code |
| --- | --- |
| `invalid_input` | `6` |
| `verification_failed` | `2` |
| `policy_blocked` | `3` |
| `approval_required` | `4` |
| `dependency_missing` | `7` |
| `io_failure` | `1` |
| `state_contention` | `1` |
| `network_transient` | `1` |
| `network_permanent` | `1` |
| `internal_failure` | `1` |

## CI-Significant Codes

For gating workflows, treat these as explicit contract signals:

- `0`: pass
- `5`: deterministic regress failure (expected fail path for forced drift tests)
- `3`: policy block
- `4`: approval required
- `6`: invalid input/config

## Practical Triage

1. Read `error_code`, `error_category`, and `hint` from `--json` output first.
2. Use exit code only as routing signal; use reason/violation fields for diagnosis.
3. For policy outcomes (`3`, `4`), inspect:
   - `verdict`
   - `reason_codes`
   - `violations`
4. For verification outcomes (`2`), inspect mismatch lists and artifact digests.

## Quick Commands

```bash
# verification failure path -> exit 2
gait verify ./gait-out/nonexistent_or_tampered.zip --json

# policy block/approval paths -> exit 3 or 4
gait policy test examples/policy/endpoint/block_denied_endpoints.yaml examples/policy/endpoint/fixtures/intent_block.json --json
gait policy test examples/policy/endpoint/require_approval_destructive.yaml examples/policy/endpoint/fixtures/intent_destructive.json --json

# deterministic regress failure path -> exit 5
gait regress run --json
```
