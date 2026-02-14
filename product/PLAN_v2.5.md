# PLAN v2.5: Context Evidence and Deterministic Proofrails (Zero-Ambiguity Execution Plan)

Date: 2026-02-14
Source of truth: `/Users/davidahmann/Projects/gait/product/PLAN_v1.md`, `/Users/davidahmann/Projects/gait/product/PLAN_2.4.md`, `/Users/davidahmann/Projects/gait/docs/uat_functional_plan.md`, `/Users/davidahmann/Projects/gait/docs/test_cadence.md`, memo "OSS Autonomy Harness wedge (Pack Runtime)"
Scope: v2.5 only (standalone Gait context-evidence hardening to increase trust, replay safety, and CI signal quality without adding external runtime dependencies)

This plan is written to be executed top-to-bottom with minimal interpretation. Each story includes concrete repository paths, commands, and acceptance criteria.

---

## Global Decisions (Locked for v2.5)

- v2.5 is a standalone Gait release. No runtime dependency, shared library dependency, or remote control-plane dependency on `/Users/davidahmann/Projects/fabra_oss`.
- External projects are reference-only for design patterns. Gait remains authoritative for:
  - RFC 8785 canonicalization
  - hash/sign/verify
  - policy evaluation and fail-closed enforcement
  - pack and regress contracts
- Context capability is evidence-first, not feature-serving:
  - we prove what context was used
  - we prove it passed deterministic policy and integrity checks
  - we do not add probabilistic context classifiers to enforcement paths
- Offline-first is preserved:
  - verify/diff/regress must run offline
  - no new mandatory network listener
- Default privacy posture is preserved:
  - metadata and digests by default
  - raw context material requires explicit unsafe flags
- Schema compatibility remains additive within major `1.x`.
- Exit codes remain stable API surface; any additions require explicit contract tests.
- Coverage and quality bars remain:
  - Go line coverage >= 85% for enforced aggregate packages
  - Python SDK coverage >= 85%
  - Go package floor >= 75%

---

## Why v2.5 (Product Fit)

v2.4 completed durable jobs + unified pack runtime. v2.5 adds a missing trust multiplier:

- deterministic proof for LLM context inputs at the same quality level as tool-call traces
- fail-closed policy hooks when context evidence is missing for high-risk actions
- lower-noise CI regressions by separating semantic context drift from runtime-only variance

This directly increases wedge value for platform and security teams without changing the v1/v2 product shape.

---

## Gap Closure Matrix (v2.5)

| Gap ID | Gap | Current State | v2.5 Closure |
|---|---|---|---|
| C-01 | Deterministic context evidence bundle | Runpack has `refs.receipts` but no first-class context envelope contract | Add ContextSpec v1 schemas + deterministic `context_envelope.json` artifact in pack/run paths |
| C-02 | Fail-closed context evidence requirement | Gate policies can enforce tool/target constraints but no explicit context evidence requirement knob | Add policy + evaluator support for required context evidence in high-risk classes |
| C-03 | Context-set binding in trace chain | Gate trace does not bind a context set digest | Add `context_set_digest` and context evidence summary to signed trace records |
| C-04 | Portable context drift signal in pack diff | Diff exists but not context-semantic classification | Add deterministic context drift classification (`semantic` vs `runtime`) in pack diff JSON |
| C-05 | Context evidence required mode for capture | Capture paths accept refs but no explicit evidence mode contract | Add `best_effort|required` evidence mode with stable reason codes and fail behavior |
| C-06 | CI regression loop for context drift | Regress focuses on runpack behavior; no context-proof grader | Add context conformance graders and bootstrap defaults |
| C-07 | End-to-end test matrix coverage for new surface | Existing matrix is broad but no v2.5 context lanes | Add v2.5 acceptance, integration, chaos, perf, and release gates across full matrix |

---

## Reference Borrow Map (Design-Only, No Dependency)

These are pattern references only. No direct import or runtime coupling.

### Approved pattern references

- Evidence-required capture mode pattern:
  - `/Users/davidahmann/Projects/fabra_oss/src/fabra/context.py`
- Privacy-mode and metadata-only receipts:
  - `/Users/davidahmann/Projects/fabra_oss/src/fabra/context.py`
- Context budget/freshness/drop diagnostics:
  - `/Users/davidahmann/Projects/fabra_oss/src/fabra/context.py`
  - `/Users/davidahmann/Projects/fabra_oss/src/fabra/models.py`
- Content-address and immutability checks:
  - `/Users/davidahmann/Projects/fabra_oss/src/fabra/server.py`
  - `/Users/davidahmann/Projects/fabra_oss/src/fabra/store/offline.py`
- Thin adapter emission of context references:
  - `/Users/davidahmann/Projects/fabra_oss/src/fabra/adapters/openai.py`
  - `/Users/davidahmann/Projects/fabra_oss/src/fabra/adapters/langchain.py`
  - `/Users/davidahmann/Projects/fabra_oss/src/fabra/exporters/logging.py`

### Explicitly not borrowed as authority

- Non-JCS digest canonicalization paths:
  - `/Users/davidahmann/Projects/fabra_oss/src/fabra/utils/integrity.py`
- HMAC-style signing model:
  - `/Users/davidahmann/Projects/fabra_oss/src/fabra/utils/signing.py`

Gait keeps existing JCS + Ed25519 model as authority.

---

## Success Metrics and Release Gates (Locked)

### Product Metrics

- `P1`: Context evidence attachment rate in generated run/job packs >= 90% on v2.5 acceptance fixtures.
- `P2`: `gait gate eval` fail-closed behavior for required context evidence is deterministic (100% stable reason codes across reruns).
- `P3`: `gait pack diff` context drift classification hash-stable across repeated runs with same inputs.
- `P4`: `gait regress run` context conformance grader emits stable reason codes and exit behavior.

### Engineering Gates

- `E1`: Existing full UAT remains green: `bash scripts/test_uat_local.sh`.
- `E2`: Existing chaos suite remains green: `make test-chaos`.
- `E3`: Existing e2e + integration remain green:
  - `make test-e2e`
  - `go test ./internal/integration -count=1`
- `E4`: Existing perf gates remain green:
  - `make test-runtime-slo`
  - `make bench-check`
- `E5`: Code security gate remains green:
  - `make codeql`
- `E6`: New v2.5 context gates remain green:
  - `make test-v2-5-acceptance`
  - `make test-context-conformance`
  - `make test-context-chaos`

### Release Gate

v2.5 is releasable only when all `P1..P4` and `E1..E6` are green.

---

## Repository Touch Map (Planned)

```text
product/
  PLAN_v2.5.md

cmd/gait/
  main.go                                  # keep top-level dispatch consistent
  run_record.go                             # context evidence mode/input flags
  gate.go                                   # fail-closed context evidence controls
  pack.go                                   # inspect/diff/build context evidence views
  regress.go                                # context-aware bootstrap and grader hooks
  doctor.go                                 # context-proof diagnostics

core/
  contextproof/                             # new: canonical context evidence primitives
    envelope.go
    digest.go
    verify.go
    privacy.go
    diff.go
  gate/
    policy.go                               # context evidence policy knobs
    evaluate.go                             # enforcement + reason codes
    trace.go                                # context_set_digest trace binding
  runpack/
    record.go                               # required/best-effort context modes
    verify.go                               # context envelope verification hooks
  pack/
    pack.go                                 # include/inspect/diff context evidence
  regress/
    init.go                                 # context fixture bootstrap
    run.go                                  # deterministic context graders
  doctor/
    doctor.go                               # context-proof diagnostics checks

core/schema/v1/
  context/
    types.go                                # new context schema types
  gate/types.go                             # additive context fields
  runpack/types.go                          # additive context envelope refs
  pack/types.go                             # additive context summary in payloads/diff
  regress/types.go                          # context conformance result fields

schemas/v1/
  context/
    envelope.schema.json                    # new
    reference_record.schema.json            # new
    budget_report.schema.json               # new
  gate/
    intent_request.schema.json              # additive context evidence fields
    trace_record.schema.json                # additive context-set binding fields
    policy.schema.json                      # additive context-policy controls
  runpack/
    refs.schema.json                        # additive context evidence fields
  pack/
    run.schema.json                         # additive context summary fields
    diff.schema.json                        # additive context drift summary fields
  regress/
    regress_result.schema.json              # additive context grader result fields

sdk/python/gait/
  models.py                                 # IntentContext context-proof fields
  decorators.py                             # context resolver support
  adapter.py                                # pass-through context evidence options
  client.py                                 # CLI flag propagation
  session.py                                # emit context receipt refs

docs/
  contracts/contextspec_v1.md               # new: context evidence contract
  contracts/primitive_contract.md           # update command/exit/json contracts
  uat_functional_plan.md                    # include v2.5 gates
  test_cadence.md                           # include v2.5 cadence lanes
  integration_checklist.md                  # context-proof integration checklist
  approval_runbook.md                       # context-required approval flows
  slo/runtime_slo.md                        # add context operation budgets

scripts/
  test_v2_5_acceptance.sh                   # new
  test_context_conformance.sh               # new
  test_context_chaos.sh                     # new
  check_context_budgets.py                  # new

internal/
  integration/contextproof_test.go          # new
  e2e/v25_context_cli_test.go               # new

perf/
  context_budgets.json                      # new
  context_budget_report.json                # new output

.github/workflows/
  ci.yml                                    # new v2.5 lane jobs
  release.yml                               # add v2_5_gate release blocker
  hardening-nightly.yml                     # include context chaos/conformance
  perf-nightly.yml                          # include context budget checks
```

---

## Epic 0: ContextSpec v1 Contract Freeze

Objective: define one deterministic context evidence contract that integrates with existing run/gate/pack/regress artifacts.

### Story 0.1: Add new ContextSpec v1 schemas

Tasks:
- Add `/Users/davidahmann/Projects/gait/schemas/v1/context/envelope.schema.json`.
- Add `/Users/davidahmann/Projects/gait/schemas/v1/context/reference_record.schema.json`.
- Add `/Users/davidahmann/Projects/gait/schemas/v1/context/budget_report.schema.json`.

Minimum required envelope fields:
- `schema_id`, `schema_version`, `created_at`, `producer_version`
- `context_set_id`
- `context_set_digest` (JCS+sha256)
- `evidence_mode` (`best_effort|required`)
- `records[]` with:
  - `ref_id`
  - `source_type`
  - `source_locator`
  - `query_digest`
  - `content_digest`
  - `retrieved_at`
  - `redaction_mode`
  - `immutability` (`unknown|mutable|immutable`)
  - `freshness_sla_seconds` (optional)

Acceptance criteria:
- New schemas validate with existing schema tooling.
- Invalid fixtures fail deterministically with stable error messages.

### Story 0.2: Extend existing schemas additively

Tasks:
- Extend `/Users/davidahmann/Projects/gait/schemas/v1/gate/intent_request.schema.json` with optional:
  - `context.context_set_digest`
  - `context.context_refs[]`
  - `context.context_evidence_mode`
- Extend `/Users/davidahmann/Projects/gait/schemas/v1/gate/trace_record.schema.json` with optional:
  - `context_set_digest`
  - `context_evidence_mode`
  - `context_ref_count`
- Extend `/Users/davidahmann/Projects/gait/schemas/v1/pack/run.schema.json` with optional:
  - `context_set_digest`
  - `context_ref_count`
  - `context_evidence_mode`
- Extend `/Users/davidahmann/Projects/gait/schemas/v1/pack/diff.schema.json` with optional:
  - `context_drift_classification`
  - `context_changed`
  - `context_runtime_only_changes`
- Extend `/Users/davidahmann/Projects/gait/schemas/v1/regress/regress_result.schema.json` with optional context grader results.

Acceptance criteria:
- All changes are additive-only.
- Existing fixtures remain valid without modification unless explicitly updated for new optional fields.

### Story 0.3: Add Go schema types and fixtures

Tasks:
- Add `/Users/davidahmann/Projects/gait/core/schema/v1/context/types.go`.
- Update:
  - `/Users/davidahmann/Projects/gait/core/schema/v1/gate/types.go`
  - `/Users/davidahmann/Projects/gait/core/schema/v1/runpack/types.go`
  - `/Users/davidahmann/Projects/gait/core/schema/v1/pack/types.go`
  - `/Users/davidahmann/Projects/gait/core/schema/v1/regress/types.go`
- Add valid/invalid fixtures in `/Users/davidahmann/Projects/gait/core/schema/testdata/`.

Acceptance criteria:
- `go test ./core/schema/...` passes with context fixtures.

---

## Epic 1: Deterministic Context Envelope Runtime

Objective: create deterministic context evidence primitives in Go core and integrate them into capture/verify paths.

### Story 1.1: Implement `core/contextproof` package

Tasks:
- Add package `/Users/davidahmann/Projects/gait/core/contextproof/` with:
  - `BuildEnvelope(...)`
  - `DigestEnvelope(...)`
  - `VerifyEnvelope(...)`
  - deterministic sort and normalization utilities
  - privacy transform helpers (`metadata`, `hashes`, `raw`)
- Canonicalize all digest-bearing JSON using existing JCS utilities.

Acceptance criteria:
- Same logical context inputs produce identical envelope digest and serialized bytes.
- Non-deterministic input ordering does not affect output digest.

### Story 1.2: Integrate capture-path evidence modes

Tasks:
- Update `/Users/davidahmann/Projects/gait/cmd/gait/run_record.go` with flags:
  - `--context-envelope <path>`
  - `--context-evidence-mode best_effort|required`
  - `--unsafe-context-raw` (explicit unsafe gate for raw context content capture)
- Update `/Users/davidahmann/Projects/gait/core/runpack/record.go` to:
  - load and validate context envelope (if provided)
  - enforce required mode failure when context evidence is missing/invalid
  - include context envelope digest linkage in generated artifacts

Acceptance criteria:
- `required` mode fails with stable reason code when missing/invalid.
- `best_effort` mode emits warning but still records runpack with explicit warning field.

### Story 1.3: Pack inclusion and inspect wiring

Tasks:
- Update `/Users/davidahmann/Projects/gait/core/pack/pack.go`:
  - include `context_envelope.json` when present
  - carry context summary fields into run payload
- Update `/Users/davidahmann/Projects/gait/cmd/gait/pack.go` inspect output to show:
  - context digest
  - count
  - evidence mode
  - privacy mode indicator

Acceptance criteria:
- `gait pack inspect --json` deterministically includes context summary when available.
- Byte determinism remains intact for same inputs.

---

## Epic 2: Fail-Closed Context Policy Enforcement

Objective: allow production profiles to require context evidence at decision boundary.

### Story 2.1: Extend policy model for context requirements

Tasks:
- Update `/Users/davidahmann/Projects/gait/schemas/v1/gate/policy.schema.json` and Go policy model to support:
  - `require_context_evidence` (bool)
  - `required_context_evidence_mode` (`required`)
  - `max_context_age_seconds` (optional deterministic freshness guard)
- Keep defaults backward-compatible (unset means no additional requirement).

Acceptance criteria:
- Existing policies parse/evaluate unchanged.
- New policy fields validate and normalize deterministically.

### Story 2.2: Evaluator and reason-code contract

Tasks:
- Update:
  - `/Users/davidahmann/Projects/gait/core/gate/policy.go`
  - `/Users/davidahmann/Projects/gait/core/gate/evaluate.go`
- Add stable reason codes:
  - `context_evidence_missing`
  - `context_set_digest_missing`
  - `context_freshness_exceeded`
  - `context_evidence_mode_mismatch`
- Ensure enforcement behavior:
  - high-risk + requirement + missing evidence => block/fail-closed

Acceptance criteria:
- Repeated evaluations of same intent/policy emit identical verdict and reason codes.
- `oss-prod` style fail-closed behavior is preserved and extended for context evidence.

### Story 2.3: Trace binding and signing

Tasks:
- Update:
  - `/Users/davidahmann/Projects/gait/core/gate/trace.go`
  - `/Users/davidahmann/Projects/gait/schemas/v1/gate/trace_record.schema.json`
- Emit signed context linkage fields:
  - `context_set_digest`
  - `context_evidence_mode`
  - `context_ref_count`

Acceptance criteria:
- `gait trace verify` (or existing trace verification paths) validates signed records with context fields.
- Missing context fields for context-required decisions are detectable in audit output.

---

## Epic 3: Deterministic Context Diff and Regression Graders

Objective: make context drift actionable in CI without introducing noisy non-semantic failures.

### Story 3.1: Extend `pack diff` with context drift classification

Tasks:
- Update `/Users/davidahmann/Projects/gait/core/contextproof/diff.go` and `/Users/davidahmann/Projects/gait/core/pack/pack.go` to classify:
  - `none`
  - `runtime_only` (retrieved_at/order-only changes after normalization)
  - `semantic` (digest/material source changes)
- Reflect classification in `/Users/davidahmann/Projects/gait/schemas/v1/pack/diff.schema.json`.

Acceptance criteria:
- Same inputs always produce same classification and JSON output ordering.
- Runtime-only changes do not get mislabeled as semantic.

### Story 3.2: Regress context conformance grader

Tasks:
- Update:
  - `/Users/davidahmann/Projects/gait/core/regress/init.go`
  - `/Users/davidahmann/Projects/gait/core/regress/run.go`
- Add deterministic context grader checks:
  - required context evidence present
  - expected context_set_digest match (optional strict mode)
  - allowed drift policy (`none|runtime_only|semantic`)
- Expose CLI flags in `/Users/davidahmann/Projects/gait/cmd/gait/regress.go`:
  - `--context-conformance`
  - `--allow-context-runtime-drift`

Acceptance criteria:
- Grader exit codes and reason codes are stable and documented.
- Incident-to-regression flow can fail builds on semantic context drift.

### Story 3.3: Bootstrap defaults

Tasks:
- Update regression bootstrap defaults so v2.5 fixtures include context conformance expectations when context evidence exists.

Acceptance criteria:
- `gait regress bootstrap` generated configs remain deterministic and backwards-compatible for older runpacks.

---

## Epic 4: SDK and Integration Surface (Thin Adapter Strategy)

Objective: expose context evidence controls in Python SDK without moving policy logic out of Go.

### Story 4.1: Extend Python models and client pass-through

Tasks:
- Update:
  - `/Users/davidahmann/Projects/gait/sdk/python/gait/models.py`
  - `/Users/davidahmann/Projects/gait/sdk/python/gait/client.py`
  - `/Users/davidahmann/Projects/gait/sdk/python/gait/adapter.py`
  - `/Users/davidahmann/Projects/gait/sdk/python/gait/decorators.py`
  - `/Users/davidahmann/Projects/gait/sdk/python/gait/session.py`
- Add context fields to `IntentContext`:
  - `context_set_digest`
  - `context_refs`
  - `context_evidence_mode`
- Keep SDK as thin subprocess wrapper; no policy parsing/evaluation in Python.

Acceptance criteria:
- SDK can submit context-linked intents to `gait gate eval` and parse results deterministically.
- Existing SDK users not using context fields remain unaffected.

### Story 4.2: Integration examples and checklist

Tasks:
- Update docs and examples to show context evidence attachment flow for one adapter path (OpenAI agents reference path is sufficient for v2.5).
- Update `/Users/davidahmann/Projects/gait/docs/integration_checklist.md` with context-proof checklist items.

Acceptance criteria:
- One end-to-end example shows deterministic context-proof flow from capture -> gate -> pack -> regress.

---

## Epic 5: Doctor and Operator Diagnostics

Objective: make context-proof misconfiguration visible in first 5 minutes.

### Story 5.1: Doctor checks

Tasks:
- Update `/Users/davidahmann/Projects/gait/cmd/gait/doctor.go` and doctor core checks to add:
  - missing context evidence in required policy profiles
  - context envelope schema mismatch
  - context digest mismatch detection
  - unsafe raw context capture enabled warning

Acceptance criteria:
- `gait doctor --json` emits stable checks and fix guidance for context-proof failures.

### Story 5.2: Approval runbook updates

Tasks:
- Update `/Users/davidahmann/Projects/gait/docs/approval_runbook.md` with context-required approval and re-evaluation steps.

Acceptance criteria:
- Runbook includes exact commands and expected reason codes.

---

## Epic 6: Test Matrix Expansion (Mandatory, Full App Coverage)

Objective: update entire test matrix to cover all existing capabilities plus v2.5 context additions.

### 6.1 Local pre-PR fast gate (unchanged baseline + context quick lane)

Required local command set:

```bash
make prepush
make test-context-conformance
```

### 6.2 PR and mainline CI matrix updates

Update `/Users/davidahmann/Projects/gait/.github/workflows/ci.yml`:

- Add job `v2-5-acceptance`:
  - `make test-v2-5-acceptance`
- Add job `context-conformance`:
  - `make test-context-conformance`
- Add job `context-chaos`:
  - `make test-context-chaos`
- Keep existing jobs as required and untouched:
  - lint, docs-site, ui-local, ui-e2e-smoke, ui-acceptance
  - test (coverage), e2e, v1/v1.6/v1.7/v1.8 acceptance
  - v2.3 and v2.4 acceptance
  - contracts, policy-compliance, release/install smoke

Acceptance criteria:
- CI enforces v2.5 jobs without reducing existing matrix.
- Coverage checks remain at existing thresholds.

### 6.3 UAT orchestrator updates

Update:
- `/Users/davidahmann/Projects/gait/scripts/test_uat_local.sh`
- `/Users/davidahmann/Projects/gait/docs/uat_functional_plan.md`

Add required quality steps:
- `quality_v2_5_acceptance`: `make test-v2-5-acceptance`
- `quality_context_conformance`: `make test-context-conformance`
- `quality_context_chaos`: `make test-context-chaos`

Acceptance criteria:
- Local UAT summary includes v2.5 and context lanes with PASS/FAIL.
- Existing install-path checks (source/release/brew) remain intact.

### 6.4 Integration + e2e + acceptance additions

Add:
- `/Users/davidahmann/Projects/gait/internal/integration/contextproof_test.go`
- `/Users/davidahmann/Projects/gait/internal/e2e/v25_context_cli_test.go`
- `/Users/davidahmann/Projects/gait/scripts/test_v2_5_acceptance.sh`
- `/Users/davidahmann/Projects/gait/scripts/test_context_conformance.sh`
- `/Users/davidahmann/Projects/gait/scripts/test_context_chaos.sh`

`test_v2_5_acceptance.sh` minimum coverage:
- run record with context evidence mode (`best_effort`, `required`)
- gate eval fail-closed when context evidence required and missing
- pack build/inspect/diff context summaries
- regress context conformance grader behavior
- doctor context-proof diagnostics

Acceptance criteria:
- Acceptance script deterministic and offline by default.
- Exit behavior stable across repeated runs.

### 6.5 Performance and runtime budget lanes

Add:
- `/Users/davidahmann/Projects/gait/perf/context_budgets.json`
- `/Users/davidahmann/Projects/gait/scripts/check_context_budgets.py`

Integrate with:
- `make test-runtime-slo`
- `make bench-check`
- `/Users/davidahmann/Projects/gait/.github/workflows/perf-nightly.yml`

New budgeted operations:
- context envelope build/verify
- gate eval with context evidence requirement
- pack diff with context classification
- regress context grader run

Acceptance criteria:
- Context budget regressions are surfaced as CI failures with deterministic reports.

### 6.6 Chaos and resilience coverage

Extend chaos coverage with `make test-context-chaos`:

Required chaos cases:
- tampered context envelope digest
- context_set_digest mismatch between intent and trace
- required mode with missing envelope
- oversized context envelope payload limit enforcement
- stale-lock/contention scenario for concurrent context capture updates

Acceptance criteria:
- Chaos lane detects and blocks bypass paths.
- No reduction in existing `make test-chaos` suites.

### 6.7 Security scanning and supply chain checks

Required gates:
- `make lint` (includes gosec, govulncheck, bandit)
- `make codeql`
- existing release integrity flow remains unchanged

Update:
- `/Users/davidahmann/Projects/gait/.github/workflows/release.yml` to add `v2_5_gate` that runs:
  - `make test-v2-5-acceptance`
  - `make test-context-conformance`
  - `make test-context-chaos`
  - `make test-e2e`
  - `go test ./internal/integration -count=1`
  - `make test-runtime-slo`
  - `make bench-check`

Acceptance criteria:
- Release cannot proceed unless v2.5 context gates are green.

---

## Epic 7: Documentation and Operator Adoption

Objective: make v2.5 understandable and runnable in under 15 minutes.

### Story 7.1: Contract documentation

Tasks:
- Add `/Users/davidahmann/Projects/gait/docs/contracts/contextspec_v1.md`:
  - schema ids
  - required/optional fields
  - digest/signature rules
  - compatibility policy

Acceptance criteria:
- Document includes exact CLI examples and expected JSON snippets.

### Story 7.2: Primitive contract and cadence updates

Tasks:
- Update:
  - `/Users/davidahmann/Projects/gait/docs/contracts/primitive_contract.md`
  - `/Users/davidahmann/Projects/gait/docs/test_cadence.md`
  - `/Users/davidahmann/Projects/gait/docs/uat_functional_plan.md`
- Include v2.5 commands in required cadence and UAT pass criteria.

Acceptance criteria:
- Docs and Make/CI targets are aligned with no stale references.

### Story 7.3: Integration checklist and examples

Tasks:
- Update `/Users/davidahmann/Projects/gait/docs/integration_checklist.md` with context-proof integration requirements:
  - capture mode selection
  - required evidence policy wiring
  - trace verification expectations
  - regress context grader setup

Acceptance criteria:
- App team can self-audit context-proof readiness in under 30 minutes.

---

## Implementation Sequence (Strict Order)

1. Freeze ContextSpec v1 schemas and types (Epic 0).
2. Build core contextproof package and run/pack wiring (Epic 1).
3. Extend gate policy/evaluator/trace bindings (Epic 2).
4. Add diff/regress context grading (Epic 3).
5. Update SDK thin adapter pass-through (Epic 4).
6. Add doctor diagnostics and runbook docs (Epic 5).
7. Expand full test matrix + release gates (Epic 6).
8. Final docs and operator handoff (Epic 7).

No stage may skip tests for its touched area.

---

## Command Contract Additions (v2.5)

Planned additive command/flag contracts:

- `gait run record`:
  - `--context-envelope <path>`
  - `--context-evidence-mode best_effort|required`
  - `--unsafe-context-raw`
- `gait gate eval`:
  - no mandatory new flag required, but JSON output extends with context reason codes/fields
- `gait pack inspect --json`:
  - context summary fields when present
- `gait regress run`:
  - `--context-conformance`
  - `--allow-context-runtime-drift`
- `gait doctor --json`:
  - context-proof checks

All new flags require help text and JSON contract tests.

---

## Validation Plan (Executable)

### Stage A: Core/unit/schema

```bash
make fmt
make lint-fast
go test ./core/contextproof/... -count=1
go test ./core/gate/... -count=1
go test ./core/schema/... -count=1
(cd sdk/python && PYTHONPATH=. uv run --python 3.13 --extra dev pytest)
```

### Stage B: Integration/e2e/acceptance

```bash
make test-e2e
go test ./internal/integration -count=1
make test-v2-5-acceptance
make test-context-conformance
make test-context-chaos
```

### Stage C: Full quality and security gates

```bash
make lint
make test
make test-chaos
make test-runtime-slo
make bench-check
make codeql
bash scripts/test_uat_local.sh
```

### Stage D: Release gate simulation

```bash
make test-v2-3-acceptance
make test-v2-4-acceptance
make test-v2-5-acceptance
make test-packspec-tck
make test-context-conformance
make test-context-chaos
```

---

## Explicit Non-Goals (v2.5)

- No hosted dashboard or service control plane.
- No contextual retrieval engine, vector store, or feature-serving product.
- No natural-language classifier in enforcement path.
- No migration to non-Go policy logic.
- No dependency injection from `/Users/davidahmann/Projects/fabra_oss`.

---

## Exit Criteria Checklist (Must All Be True)

- ContextSpec v1 schemas and Go types are merged with additive compatibility.
- Run capture supports deterministic context evidence modes with stable fail-closed behavior.
- Gate traces bind context_set_digest for auditable decisions.
- Pack build/inspect/diff include deterministic context evidence summaries.
- Regress supports context conformance grading with stable exit and reason codes.
- SDK exposes context fields as thin pass-through only.
- Doctor surfaces context-proof configuration issues deterministically.
- CI, release, and local UAT matrix include v2.5 context lanes.
- Existing app capabilities and historical acceptance lanes remain green.

---

## Definition of Done (applies to every v2.5 story)

- Code formatted and linted (`make fmt`, `make lint`).
- Tests added and passing for touched areas plus full matrix where required.
- Any schema/artifact change includes:
  - schema update under `/Users/davidahmann/Projects/gait/schemas/v1/`
  - matching Go type update under `/Users/davidahmann/Projects/gait/core/schema/v1/`
  - validator fixtures under `/Users/davidahmann/Projects/gait/core/schema/testdata/`
- `--json` output changes are contract-tested with deterministic golden fixtures.
- Offline-first workflows remain functional.
- No external runtime dependency added.
