# PLAN v2.6 Activation: First-Session Value and Adoption Speed

Date: 2026-02-14
Scope: Activation and UX polish only (no architectural rewrite)
Status: Implemented on `codex/new-branch` (pending merge)

## Goal

Reduce time-to-value from install to repeat usage by making the default experience self-guided, durable-job-first, and policy-aware.

## Why v2.6

v2.4 and v2.5 completed major capability depth (durable jobs, unified packs, context-proof evidence). The main remaining gap is activation speed:

- users can complete workflows, but first-session guidance is weak
- strongest differentiators are not on the default path
- docs and onboarding are comprehensive but heavy for first integration

## Success Metrics (Locked)

- `A1`: `gait demo` users see explicit next-step guidance in command output.
- `A2`: Median time for A1->A4 (demo, verify, regress init/run) reduced to <= 30 minutes in guided walkthrough trials.
- `A3`: First durable-job completion rate increases after `demo --durable` rollout.
- `A4`: First policy-block demo completion rate increases after guided policy path rollout.
- `A5`: Integration checklist completion starts improve by separating core and advanced tracks.

## Non-Goals

- No change to cryptographic contracts (JCS, SHA-256, Ed25519).
- No removal of existing commands or schema fields.
- No hosted control-plane work in this phase.
- No weakening of fail-closed behavior.

## Workstream 0: Scope + Instrumentation Baseline

### W0.1 Freeze v2.6 activation scope

Tasks:
- Create implementation tracker from this plan.
- Tag all v2.6 tasks as activation-only.

Acceptance:
- Explicit task board exists and maps each item in this plan to owner and milestone.

### W0.2 Define KPI capture approach

Tasks:
- Define how `demo -> verify -> regress` completion will be measured.
- Define rollout comparison window (pre/post v2.6).

Acceptance:
- KPI definition doc added and linked from this plan before implementation starts.

## Workstream 1: First-5-Minute CLI Guidance

### W1.1 Add next-step hints to `gait demo` and `gait verify`

Tasks:
- Emit clear "what to run next" guidance in human output.
- Keep `--json` stable and machine-readable.

Acceptance:
- New command tests cover text guidance and ensure JSON compatibility remains stable.

### W1.2 Add `gait demo --durable`

Tasks:
- Create guided durable lifecycle path: submit -> checkpoint -> simulated interruption -> resume -> verify pack.
- Keep flow fully offline-first.

Acceptance:
- Dedicated acceptance test verifies deterministic output and successful lifecycle completion.

### W1.3 Add `gait demo --policy`

Tasks:
- Include a deterministic high-risk block example in first-session demo.
- Surface matched rule and reason codes with user-readable explanation.

Acceptance:
- Acceptance test validates fail-closed block behavior and reason-code stability.

### W1.4 Clarify `signature_status=missing` messaging

Tasks:
- Improve human output explanation for unsigned dev/local artifacts.
- Clarify when this is expected vs a real issue.

Acceptance:
- Help/verify output tests cover messaging variants (dev/local vs signed artifacts).

### W1.5 Improve Python SDK binary-not-found error

Tasks:
- Convert opaque missing-binary failures into actionable remediation output.
- Include clear install/PATH guidance.

Acceptance:
- SDK tests assert error text contains actionable fixes.

## Workstream 2: Integration and Docs Simplification

### W2.1 Tier integration checklist

Tasks:
- Split checklist into:
  - Core first integration (minimum required set)
  - Advanced hardening/scaling controls
- Keep full checklist available for enterprise/security teams.

Acceptance:
- `docs/integration_checklist.md` clearly labels core vs advanced and preserves full coverage.

### W2.2 Add formal SDK docs

Tasks:
- Create `docs/sdk/` section covering Python SDK contract, subprocess model, error handling, and minimal examples.

Acceptance:
- SDK docs linked from `README.md` and `docs/README.md`.

### W2.3 Promote "observe -> enforce" rollout path

Tasks:
- Surface rollout pattern in CLI help/examples and docs.
- Provide short default path and explicit escalation path.

Acceptance:
- Policy docs and command help reference the same rollout sequence.

## Workstream 3: Guided Activation Flow

### W3.1 Add `gait tour`

Tasks:
- Create one guided command that walks A1->A4 in one session.
- Include durable and policy branch hints.

Acceptance:
- Tour command completes offline and emits deterministic summary output.

### W3.2 Add concise doctor mode

Tasks:
- Add `gait doctor --summary` for healthy environments.
- Preserve full detail mode.

Acceptance:
- Doctor tests verify summary/full mode outputs and contract stability.

### W3.3 Improve adoption-metrics discoverability

Tasks:
- Surface metrics opt-in hints in first-run UX and docs.
- Keep opt-in/privacy stance unchanged.

Acceptance:
- First-run output and docs include clear metrics opt-in instructions.

## Workstream 4: Messaging and Content Support

### W4.1 README hero demo asset review

Tasks:
- Evaluate whether current README GIF reflects durable-job-first and policy-aware value path.
- Review/update the existing recording script path (`scripts/record_runpack_hero_demo.sh`) or add a new script variant if required.
- Re-record and replace GIF only if review determines mismatch.

Acceptance:
- Decision documented (keep or replace).
- If replaced: script + generated asset workflow documented and reproducible.

### W4.2 Docs-site homepage alignment

Tasks:
- Ensure docs-site front page reflects v2.6 activation narrative consistently with README and contracts.

Acceptance:
- Copy alignment check completed across README, docs-site landing page, and docs map.

### W4.3 Content cadence plan

Tasks:
- Define recurring content themes for adoption (incident->regress, durable jobs, policy rollout, context proof).

Acceptance:
- Quarterly content calendar draft added to docs/launch or equivalent planning location.

## Workstream 5: Release Documentation Consolidation

### W5.1 Prepare `CHANGELOG.md` v1.2.0 entry

Tasks:
- Add v1.2.0 changelog section that consolidates shipped work from:
  - PLAN 2.4 (durable jobs + PackSpec v1 + runtime convergence)
  - PLAN 2.5 (context evidence + conformance + chaos/perf/release gating)
- Separate user-facing highlights from migration/compatibility notes.

Acceptance:
- `CHANGELOG.md` has clear v1.2.0 section with:
  - Added / Changed / Fixed headings
  - upgrade notes
  - contract-impact summary

Note:
- This plan item is documentation work and does not imply release tagging in this phase.

## Validation Gates (for v2.6 implementation phase)

Minimum required validation set per PR slice:

- `make lint`
- `make test`
- `make test-e2e`
- `go test ./internal/integration -count=1`
- `make test-v2-3-acceptance`
- `make test-v2-4-acceptance`
- `make test-v2-5-acceptance`
- `make test-context-conformance`
- `make test-context-chaos`
- `make test-ui-acceptance`
- `make test-ui-unit`
- `make test-ui-e2e-smoke`
- `make test-ui-perf`
- `make test-packspec-tck`
- `make test-runtime-slo`
- `make bench-check`
- `make codeql`
- `bash scripts/test_uat_local.sh`

## Phased Delivery Recommendation

- Week 1: W1.1-W1.5
- Weeks 2-4: W2.1-W2.3
- Month 2: W3.1-W3.3
- Month 3: W4.1-W4.3 + W5.1

## Definition of Done (v2.6 Activation)

- First-session guidance is explicit in CLI defaults.
- Durable jobs and policy block are both reachable from guided demo paths.
- Integration docs are tiered and SDK docs are formalized.
- README/docs-site/demo asset story is consistent with shipped behavior.
- `CHANGELOG.md` includes v1.2.0 consolidation of 2.4 and 2.5 work.
- Full validation matrix remains green with no regression in v2.3/v2.4/v2.5 guarantees.
