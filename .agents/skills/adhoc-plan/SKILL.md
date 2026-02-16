---
name: adhoc-plan
description: Convert user-provided recommended work items into an execution-ready Gait backlog plan at a user-provided output path, with epics, stories, test requirements, and CI matrix wiring.
disable-model-invocation: true
---

# Recommendations to Backlog Plan (Gait)

Execute this workflow when the user asks to turn recommended items into a concrete backlog plan before implementation.

## Scope

- Repository root: `/Users/davidahmann/Projects/gait`
- Recommendation source: user-provided recommended items for this run
- No dependency on `/Users/davidahmann/Projects/gait/product/ideas.md`
- Planning-only skill. Do not implement code in this workflow.

## Input Contract (Mandatory)

- `recommended_items`: structured list or raw text of recommended work to plan.
- `output_plan_path`: absolute or repo-relative file path where the generated plan will be written.

Validation rules:
- Both arguments are required.
- `output_plan_path` must resolve inside the repository and be writable.
- If either input is missing or invalid, stop and output a blocker report.

## Workflow

1. Parse `recommended_items` and normalize each item to:
- recommendation
- why
- strategic direction
- expected moat/benefit
2. Remove duplicates and out-of-scope items.
3. Cluster recommendations into coherent epics.
4. Prioritize with `P0/P1/P2` using contract risk, moat gain, adoption leverage, and dependency order.
5. Create execution-ready stories with:
- tasks
- repo paths
- run commands
- test requirements
- matrix wiring
- acceptance criteria
6. Add plan-level `Test Matrix Wiring`.
7. Add `Recommendation Traceability` mapping recommendations to epic/story IDs.
8. Add `Minimum-Now Sequence`, `Exit Criteria`, and `Definition of Done`.
9. Verify quality gates.
10. Overwrite `output_plan_path` with the final plan.

## Command Contract (JSON Required)

Use `gait` commands with `--json` whenever the plan needs machine-readable evidence, for example:

- `gait doctor --json`
- `gait regress bootstrap --from fixtures/run_demo/runpack.zip --json`

## Non-Negotiables

- Preserve Gait contracts:
- determinism
- offline-first defaults
- fail-closed policy enforcement
- schema stability
- exit code stability
- Respect architecture boundaries:
- Go core authoritative for enforcement/verification
- Python remains thin adoption layer
- No dashboard-first scope in core backlog.
- No minor polish as primary backlog.
- Every story must include tests and matrix wiring.

## Test Requirements by Work Type (Mandatory)

1. Schema/artifact changes:
- schema validation tests
- fixture/golden updates
- compatibility/migration tests

2. CLI behavior changes:
- help/usage tests
- `--json` stability tests
- exit-code contract tests

3. Gate/policy/fail-closed changes:
- deterministic allow/block/require_approval fixtures
- fail-closed undecidable-path tests
- reason-code stability checks

4. Determinism/hash/sign/packaging changes:
- byte-stability repeat-run tests
- canonicalization/digest checks
- verify/diff determinism tests
- `make test-packspec-tck` when applicable

5. Job runtime/state/concurrency changes:
- lifecycle tests
- crash-safe/atomic-write tests
- contention/concurrency tests
- chaos suites when applicable

6. SDK/adapter boundary changes:
- wrapper error-mapping tests
- adapter parity/conformance tests

7. Voice/context-proof changes:
- voice/context conformance acceptance suites as applicable

8. Docs/examples changes:
- docs consistency checks
- storyline/smoke checks when user flow changes

## Test Matrix Wiring Contract (Plan-Level)

The plan must include a `Test Matrix Wiring` section with:

- Fast lane
- Core CI lane
- Acceptance lane
- Cross-platform lane
- Risk lane
- Merge/release gating rule

Every story must declare its lane wiring.

## Plan Format Contract

Required sections:

1. `# PLAN <name>: <theme>`
2. `Date`, `Source of truth`, `Scope`
3. `Global Decisions (Locked)`
4. `Current Baseline (Observed)`
5. `Exit Criteria`
6. `Recommendation Traceability`
7. `Test Matrix Wiring`
8. Epic sections with objectives and stories
9. `Minimum-Now Sequence`
10. `Explicit Non-Goals`
11. `Definition of Done`

Story template:

- `### Story <ID>: <title>`
- `Priority:`
- `Tasks:`
- `Repo paths:`
- `Run commands:`
- `Test requirements:`
- `Matrix wiring:`
- `Acceptance criteria:`
- Optional: `Dependencies:`, `Risks:`

## Quality Gate

Before finalizing:

- Every recommendation maps to at least one epic/story.
- Every story is actionable without guesswork.
- Acceptance criteria are testable and deterministic.
- Paths are real and repo-relevant.
- Test requirements match story type.
- Matrix wiring exists for every story.
- Sequence is dependency-aware and implementation-ready.

## Failure Mode

If inputs are missing or recommendations are not plan-ready, write only:

- `No backlog plan generated.`
- `Reason:` concise blocker summary.
- `Missing inputs:` exact required fields.

Do not fabricate backlog content.
