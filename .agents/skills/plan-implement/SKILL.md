---
name: plan-implement
description: Implement a Gait backlog plan end-to-end from product/PLAN_NEXT.md (or a specified plan), with strict branch bootstrap, per-story test wiring, CodeQL validation, and final DoD/acceptance revalidation. No issue/PR automation.
disable-model-invocation: true
---

# Plan Implementation (Gait)

Execute this workflow for: "implement the plan", "execute PLAN_NEXT", "ship plan stories", or "run backlog implementation end-to-end."

## Scope

- Repository: `/Users/davidahmann/Projects/gait`
- Default input plan: `/Users/davidahmann/Projects/gait/product/PLAN_NEXT.md`
- Optional input plan: user-specified file under `/Users/davidahmann/Projects/gait/product/`
- This skill executes code/docs/tests for planned stories.
- Out of scope by default:
- GitHub issue creation
- PR creation
- Automated comments/discussions
- Auto-commit/auto-push

## Preconditions

- Plan file exists and is readable.
- Plan includes:
- `Global Decisions (Locked)`
- `Exit Criteria`
- `Test Matrix Wiring`
- Story-level `Tasks`, `Repo paths`, `Run commands`, `Test requirements`, `Matrix wiring`, `Acceptance criteria`
- If required sections are missing, stop and report blockers.

## Git Bootstrap Contract (Mandatory)

Before implementation starts, run in order:

1. `git fetch origin main`
2. `git checkout main`
3. `git pull --ff-only origin main`
4. `git checkout -b codex/<plan-scope>`

Rules:
- If working tree is dirty before step 1, stop and report blocker for user decision.
- If unexpected changes appear during implementation, stop immediately and ask how to proceed.
- Do not switch to other branches unless user explicitly requests it.

## Workflow

1. Parse plan and build execution queue:
- Follow `Minimum-Now Sequence` first.
- Respect dependencies and `P0 -> P1 -> P2`.

2. Run baseline before first code change:
- `make lint-fast`
- `make test-fast`
- Record failures as `pre-existing` vs `introduced`.

3. Execute one story at a time:
- Implement only scoped story changes.
- Update tests required by story type.
- Update docs surgically only if user-facing behavior changed.
- Do not start next story until current story is validated.

4. Validate story completion:
- Run story `Run commands`.
- Run story `Test requirements`.
- Run story `Matrix wiring` lanes.
- If anything required is skipped, mark story incomplete.

5. Run epic-level validation after each epic:
- Execute relevant integration/acceptance suites for impacted surfaces.

6. Run final plan-level validation:
- `make prepush-full` (preferred, includes CodeQL), or
- `make prepush` and `make codeql` explicitly.
- Never finish without CodeQL unless user explicitly waives it.

7. Revalidate implementation against plan contracts:
- Re-check every implemented story against acceptance criteria.
- Re-check plan `Definition of Done`.

## Command Anchors

- `gait doctor --json` to verify local environment and dependency readiness before implementation.
- `gait gate eval --policy <policy.yaml> --input <intent.json> --json` for policy-story contract checks.
- `gait pack verify <artifact.zip> --json` for artifact-story integrity checks.
- Re-check `Exit Criteria`.
- Produce a `met/not met` checklist with command evidence for each item.

## Test Requirements by Work Type (Mandatory)

1. Schema/artifact contract changes:
- Schema validation tests
- Golden fixtures
- Compatibility/migration tests
- `make test-contracts`

2. CLI behavior changes (flags/JSON/exits):
- Command tests in `cmd/gait/*_test.go`
- JSON output stability tests
- Exit-code contract tests

3. Gate/policy/fail-closed changes:
- Deterministic allow/block/require_approval fixture tests
- Fail-closed undecidable-path tests
- Stable reason-code tests

4. Determinism/hash/sign/packaging changes:
- Repeat-run byte-stability tests
- Canonicalization/digest stability tests
- Verify/diff determinism checks
- `make test-packspec-tck` when relevant

5. Job runtime/state/concurrency changes:
- Lifecycle tests (submit/checkpoint/pause/resume/cancel)
- Atomic-write/crash-safety tests
- Contention/concurrency tests
- Chaos tests when scoped

6. SDK/adapter boundary changes:
- Wrapper behavior/error-mapping tests
- Adapter parity/conformance tests
- `make test-adapter-parity` when relevant

7. Voice/context evidence changes:
- `make test-voice-acceptance` and/or `make test-context-conformance` as applicable

8. Docs/examples changes:
- `make test-docs-consistency`
- `make test-docs-storyline` when operator flow changes

## Test Matrix Wiring (Enforcement)

Each story must map to and run required lanes:

- Fast lane: `make lint-fast`, `make test-fast`
- Core lane: targeted unit/integration suites
- Acceptance lane: relevant `make test-*-acceptance` targets
- Cross-platform lane: ensure Linux/macOS/Windows-safe behavior for touched surfaces
- Risk lane: determinism/safety/security/perf suites as required by story

No story is complete without passing its mapped lanes.

## Surgical Docs Sync Rule

If a story introduces user-visible behavior changes, update only impacted docs in the same story:

- `/Users/davidahmann/Projects/gait/README.md`
- `/Users/davidahmann/Projects/gait/docs/`
- `/Users/davidahmann/Projects/gait/docs-site/public/llms.txt`
- `/Users/davidahmann/Projects/gait/docs-site/public/llm/*.md`

If story is internal-only and behavior is unchanged, do not force doc churn.

## Safety Rules

- Preserve non-negotiables: determinism, offline-first, fail-closed, schema stability, stable exit codes.
- Never weaken non-allow => non-execute paths.
- No destructive git operations unless explicitly requested.
- No auto-commit or auto-push.
- Keep changes story-scoped; no unrelated refactors.

## Quality Rules

- Facts must be backed by command/test evidence.
- Do not claim tests ran if they did not.
- No silent skips of required checks.
- Tests must use temp output paths (no artifact leakage into source tree).
- If code/docs drift is introduced by user-facing change, patch docs in same story.

## Blocker Handling

If blocked:

1. Stop the blocked story.
2. Report exact blocker and impacted acceptance criteria.
3. Continue only independent unblocked stories.
4. End with minimum unblock actions.

## Completion Criteria

Implementation is complete only when all are true:

- All non-blocked in-scope stories are implemented.
- Story acceptance criteria are satisfied with evidence.
- Plan `Definition of Done` is satisfied.
- Plan `Exit Criteria` is satisfied.
- Required matrix lanes and CodeQL are passing.

## Expected Output

- `Execution summary`: completed/deferred/blocked stories
- `Change log`: files modified per story
- `Validation log`: commands and pass/fail results
- `Revalidation report`: story acceptance + DoD + exit criteria (`met/not met` with evidence)
- `Residual risk`: remaining gaps and next required stories
