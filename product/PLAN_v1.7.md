# PLAN v1.7: OSS Execution Wedge Expansion (Endpoint + Skills + SLO)

Date: 2026-02-09  
Source of truth: `product/PRD.md`, `product/ROADMAP.md`, `product/PLAN_v1.md`, `product/PLAN_v1.6.md`, current codebase (`main`)  
Scope: OSS execution path only. This plan extends v1.6 with endpoint-governance depth, skill supply-chain controls, and measurable runtime SLO proof.

This plan is execution-ready and gap-driven. Every story includes concrete tasks, repo paths, and acceptance criteria.

---

## v1.7 Objective

Complete the OSS wedge for production local-agent adoption by finishing three prerequisites before enterprise control plane build:

- endpoint action taxonomy + deterministic policy enforcement
- skill provenance and signed/pinned execution safety
- hard runtime SLO proof for gate overhead and fail-closed behavior

Result target:
- Gait remains the default local execution boundary and evidence substrate for agent actions.

---

## Locked Product Decisions (OSS v1.7)

- Keep OSS focused on execution-plane controls and artifacts. No multi-tenant control plane in v1.7.
- Keep runtime posture fail-closed by default; keep `--simulate` as non-enforcing rollout mode.
- Keep policy engine in Go core; do not introduce OPA/Rego dependency in v1.7.
- Keep category boundary stable: Gait is control + proof for agent actions, not an agent orchestrator.
- Keep artifact and schema continuity with v1.x compatibility guarantees.

---

## Current Baseline (Observed in Codebase)

Already implemented:
- Primitive contract and compatibility guard (`docs/contracts/primitive_contract.md`, `scripts/test_contracts.sh`).
- Canonical wrapper + sidecar + MCP proxy boundary (`sdk/python/gait/adapter.py`, `examples/sidecar/gate_sidecar.py`, `cmd/gait/mcp.go`).
- Deterministic incident loop and bootstrap path (`gait regress bootstrap`).
- Local signal engine and schemas (`core/scout/signal.go`, `schemas/v1/scout/*`).
- Wedge integrity CI gate for v1.6 (`scripts/test_v1_6_acceptance.sh`, `.github/workflows/ci.yml` job `v1-6-acceptance`).
- Coverage and perf budget checks in local/CI validation paths.

Gaps to close in v1.7:
- Endpoint primitives are not yet first-class policy taxonomy with explicit constraints (path/domain/destructive classes).
- Skill execution provenance is not yet modeled as supply-chain input with signed/pinned publisher controls.
- Runtime SLO proof is not yet formalized as CI-enforced latency/error-budget contract.
- OSS docs still need an explicit “local-agent safety lane” that maps endpoint + skill controls to operator workflows.
- Required local-agent adapters for `OpenClaw` and `AutoGPT` are not yet implemented with parity checks.

---

## v1.7 Exit Criteria

v1.7 is complete only when all are true:

- Endpoint actions are represented by a stable taxonomy and enforced by Gate deterministically.
- Skill-triggered actions carry provenance and can be verified against signed/pinned allowlist policy.
- Fail-closed behavior and gate overhead are backed by explicit SLO budgets and CI checks.
- A team can adopt endpoint-safe policy and skill provenance checks in one PR using provided templates.
- Required adapter paths for `OpenClaw` and `AutoGPT` pass the same conformance suite as existing adapters.
- OSS remains offline-first, default-safe, vendor-neutral, and independently verifiable.

---

## Epic V17.0: Endpoint Action Model

Objective: make endpoint primitives first-class governable actions.

### Story V17.0.1: Taxonomy spec and schema updates

Tasks:
- Add normative endpoint taxonomy doc with stable class IDs:
  - `fs.read`, `fs.write`, `fs.delete`
  - `proc.exec`
  - `net.http`, `net.dns`
  - optional `ui.*` classes as reserved for additive rollout
- Extend intent and policy schemas/types with endpoint classification metadata and constraints.

Repo paths:
- `docs/contracts/endpoint_action_model.md`
- `schemas/v1/gate/`
- `core/schema/v1/gate/`

Acceptance criteria:
- Endpoint classes are versioned, documented, and schema-validated.
- Existing intent fixtures remain compatible or migrate deterministically.

### Story V17.0.2: Endpoint normalization in Gate

Tasks:
- Add deterministic normalization helpers mapping tool calls to endpoint classes.
- Ensure missing/ambiguous endpoint class in high-risk mode fails closed with explicit reason code.

Repo paths:
- `core/gate/intent.go`
- `core/gate/policy.go`
- `cmd/gait/gate.go`

Acceptance criteria:
- Same input yields same endpoint class and digest across OS matrix.
- Fail-closed reason codes are stable and tested.

### Story V17.0.3: Endpoint constraint enforcement

Tasks:
- Add policy constraints for:
  - path allowlist/denylist patterns
  - domain allowlist/egress classes
  - destructive operation classification
- Add deterministic violation codes and explain text.

Repo paths:
- `core/gate/policy.go`
- `schemas/v1/gate/`
- `examples/policy/endpoint/`

Acceptance criteria:
- Violating endpoint constraints blocks or requires approval predictably.
- Fixture tests cover allow/block/require_approval for each endpoint class.

### Story V17.0.4: Endpoint fixtures and migration tests

Tasks:
- Add valid/invalid endpoint intent fixtures.
- Add migration tests for pre-v1.7 intents to preserve compatibility.

Repo paths:
- `core/schema/testdata/`
- `core/schema/validate/validate_test.go`
- `cmd/gait/main_test.go`

Acceptance criteria:
- Endpoint fixtures validate in CI.
- Migration path is deterministic and documented.

---

## Epic V17.1: Skill Provenance and Supply Chain Controls

Objective: treat skill-triggered execution as governable supply-chain input.

### Story V17.1.1: Skill provenance model

Tasks:
- Define skill provenance fields attached to intents/traces:
  - skill name, version, source, publisher, digest, signature key ID
- Add schema and type support for provenance references without raw secret leakage.

Repo paths:
- `schemas/v1/gate/`
- `core/schema/v1/gate/`
- `docs/contracts/skill_provenance.md`

Acceptance criteria:
- Skill provenance fields are optional but validated when present.
- Trace outputs preserve provenance deterministically.

### Story V17.1.2: Signed and pinned skill verification

Tasks:
- Extend registry verification flow for skill bundles/manifests:
  - signature check
  - digest pinning
  - publisher allowlist checks
- Emit deterministic verification report artifact.

Repo paths:
- `core/registry/`
- `cmd/gait/registry.go`
- `schemas/v1/registry/`
- `examples/skills/`

Acceptance criteria:
- Unsigned/unpinned/disallowed skill bundles fail with stable error codes.
- Verification report is machine-readable and deterministic.

### Story V17.1.3: Policy hooks for skill trust

Tasks:
- Add policy conditions for trusted skill publisher/source classes.
- Support rule outcomes: allow, block, require_approval.

Repo paths:
- `core/gate/policy.go`
- `examples/policy/skills/`
- `docs/policy_rollout.md`

Acceptance criteria:
- Skill trust policy behavior is fixture-tested and CI-enforced.
- Approval path works with skill provenance context.

### Story V17.1.4: Skill safety acceptance suite

Tasks:
- Add focused test suite for skill provenance and trust path.

Repo paths:
- `scripts/test_skill_supply_chain.sh`
- `.github/workflows/ci.yml`
- `Makefile`

Acceptance criteria:
- Skill supply-chain regressions fail PR checks.

---

## Epic V17.2: Runtime SLO Proof and Fail-Closed Guarantees

Objective: convert safety and overhead claims into measured contracts.

### Story V17.2.1: Runtime SLO contract

Tasks:
- Define v1.7 runtime SLOs for gate path:
  - p50/p95/p99 latency budgets by endpoint class
  - error-budget envelope for evaluation failures
  - fail-closed behavior expectations by profile
- Publish operational SLO doc and runbook.

Repo paths:
- `docs/slo/runtime_slo.md`
- `docs/approval_runbook.md`
- `README.md`

Acceptance criteria:
- SLO thresholds are explicit and tied to measurable commands.

### Story V17.2.2: Tail-latency and reliability harness

Tasks:
- Add repeatable benchmarks/load checks for gate eval path and common endpoint classes.
- Export report artifacts with percentile breakdowns.

Repo paths:
- `perf/`
- `scripts/check_command_budgets.py`
- `scripts/`

Acceptance criteria:
- Harness runs offline and deterministically in CI.
- Budget failures produce actionable diagnostics.

### Story V17.2.3: Fail-closed verification matrix

Tasks:
- Add matrix tests for fail-closed cases:
  - invalid intent
  - missing policy signals for high-risk profile
  - skill provenance verification failure
  - broker/evidence failure for protected profiles

Repo paths:
- `cmd/gait/main_test.go`
- `core/gate/`
- `internal/e2e/`

Acceptance criteria:
- Matrix is explicit and stable across OS matrix.
- Exit codes and reason codes remain deterministic.

---

## Epic V17.3: Local-Agent Adoption Kit (OSS Distribution)

Objective: make endpoint-safe adoption easy without hosted dependencies.

### Story V17.3.1: Local-agent integration kit

Tasks:
- Add one canonical local-agent integration template:
  - wrapper mode
  - sidecar mode
  - MCP proxy mode
- Include endpoint and skill provenance examples.

Repo paths:
- `examples/integrations/`
- `docs/integration_checklist.md`
- `README.md`

Acceptance criteria:
- Integration template works end-to-end offline with fixtures.

### Story V17.3.2: Copy-paste CI kit for endpoint + skill checks

Tasks:
- Extend CI template to include:
  - regress checks
  - endpoint policy fixture tests
  - skill verification checks

Repo paths:
- `.github/workflows/adoption-regress-template.yml`
- `docs/ci_regress_kit.md`

Acceptance criteria:
- New repo can enable full lane with one template and minimal edits.

### Story V17.3.3: OpenClaw adapter (required)

Tasks:
- Add one canonical `OpenClaw` integration example with parity contract behavior.
- Normalize OpenClaw tool-call payloads into `IntentRequest` and route through `gait gate eval`.
- Include endpoint-class mapping and skill provenance context in adapter output path.
- Enforce fail-closed behavior for all non-`allow` verdicts and evaluation errors.

Repo paths:
- `examples/integrations/openclaw/`
- `docs/integration_checklist.md`
- `scripts/test_adoption_smoke.sh`

Acceptance criteria:
- OpenClaw adapter supports deterministic allow/block scenarios with expected artifact outputs.
- OpenClaw adapter never executes side effects on non-`allow` results.

### Story V17.3.4: AutoGPT adapter (required)

Tasks:
- Add one canonical `AutoGPT` integration example with parity contract behavior.
- Normalize AutoGPT tool-call payloads into `IntentRequest` and route through `gait gate eval`.
- Include endpoint-class mapping and skill provenance context in adapter output path.
- Enforce fail-closed behavior for all non-`allow` verdicts and evaluation errors.

Repo paths:
- `examples/integrations/autogpt/`
- `docs/integration_checklist.md`
- `scripts/test_adoption_smoke.sh`

Acceptance criteria:
- AutoGPT adapter supports deterministic allow/block scenarios with expected artifact outputs.
- AutoGPT adapter never executes side effects on non-`allow` results.

### Story V17.3.5: Adapter parity conformance harness

Tasks:
- Add shared adapter conformance checks across:
  - `openai_agents`
  - `langchain`
  - `autogen`
  - `openclaw`
  - `autogpt`
- Validate common contract outputs:
  - verdict parity
  - `executed` semantics
  - deterministic trace/evidence paths
  - stable fail-closed handling

Repo paths:
- `scripts/test_adoption_smoke.sh` or `scripts/test_adapter_parity.sh`
- `.github/workflows/ci.yml`
- `Makefile`

Acceptance criteria:
- Any adapter contract drift fails local and CI adoption suites.

---

## Epic V17.4: OSS Wedge Integrity Gate (v1.7)

Objective: prevent silent drift from endpoint/skill/SLO commitments.

### Story V17.4.1: v1.7 acceptance script

Tasks:
- Add `scripts/test_v1_7_acceptance.sh` validating:
  - endpoint taxonomy policy path
  - skill provenance verification path
  - fail-closed matrix checks
  - local signal + receipt path continuity
  - SLO budget check gate

Repo paths:
- `scripts/`
- `Makefile`

Acceptance criteria:
- Script is deterministic and green locally before tag.

### Story V17.4.2: CI gate for v1.7

Tasks:
- Add dedicated CI job `v1-7-acceptance`.

Repo paths:
- `.github/workflows/ci.yml`

Acceptance criteria:
- PRs fail when v1.7 wedge guarantees regress.

---

## Measurement and Success Metrics (v1.7)

Activation metrics:
- `A1`: endpoint policy fixtures adopted in active repos
- `A2`: skill provenance verification enabled in active repos
- `A3`: local-agent integration template usage
- `A4`: OpenClaw and AutoGPT adapter parity suite pass rate

Operational metrics:
- `O1`: p95/p99 gate latency within v1.7 SLO budgets
- `O2`: percent of high-risk endpoint actions evaluated through Gate
- `O3`: percent of skill-triggered actions with valid provenance and trust checks

Wedge metrics:
- `W1`: incident-to-regression conversion rate remains stable/improves after endpoint controls
- `W2`: duplicate incident collapse rate in local signal reports
- `W3`: fail-closed policy decision consistency across OS matrix

Release gate for v1.7:
- `A1` to `A4` workflows are reproducible from fresh checkout.
- `O1` and `O3` are enforceable in CI.

---

## Explicit Non-Goals (v1.7)

- multi-tenant control-plane services in OSS runtime path
- hosted dashboard or fleet management platform scope
- agent scheduling/orchestration features
- OPA/Rego policy engine migration without concrete design-partner requirement
- fail-open defaults for high-risk execution paths

---

## Definition of Done (v1.7)

- Endpoint action taxonomy is explicit, stable, and policy-enforced.
- Skill provenance is verifiable (signed/pinned/allowlisted) and auditable.
- Runtime SLO and fail-closed guarantees are measured and CI-enforced.
- OSS adoption path remains one-wrapper/one-sidecar/one-CI-lane and offline-first.
- OpenClaw and AutoGPT are shipped with adapter parity guarantees.
- ENT v2 can consume v1.7 artifacts without changing OSS execution semantics.
