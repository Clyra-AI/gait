# PLAN DS2: Documentation Coherence, Priming, and Integration Clarity

Date: 2026-02-15  
Source of truth checked in this review:
- `/Users/davidahmann/Projects/gait/README.md`
- `/Users/davidahmann/Projects/gait/docs/architecture.md`
- `/Users/davidahmann/Projects/gait/docs/flows.md`
- `/Users/davidahmann/Projects/gait/docs/concepts/mental_model.md`
- `/Users/davidahmann/Projects/gait/docs/scenarios/simple_agent_tool_boundary.md`
- `/Users/davidahmann/Projects/gait/docs/ci_regress_kit.md`
- `/Users/davidahmann/Projects/gait/docs/integration_checklist.md`
- `/Users/davidahmann/Projects/gait/docs/agent_integration_boundary.md`
- `/Users/davidahmann/Projects/gait/docs/demo_output_legend.md`
- `/Users/davidahmann/Projects/gait/docs-site/public/llm/product.md`
- `/Users/davidahmann/Projects/gait/docs-site/public/llm/contracts.md`
- `/Users/davidahmann/Projects/gait/docs-site/public/llms.txt`
- `/Users/davidahmann/Projects/gait/cmd/gait/verify.go`
- `/Users/davidahmann/Projects/gait/examples/integrations/openai_agents/quickstart.py`

Scope: docs and docs-site coherence track only (positioning clarity, naming consistency, flow ownership clarity, versioning semantics, and drift-prevention). No runtime behavior changes in this plan.

---

## Global Decisions (Locked For DS2)

- DS2 is a docs-system track, not a runtime feature track.
- Command semantics, schemas, and exit-code contracts remain unchanged.
- `docs/contracts/*` remains normative for behavior claims.
- Every high-level docs claim must map to:
  - one runnable command,
  - one concrete artifact path,
  - one explicit fail-closed statement where relevant.
- We separate docs planes consistently:
  - runtime integration plane,
  - operator CLI plane,
  - CI/release gate plane.
- **Preferred decision (locked): remove `v2.x` tags from evergreen page titles** and keep version history in release/changelog/compatibility docs.

---

## Advisor Feedback Validation (Against Current Repo State)

### Validated as true

1. Capability naming/list drift exists across docs.
2. Version tags in evergreen pages are unclear without context.
3. New docs are being added faster than legacy docs are reconciled.
4. Jobs flow is not first-class discoverable for users who land from high-level docs.
5. Flow/architecture diagrams still do not map steps to concrete code paths.
6. Tool-boundary concept is present but not canonically defined once and reused everywhere.
7. README lacks explicit `when to use` / `when not to use`.
8. Differentiation for durable jobs vs existing checkpoint/observability stacks is too implicit.

### Partially true (improved but still not strong enough)

1. "Go Core" confusion: explanatory notes exist, but terminology still creates cognitive load.
2. Demo purpose clarity: dedicated legend exists, but README framing still undersells demo as multi-branch learning surface.
3. Problem-first narrative: some pages include it, but docs system still trends primitive-first.

### Evidence anchors

- Version tags in evergreen docs:
  - `/Users/davidahmann/Projects/gait/docs/ci_regress_kit.md` title line
  - `/Users/davidahmann/Projects/gait/docs/integration_checklist.md` title line
  - `/Users/davidahmann/Projects/gait/docs/flows.md` section labels
- Terminology confusion:
  - `/Users/davidahmann/Projects/gait/docs/flows.md` (`Go Core`, `Policy Engine`)
  - `/Users/davidahmann/Projects/gait/docs/architecture.md` (`Go Core (core/*)`)
- Missing code-path linkage from diagrams:
  - contrast with concrete integration path in `/Users/davidahmann/Projects/gait/examples/integrations/openai_agents/quickstart.py`
- Exit code drift risk:
  - canonical constants in `/Users/davidahmann/Projects/gait/cmd/gait/verify.go`
  - differing summaries in docs/llm surfaces.

---

## Top Meta Lessons

1. **Docs contract drift is the main failure mode**
- Multiple pages describe the same surfaces independently, causing divergence in capability lists, exits, and positioning.

2. **Audience priming is underweighted**
- Readers are asked to parse primitives before the incident pain is made concrete.

3. **Ownership boundaries are not visually obvious**
- Diagrams describe flow but not clearly "your code vs Gait vs external tool."

4. **Version metadata is leaking into evergreen guidance**
- Release-lane language appears in docs that should stay timeless.

5. **Process is append-heavy**
- New pages are added, but no hard gate enforces synchronized updates to existing core docs.

---

## Meta-Problem Remedies (System Design)

### Remedy M1: Single Canonical Surface Taxonomy

Define one canonical capability taxonomy and require all overview pages to align with it.

Proposed canonical product surface (docs-facing):
- Core: `runpack/pack`, `gate`, `regress`, `doctor`, `jobs`
- Extended but first-class: `voice`, `context evidence`
- Supporting: `scout`, `registry`, `mcp`

Primary source location:
- Add/maintain one canonical page under `docs/contracts/` or `docs/` and reference it everywhere else.

### Remedy M2: Problem-First Narrative Contract

Every top-level onboarding doc must follow:
1. Incident/problem scenario
2. Why existing approaches fail
3. Which Gait surface fixes it
4. Exact command and artifact output
5. When to use / when not to use

### Remedy M3: Diagram Ownership Contract

All major diagrams must include explicit lanes:
- Your Runtime Code
- Gait Control/Evidence Layer
- Real Tool/External System

And include concrete path callouts for one reference integration.

### Remedy M4: Evergreen Title Policy

Evergreen docs must not include release labels in title text.
Release/version context belongs in:
- compatibility matrix,
- changelog/release notes,
- release-scoped plan docs.

### Remedy M5: Drift Prevention In CI

Add docs checks that fail CI on:
- capability list mismatch across key docs,
- exit code table mismatch against CLI constants,
- missing `when to use / when not to use` in required onboarding pages,
- new page added without nav/discoverability updates.

---

## Identified Issues and Concrete Remediation Workstreams

## Workstream A: Priming and Positioning Clarity

### Story A1: README Problem -> Solution Rewrite

Tasks:
- Lead with concrete incidents:
  - destructive tool action in production,
  - silent side-effect failure,
  - irreproducible bug/regression.
- Map each incident to specific Gait surfaces.
- Add explicit `When to use` and `When not to use`.

Files:
- `/Users/davidahmann/Projects/gait/README.md`

Acceptance criteria:
- Reader can explain why Gait exists before reading primitives.
- README includes explicit fit/non-fit guidance.

### Story A2: Mental Model Ordering Fix

Tasks:
- Reorder from primitives-first to incident-first.
- Keep primitive definitions concise and linked to incidents.

Files:
- `/Users/davidahmann/Projects/gait/docs/concepts/mental_model.md`

Acceptance criteria:
- First screen answers "why now" without needing other pages.

---

## Workstream B: Tool-Boundary Definition and Ownership Clarity

### Story B1: Canonical Tool-Boundary Definition

Tasks:
- Add one canonical definition block:
  - where boundary exists in code,
  - what crosses it (`IntentRequest`),
  - enforcement rule (`non-allow => non-execute`).
- Reuse exact wording across key docs.

Files:
- `/Users/davidahmann/Projects/gait/docs/concepts/mental_model.md`
- `/Users/davidahmann/Projects/gait/docs/scenarios/simple_agent_tool_boundary.md`
- `/Users/davidahmann/Projects/gait/docs/architecture.md`
- `/Users/davidahmann/Projects/gait/docs/flows.md`

Acceptance criteria:
- "Tool boundary" means one unambiguous thing across docs.

### Story B2: Ownership-Labeled Diagrams + Code Path Anchors

Tasks:
- Update architecture/flow diagrams with ownership lanes.
- Add "where this happens in repo" callouts:
  - reference `/Users/davidahmann/Projects/gait/examples/integrations/openai_agents/quickstart.py`.

Files:
- `/Users/davidahmann/Projects/gait/docs/architecture.md`
- `/Users/davidahmann/Projects/gait/docs/flows.md`
- `/Users/davidahmann/Projects/gait/docs/scenarios/simple_agent_tool_boundary.md`

Acceptance criteria:
- Reader can trace a runtime decision from diagram step to actual file path.

---

## Workstream C: Capability Naming and Surface Consistency

### Story C1: Cross-Doc Capability Harmonization

Tasks:
- Align capability lists in:
  - README,
  - docs overview/map pages,
  - docs-site `llm/*` and `llms.txt`.
- Ensure `doctor` and `jobs` are represented consistently with core taxonomy.

Files:
- `/Users/davidahmann/Projects/gait/README.md`
- `/Users/davidahmann/Projects/gait/docs/README.md`
- `/Users/davidahmann/Projects/gait/docs-site/public/llm/product.md`
- `/Users/davidahmann/Projects/gait/docs-site/public/llm/contracts.md`
- `/Users/davidahmann/Projects/gait/docs-site/public/llms.txt`

Acceptance criteria:
- No conflicting capability lists across these surfaces.

### Story C2: Demo Intent Clarification

Tasks:
- Make clear `gait demo` is a guided synthetic scenario exposing multiple feature branches.
- Keep `demo_output_legend` as field-level truth and link prominently.

Files:
- `/Users/davidahmann/Projects/gait/README.md`
- `/Users/davidahmann/Projects/gait/docs/demo_output_legend.md`

Acceptance criteria:
- Reader understands demo purpose before running it.

---

## Workstream D: Durable Jobs Clarity and Differentiation

### Story D1: Dedicated Durable Jobs Doc

Tasks:
- Add a dedicated jobs page explaining:
  - lifecycle semantics (`submit/status/checkpoint/pause/resume/cancel/approve/inspect`),
  - deterministic artifacts and verification paths,
  - differences vs observability/checkpoint tooling.
- Include "when this is better / when not necessary."

Files:
- New: `/Users/davidahmann/Projects/gait/docs/durable_jobs.md`
- Update links in docs nav and README.

Acceptance criteria:
- Jobs flow is discoverable without hunting in generic flow docs.

### Story D2: Comparative Positioning Section

Tasks:
- Add explicit comparison table (neutral and concrete):
  - hosted observability/checkpoint tools vs Gait runtime-control + signed artifact model.

Files:
- `/Users/davidahmann/Projects/gait/docs/durable_jobs.md`
- optionally mirrored short version in `/Users/davidahmann/Projects/gait/docs/concepts/mental_model.md`

Acceptance criteria:
- Reader can articulate differentiated value in one minute.

---

## Workstream E: Versioning Semantics and Evergreen Hygiene

### Story E1: Remove `v2.x` From Evergreen Titles (Preferred)

Tasks:
- Remove release tags from evergreen page titles/headers.
- Keep release-introduction history in section callouts or compatibility docs instead.

Initial candidate files:
- `/Users/davidahmann/Projects/gait/docs/ci_regress_kit.md`
- `/Users/davidahmann/Projects/gait/docs/integration_checklist.md`
- `/Users/davidahmann/Projects/gait/docs/flows.md` (section naming)
- any additional evergreen pages with release labels in title strings.

Acceptance criteria:
- Evergreen pages read as timeless guidance.
- Version-specific context still preserved in compatibility/changelog/release docs.

### Story E2: Add Versioning Explainers Where Needed

Tasks:
- Add concise "Version semantics" note:
  - what belongs to contracts,
  - what belongs to release plans,
  - where to find compatibility windows.

Primary references:
- `/Users/davidahmann/Projects/gait/docs/contracts/compatibility_matrix.md`
- `/Users/davidahmann/Projects/gait/docs/contracts/packspec_v1.md`

Acceptance criteria:
- No reader confusion around `v2.x` labels in operational docs.

---

## Workstream F: Similar-Issue Prevention Program

### Story F1: Docs Consistency Gate

Tasks:
- Add a docs linter/check script verifying:
  - capability surface consistency across key pages,
  - exit-code consistency against CLI constants,
  - required sections in onboarding docs,
  - no forbidden release tags in evergreen titles.

Potential script:
- `/Users/davidahmann/Projects/gait/scripts/test_docs_consistency.sh`

Acceptance criteria:
- CI fails deterministically on drift.

### Story F2: Docs Change Checklist

Tasks:
- Add PR checklist requiring:
  - README/docs/docs-site synchronization,
  - nav/discoverability updates for new docs,
  - if terminology changed, update glossary/canonical definition pages.

Files:
- contribution docs and PR templates in repo.

Acceptance criteria:
- "Add page without reconciling existing pages" becomes rare and auditable.

---

## Similar Issues To Proactively Hunt Next

1. Command examples that work in one page but fail in another due path/context assumptions.
2. Exit code references that omit `7` and `8` while code treats them as public behavior.
3. Docs-site `llm/*` simplifications that diverge from canonical docs.
4. Diagram actor labels that imply Gait is the agent runtime.
5. Terms that exist in multiple pages without a canonical definition anchor.

---

## Execution Order (Recommended)

1. Workstream A (priming)  
2. Workstream B (tool-boundary and ownership diagrams)  
3. Workstream E (evergreen title cleanup)  
4. Workstream C (capability harmonization incl. doctor/jobs lists)  
5. Workstream D (dedicated durable jobs path + differentiation)  
6. Workstream F (drift-prevention automation)

---

## Success Criteria (DS2 Exit)

- All advisor concerns are either closed or explicitly tracked with owner/date.
- Core docs answer:
  - what problem Gait solves,
  - where the boundary is,
  - where in code integration happens,
  - when to use and when not to use.
- Evergreen pages no longer carry `v2.x` title markers.
- Docs consistency checks run in CI and block regressions.

---

## Non-Goals

- Changing runtime semantics or CLI behavior.
- Replacing normative contract docs with marketing copy.
- Introducing hosted-service assumptions into core docs.
