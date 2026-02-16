---
name: backlog-plan
description: Transform strategic recommendations in product/ideas.md into an execution-ready Gait backlog plan with epics, stories, tasks, repo paths, commands, acceptance criteria, and explicit test-matrix wiring.
disable-model-invocation: true
---

# Ideas to Backlog Plan (Gait)

Execute this workflow when asked to convert strategic feature recommendations into a concrete product backlog plan.

## Scope

- Repository root: `/Users/davidahmann/Projects/gait`
- Input file: `/Users/davidahmann/Projects/gait/product/ideas.md`
- Structure references (match level of detail and style):
- `/Users/davidahmann/Projects/gait/product/PLAN_v1.md`
- `/Users/davidahmann/Projects/gait/product/PLAN_v1.7.md`
- `/Users/davidahmann/Projects/gait/product/PLAN_HARDENING.md`
- `/Users/davidahmann/Projects/gait/product/PLAN_ADOPTION.md`
- `/Users/davidahmann/Projects/gait/product/PLAN_ENT.md`
- Output file: `/Users/davidahmann/Projects/gait/product/PLAN_NEXT.md` (unless user specifies a different target)
- Planning only. Do not implement code or docs outside the target plan file.

## Preconditions

- `ideas.md` must contain strategic recommendations with evidence.
- Each recommendation should include:
- recommendation name
- why-now trigger
- strategic capability direction
- moat/benefit rationale
- source links

If these are missing, stop and output a gap note instead of inventing details.

## Workflow

1. Read `ideas.md` and extract candidate initiatives.
2. Read reference plans to mirror structure and detail level.
3. Cluster ideas into coherent epics (avoid one-idea-per-epic fragmentation).
4. Prioritize using `P0/P1/P2` based on contract risk reduction, moat expansion, adoption leverage, and sequencing dependency.
5. Produce execution-ready epics and stories.
6. For every story, include concrete tasks, repo paths, run commands, acceptance criteria, test requirements, and CI matrix wiring.
7. Build a plan-level test matrix section mapping stories to CI lanes (fast, integration, acceptance, cross-platform).
8. Ensure each story defines tests based on work type (schema, CLI, gate/policy, determinism, runtime, SDK, docs/examples).
9. Add explicit boundaries and non-goals to prevent scope drift.
10. Add delivery sequencing section (phase/week-based minimum-now path).
11. Add definition of done and release/exit gate criteria.
12. Write full plan to target file, overwriting prior contents.

## Non-Negotiables

- Preserve Gait core contracts: determinism, offline-first defaults, fail-closed policy posture, schema stability, and exit code stability.
- Respect architecture boundaries: Go core is authoritative for enforcement/verification logic; Python remains thin adoption layer.
- Avoid dashboard-first or hosted-only dependencies in backlog core.
- Do not include implementation code, pseudo-code, or ticket boilerplate.
- Do not recommend minor polish work as primary backlog items.
- Every story must include test requirements and explicit matrix wiring.
- No story is complete without same-change test updates, except explicitly justified docs-only stories.

## Test Requirements by Work Type (Mandatory)

1. Schema or artifact contract work:
- Add schema validation tests.
- Add or update golden fixtures.
- Add compatibility or migration tests.

2. CLI surface work (flags, args, `--json`, exits):
- Add command tests for help/usage behavior.
- Add `--json` stability tests.
- Add exit code contract tests.

3. Gate or policy semantics:
- Add deterministic allow/block/require_approval fixture tests.
- Add fail-closed tests for evaluator-missing or undecidable paths.
- Add reason code stability checks.

4. Determinism, hashing, signing, packaging:
- Add byte-stability tests for repeated runs with identical input.
- Add canonicalization and digest stability tests.
- Add verify/diff determinism tests.

5. Job runtime, state, concurrency, persistence:
- Add pause/resume/cancel/checkpoint lifecycle tests.
- Add crash-safe/atomic write tests.
- Add concurrent execution and contention tests.

6. SDK or adapter boundary work:
- Add wrapper behavior/error-mapping tests.
- Add adapter conformance tests against canonical sidecar/gate path.
- Preserve Go-authoritative decision boundary tests.

7. Docs/examples contract changes:
- Add command-smoke checks for documented flows.
- Add docs-versus-CLI parity checks where possible.
- Update acceptance scripts if docs alter required operator path.

## Test Matrix Wiring Contract (Plan-Level)

Every generated plan must include a section named `Test Matrix Wiring` with:

- `Fast lane`: pre-push or quick CI checks required for each epic.
- `Core CI lane`: required unit/integration UAT checks in default CI.
- `Acceptance lane`: deterministic acceptance scripts required before merge or release.
- `Cross-platform lane`: Linux/macOS/Windows expectations for affected stories.
- `Risk lane`: extra suites for high-risk stories (policy, determinism, security, portability).
- `Gating rule`: merge/release block conditions tied to failed required lanes.

## Plan Format Contract

Required top sections:

1. `# PLAN <name>: <theme>`
2. `Date`, `Source of truth`, `Scope`
3. `Global Decisions (Locked)`
4. `Current Baseline (Observed)`
5. `Exit Criteria`
6. `Test Matrix Wiring`
7. `Epic` sections with `Objective` and `Story` breakdowns
8. `Minimum-Now Sequence` (phased execution)
9. `Explicit Non-Goals`
10. `Definition of Done`

Story template (required fields):

- `### Story <ID>: <title>`
- `Priority:`
- `Tasks:`
- `Repo paths:`
- `Run commands:`
- `Test requirements:`
- `Matrix wiring:`
- `Acceptance criteria:`
- Optional when needed:
- `Dependencies:`
- `Risks:`

## Quality Gate for Output

Before finalizing, verify:

- Every epic maps back to at least one idea from `ideas.md`.
- Every story is actionable without guesswork.
- Acceptance criteria are testable and deterministic.
- Paths are real and repo-relevant.
- Test requirements match story work type.
- Matrix wiring is present for every story.
- Sequence is dependency-aware.
- Plan stays strategic and execution-relevant (not cosmetic).

## Command Anchors

- Include concrete plan tasks that reference verifiable CLI surfaces, for example:
  - `gait doctor --json`
  - `gait gate eval --policy <policy.yaml> --intent <intent.json> --json`
  - `gait pack verify <pack.zip> --json`

## Failure Mode

If `ideas.md` lacks strategic quality or evidence, write only:

- `No backlog plan generated.`
- `Reason:` concise blocker summary.
- `Missing inputs:` exact missing fields required to proceed.

Do not fabricate backlog content when source strategy quality is insufficient.
