---
name: pr-comments
description: Analyze Codex/agent review comments on one or more PRs, triage what should be implemented, and provide a clear implementation recommendation list for the user.
disable-model-invocation: true
---

# PR Comments Triage (Gait)

Execute this workflow when asked to review PR comments from agents/Codex and recommend what should actually be implemented and how.

## Scope

- Repository context: `/Users/davidahmann/Projects/gait`
- Input PRs: user-provided PR number(s)
- Mode: analysis and recommendation only (read-only by default)
- No code changes, commits, or pushes in this skill unless explicitly requested in a follow-up

## Input Contract (Mandatory)

- `pr_numbers`: one or more PR numbers
- Optional:
- `repo` (owner/name if not inferable from current git remote)
- `include_non_agent_comments` (`false` by default)

If `pr_numbers` is missing, stop and report blocker.

## Workflow

1. Resolve repository and PR targets from input.
2. For each PR:
- fetch PR metadata (state, base/head, latest head SHA)
- fetch review comments, review summaries, and issue comments
- collect inline comment context (file + line + diff hunk when available)
3. Filter comments:
- include agent/Codex comments by default
- exclude outdated/stale comments not applicable to latest head SHA
- collapse duplicates/near-duplicates
4. Triage each comment into:
- `implement`
- `defer`
- `reject`
5. Score each triaged item:
- severity (`P0/P1/P2/P3`)
- confidence (`high/medium/low`)
- impact area (`safety`, `determinism`, `contract`, `portability`, `docs`, `maintainability`)
6. Produce implementation guidance for `implement` items:
- what to change
- why it matters
- minimal safe fix direction
- tests/validation required
7. Produce rationale for `defer/reject`:
- reason and conditions to revisit
8. Generate final user-facing recommendation report.

## Triage Rules (Gait-Specific)

Prioritize comments that affect:

1. Fail-closed behavior and enforcement boundaries.
2. Determinism (verify/diff/replay/pack reproducibility).
3. Schema and CLI contract stability (`--json`, exit codes, compatibility).
4. Security/privacy controls (signing, secrets handling, unsafe interlocks).
5. Cross-platform/CI portability issues.
6. Docs drift where behavior has changed.

Deprioritize or reject:

- style-only nits with no runtime/contract impact
- speculative refactors outside PR scope
- stale comments already fixed on newer commits

## Command Anchors

- Use `gait` JSON outputs when validating recommendation risk:
  - `gait doctor --json`
  - `gait gate eval --policy <policy.yaml> --intent <intent.json> --json`
  - `gait pack verify <pack.zip> --json`

## Recommendation Format (Per Implement Item)

- `PR`: number
- `Comment ref`: reviewer + permalink (or comment id)
- `Location`: file:line (if inline)
- `Decision`: implement
- `Severity`: P0/P1/P2/P3
- `Why`: risk and concrete break scenario
- `How`: concise implementation direction (no code unless asked)
- `Validation`: exact tests/checks to run

## Output Contract

Return sections in this order:

1. `Implement Now` (ordered by severity, then confidence)
2. `Defer` (with trigger/condition for future action)
3. `Reject` (with concise rationale)
4. `Cross-PR Patterns` (optional: repeated root causes seen across PRs)
5. `Validation Plan` (commands/checks required if user asks to implement)

## Safety Rules

- Read-only analysis by default.
- Do not edit files or run git write operations in this skill.
- Do not claim a comment is resolved unless verified against current head.
- Keep recommendations scoped to actual comment evidence.

## Quality Rules

- Evidence-first: every recommendation must map to a real comment.
- Distinguish facts from inference.
- Avoid generic advice; tie guidance to exact files/contracts.
- Prefer minimal-risk changes over broad refactors.

## Failure Mode

If comments cannot be fetched or are insufficient:

- `No actionable PR comment triage produced.`
- `Reason:` concise blocker
- `Missing inputs/access:` exact requirement (PR numbers, repo, auth, permission)

Do not fabricate recommendations.
