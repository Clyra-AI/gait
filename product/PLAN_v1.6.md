# PLAN v1.6: OSS Wedge Execution Plan (Agent Control Plane)

Date: 2026-02-07
Source of truth: `product/PRD.md`, `product/ROADMAP.md`, `product/PLAN_v1.md`, `product/PLAN_ADOPTION.md`, current codebase (`main`)
Scope: OSS execution path only. This plan strengthens the adoption wedge and avoids enterprise control-plane scope.

This plan is execution-ready and gap-driven. Every story includes concrete tasks, repo paths, and acceptance criteria.

---

## v1.6 Objective

Win the OSS wedge by making Gait the default execution control and evidence path for production agent actions.

Wedge definition:
- standard artifact contract (`runpack` and `trace`)
- deterministic incident to regression loop
- enforceable tool-boundary gate
- low-friction integration in one wrapper path, one sidecar path, and one CI path

---

## Locked Product Decisions (OSS v1.6)

- Keep OSS focused on the execution path. No hosted dashboards in v1.6.
- Keep category framing simple: `Agent Control Plane` (explainer: runtime control and proof for agent actions).
- Keep the non-negotiable primitives stable:
  - `IntentRequest`
  - `GateResult`
  - `TraceRecord`
  - `Runpack`
- Keep default privacy posture: reference receipts by default, raw capture explicit only.
- Keep vendor neutrality: do not hard-couple to a single model provider or framework.

---

## Current Baseline (Observed in Codebase)

Already implemented:
- Stable command surface for runpack, regress, gate, trace, doctor (`cmd/gait/*`).
- Stable schema types for gate and runpack primitives (`core/schema/v1/gate/types.go`, `core/schema/v1/runpack/types.go`).
- Canonical thin Python wrapper path (`sdk/python/gait/adapter.py`, `sdk/python/gait/client.py`).
- CI path for lint/test/e2e/acceptance and policy/hardening suites (`.github/workflows/ci.yml`).
- Adoption assets and docs ladder pieces (`docs/integration_checklist.md`, `docs/policy_rollout.md`, `docs/approval_runbook.md`, `docs/ci_regress_kit.md`).
- Ticket footer semantics in demo output (`cmd/gait/demo.go`).

Gaps to close in v1.6:
- No canonical sidecar mode for non-Python stacks.
- Primitive contract is implemented but not documented as one explicit normative API contract.
- No local signal engine that deduplicates incidents, attributes drift causes, and ranks by blast radius.
- No single CI action/workflow artifact positioned as the default "one CI path" integration.
- Docs are improved but still need a strict one-ladder enforcement and anti-fragmentation checks.

---

## v1.6 Exit Criteria

v1.6 is complete only when all are true:
- A new team can integrate Gait with one wrapper OR one sidecar in under 30 minutes.
- A new team can enable one deterministic CI regression check in one PR.
- Local signal report can summarize top incident families and top drift causes from local artifacts.
- The four primitives are documented in one normative contract doc and enforced by compatibility checks.
- OSS remains fully useful offline without enterprise services.

---

## Epic V16.0: Primitive Contract Lock

Objective: make primitive contracts explicit, testable, and hard to accidentally break.

### Story V16.0.1: Publish normative primitive contract doc

Tasks:
- Add `docs/contracts/primitive_contract.md` with strict definitions for:
  - `IntentRequest`
  - `GateResult`
  - `TraceRecord`
  - `Runpack` (manifest and required files)
- Include required fields, compatibility rules, and producer/consumer obligations.

Acceptance criteria:
- README links this contract as the canonical integration contract.
- Contract language is normative and unambiguous.

### Story V16.0.2: Add contract compatibility guard checks

Tasks:
- Extend compatibility checks to fail when required primitive fields are removed or semantics change without version bump.
- Wire checks to existing contract job in CI.

Repo paths:
- `scripts/`
- `.github/workflows/ci.yml`
- `Makefile`

Acceptance criteria:
- Contract regression fails CI deterministically.

### Story V16.0.3: Add consumer fixtures for primitives

Tasks:
- Add golden fixtures representing minimal valid and invalid contracts.
- Validate both Go and Python consumer parsing against fixtures.

Repo paths:
- `core/schema/testdata/`
- `sdk/python/tests/`

Acceptance criteria:
- Primitive fixtures are validated in test runs and versioned.

---

## Epic V16.1: Integration Friction to Near Zero

Objective: enforce one wrapper path, one sidecar path, one CI path.

### Story V16.1.1: Canonical wrapper path hardening

Tasks:
- Keep one official wrapper pattern in Python and remove ambiguity from docs.
- Add explicit fail-closed examples for `block`, `require_approval`, and evaluation failure.

Repo paths:
- `sdk/python/gait/adapter.py`
- `sdk/python/tests/`
- `docs/integration_checklist.md`

Acceptance criteria:
- Wrapper conformance tests enforce non-bypass execution behavior.

### Story V16.1.2: Introduce canonical sidecar mode

Tasks:
- Add sidecar contract and starter script:
  - read normalized `IntentRequest` from stdin/file
  - execute `gait gate eval --json`
  - return `GateResult` plus trace path
- Add sidecar integration guide for non-Python environments.

Repo paths:
- `examples/sidecar/`
- `scripts/`
- `docs/integration_checklist.md`

Acceptance criteria:
- Sidecar path is runnable offline with provided fixtures.

### Story V16.1.3: Single CI path kit

Tasks:
- Publish one default CI template as the recommended path:
  - run `gait regress run --json --junit ...`
  - upload artifact outputs
  - fail on stable exit codes
- Deprecate duplicate CI snippets in docs.

Repo paths:
- `.github/workflows/adoption-regress-template.yml`
- `docs/ci_regress_kit.md`
- `README.md`

Acceptance criteria:
- "One CI path" is explicit in README and docs.

---

## Epic V16.2: Receipt Loop and Proof UX

Objective: strengthen the viral receipt workflow as the default collaboration unit.

### Story V16.2.1: Ticket footer consistency checks

Tasks:
- Add tests that lock `ticket_footer` format and required fields.
- Add docs examples for incident, PR, and postmortem usage.

Repo paths:
- `cmd/gait/demo.go`
- `cmd/gait/main_test.go`
- `docs/evidence_templates.md`

Acceptance criteria:
- Footer format is deterministic and schema-like across releases.

### Story V16.2.2: Add receipt extraction helper

Tasks:
- Add helper command or script to extract and print receipt from runpack metadata.
- Keep output one-line and ticket-safe.

Repo paths:
- `cmd/gait/` or `scripts/`
- `README.md`

Acceptance criteria:
- Teams can regenerate receipt from artifact without rerunning original scenario.

---

## Epic V16.3: Incident to Regression Default Path

Objective: ensure incident-to-regression is always the primary operational loop.

### Story V16.3.1: One-command bootstrap from run_id

Tasks:
- Add convenience path that runs:
  - fixture init
  - regress execution
  - optional junit output
- Keep existing commands unchanged; this is additive convenience.

Repo paths:
- `cmd/gait/regress.go`
- `scripts/`
- `README.md`

Acceptance criteria:
- New users can complete incident to regression flow with one documented command path.

### Story V16.3.2: Regression DX summaries

Tasks:
- Improve regress failure summary to include:
  - top failure reason
  - minimal next command
  - artifact pointers

Repo paths:
- `core/regress/`
- `cmd/gait/regress.go`

Acceptance criteria:
- Regress failures are actionable in one read without opening large JSON first.

---

## Epic V16.4: Local Signal Engine v0 (Offline)

Objective: reduce noise by collapsing runs into actionable local signals without hosted dependencies.

### Story V16.4.1: Run fingerprint schema and computation

Tasks:
- Add schema/type for run fingerprints derived from deterministic fields:
  - action sequence/tool classes
  - target systems
  - reason code vectors
  - reference receipts digests
- Compute fingerprint from runpack + traces.

Repo paths:
- `schemas/v1/scout/`
- `core/schema/v1/scout/`
- `core/scout/`

Acceptance criteria:
- Same incident family yields same fingerprint deterministically.

### Story V16.4.2: Incident family clustering report

Tasks:
- Add local report command (scout namespace) for clustering similar failures.
- Output top families with counts and canonical representative run.

Repo paths:
- `cmd/gait/scout.go`
- `core/scout/`

Acceptance criteria:
- Command outputs stable JSON and concise summary without network access.

### Story V16.4.3: Drift attribution and ranking

Tasks:
- Attribute drift to deterministic driver categories:
  - policy change
  - tool result shape change
  - reference set change
  - configuration change
- Add deterministic severity ranking based on:
  - tool privilege class
  - target sensitivity
  - approval/policy posture

Repo paths:
- `core/scout/`
- `schemas/v1/scout/`

Acceptance criteria:
- Output includes `top_issues` sorted by deterministic risk score.

### Story V16.4.4: Next-best-fix suggestions

Tasks:
- Emit bounded suggestions:
  - policy fixture to add
  - regress fixture to add
  - likely scope of change
- Keep explainable rule-based logic (no opaque ML dependency).

Acceptance criteria:
- Suggestions are deterministic for same inputs.

---

## Epic V16.5: Vendor-Neutral Distribution and Positioning

Objective: protect neutrality and avoid framework lock-in.

### Story V16.5.1: Adapter neutrality policy

Tasks:
- Add contribution policy requiring adapter parity principles and no privileged first-party bypasses.
- Define acceptance bar for new adapter examples.

Repo paths:
- `CONTRIBUTING.md`
- `examples/integrations/`

Acceptance criteria:
- Adapter additions follow one contract, not per-vendor divergent behavior.

### Story V16.5.2: Category and messaging guardrails

Tasks:
- Add concise external messaging section:
  - what Gait is
  - what Gait is not
  - why artifact-first and execution-boundary-first
- Keep claims aligned with shipped behavior.

Repo paths:
- `README.md`
- `docs/`

Acceptance criteria:
- Messaging avoids generic "AI governance dashboard" positioning.

---

## Epic V16.6: OSS Quality Gate for Wedge Integrity

Objective: prevent drift away from the wedge.

### Story V16.6.1: Add v1.6 acceptance script

Tasks:
- Add `scripts/test_v1_6_acceptance.sh` validating:
  - one-path onboarding
  - ticket footer contract
  - incident to regression path
  - wrapper or sidecar fail-closed behavior
  - local signal report generation

Acceptance criteria:
- Script is green in CI before release tagging.

### Story V16.6.2: CI job for v1.6 wedge checks

Tasks:
- Add dedicated CI job:
  - `v1-6-acceptance`
- Keep runtime short enough for PR usage.

Repo paths:
- `.github/workflows/ci.yml`

Acceptance criteria:
- Wedge regressions fail in PR checks.

---

## Measurement and Success Metrics (v1.6)

Activation metrics:
- `A1`: `gait demo` success
- `A2`: `gait verify` success
- `A3`: `gait regress init` success
- `A4`: `gait regress run` success

Wedge metrics:
- `W1`: percent of run incidents with valid ticket footer attached
- `W2`: percent of incidents converted to regress fixtures within 24 hours
- `W3`: percent of high-risk tool paths routed through gate wrapper/sidecar
- `W4`: percent of duplicate incident runs collapsed into one signal family

Release gate for v1.6:
- `A1` to `A4` path works from fresh checkout.
- `W2` and `W3` trend up release over release.

---

## Definition of Done (v1.6)

- Primitive contract is explicit, documented, and enforced in CI.
- Integration path is unambiguous: one wrapper, one sidecar, one CI kit.
- Ticket footer and incident to regression loop are deterministic and easy to execute.
- Local signal report reduces incident/drift noise without hosted dependencies.
- OSS remains offline-first, default-safe, and vendor-neutral.
