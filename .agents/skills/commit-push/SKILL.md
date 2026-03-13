---
name: commit-push
description: Commit and push all unstaged changes on the current branch, open/update PR, fix actionable blockers through CI and review loops, merge to main, and keep driving post-merge CI to green until only a hard blocker remains.
disable-model-invocation: true
---

# PR Ship Loop (Gait)

Execute this workflow for: "commit/push/open PR", "ship this branch", "merge after CI", or "post-merge fix loop."

## Scope

- Repository: `/Users/tr/gait`
- Works on current local branch, then merges into `main`.
- No GitHub issue creation.
- PR text must use heredoc EOF bodies (no inline `--body` strings).
- Default posture: when a blocker is actionable from the repo, branch, or CI logs, fix it in-loop and continue instead of stopping at first failure.

## Preconditions

- Current branch is not `main`.
- Local changes exist or an existing PR already exists for the branch.
- `gh` auth is available for repo operations.

If preconditions fail, stop and report.

## Workflow

1. Preflight and branch safety:
- `git status --short`
- `git rev-parse --abbrev-ref HEAD`
- If on `main`, stop.
- If unexpected unrelated changes are present, stop and report.

2. Sync branch base:
- `git fetch origin main`
- Ensure current branch is rebased/merged with latest `origin/main` using non-interactive commands.

3. Local validation and remediation before commit:
- `make prepush-full`
- If it fails for actionable repo/test/lint/config issues, inspect the failures, implement the minimal fix set, and rerun until green.
- In each remediation pass, fix all currently-known actionable failures before rerunning, not just the first one encountered.
- Stop only for non-actionable external blockers, unsafe repo state, or 2 consecutive no-progress remediation passes.

4. Stage and commit all unstaged files on this branch:
- `git add -A`
- `git commit -m "<scope>: <summary>"` (skip commit only if no changes after staging)

5. Push branch:
- `git push -u origin <branch>` (or `git push origin <branch>` if upstream already set)

6. Open or update PR:
- If PR exists for head branch, reuse it.
- Otherwise create PR with heredoc body:
- `gh pr create --title "..." --body-file - <<'EOF'`
- `<problem / changes / validation>`
- `EOF`
- Do not use inline shell-expanded body text.

7. Monitor PR CI until green:
- Watch required checks/run(s) with timeout `25 minutes`.
- Use polling/watch (for example every 10s).
- If green, continue.
- If failed, classify failure:
- actionable product/test failure
- flaky/infra/transient
- permission/workflow policy failure
- For actionable product/test/workflow failures that can be fixed on the branch, enter remediation immediately, fix the full actionable set from the failing run, push, and continue the PR loop.
- For flaky/infra/transient failures, retry or rerun non-interactively when appropriate and continue; stop only if the failure persists and is not repo-fixable.
- For permission/workflow policy failures, fix them when the remedy is in-repo or branch-scoped; otherwise stop and report the external blocker.

8. Codex review settle gate (mandatory, passive, latest-head preferred):
- After PR creation/update and green CI, inspect Codex review output before merge.
- Poll PR reviews/comments/reactions every `15s`, preferring signals tied to the latest PR head SHA.
- Never post `@codex review` or any other PR/issue comment solely to solicit review.
- Default reviewer identity for this gate: `chatgpt-codex-connector` (GitHub UI may render as `chatgpt-codex-connector bot`).
- Accepted settle signals on the latest PR head:
- actionable Codex review comments/suggestions -> proceed to pre-merge fix loop
- explicit approval/all-good signal -> review gate satisfied
- Codex-authored `+1` / thumbs-up on PR body, issue comment, or review comment -> review gate satisfied when required PR CI is green and no unresolved `P0/P1` Codex items remain
- Codex-authored `eyes` reaction on the PR body, issue comment, or review comment means review is in progress:
- treat `eyes` as a live in-progress signal, not a terminal settle signal
- once `eyes` is observed, assume a terminal Codex signal should follow soon and continue polling every `30s` for up to `15 minutes` before declaring the gate blocked
- once `eyes` is observed, wait for a terminal Codex signal (`actionable`, `approved`, `thumbs_up`, explicit service/quota failure, or a new actionable comment/review on the latest head) before deciding the gate result
- Use a two-stage gate:
- Stage A: for the first `15 minutes`, look for latest-head Codex terminal signals or an `eyes` in-progress signal
- Stage B: if `eyes` was observed during Stage A or later, continue polling every `30s` for up to `15 minutes` from the first observed `eyes` on the current review cycle
- If no terminal Codex signal appears within that `15 minute` Stage B window, stop and report a blocked Codex review gate
- If no latest-head terminal signal appears and no `eyes` in-progress signal is seen during Stage A, fall back to PR-wide Codex review inventory:
- collect prior Codex reviews, inline comments, issue comments, and Codex-authored `+1` / thumbs-up reactions on the PR
- require at least one prior Codex review artifact before permitting `carry_forward`
- if required PR CI is green and no unresolved `P0/P1` Codex items remain, treat gate as satisfied as `carry_forward`
- if unresolved `P0/P1` Codex items remain, stop and report blocker
- if no prior Codex review artifact exists, stop and report blocker
- if Codex explicitly reports service/quota failure for automatic review, stop and report blocker
- Do not create a new GitHub comment to force or retry review.

9. Pre-merge unresolved comment triage and fix loop:
- Fetch unresolved PR review threads/comments, preferring latest-head Codex items first, then any GitHub Advanced Security inline comments on the latest head, and then any still-open carry-forward `P0/P1` items from earlier heads.
- Triage each unresolved item as `implement`, `blocked`, `defer`, `reject`, or `already_satisfied`.
- Resolve the corresponding GitHub review thread for `implement` or `already_satisfied` items once they are satisfied on the current head.
- Do not resolve threads for `blocked`, `defer`, or `reject`.
- Treat deterministic GitHub Advanced Security inline comments as `implement` by default when they describe a concrete code pattern on the current head.
- Auto-fix only `implement` items that are:
- `P0/P1`, or
- high-confidence `P2` with concrete repro or break path
- For each fix loop:
- batch all compatible `implement` items for the current head into the smallest coherent fix set; do not stop after addressing a single comment if more actionable blockers are known
- apply the minimal scoped fix on the same PR branch
- run `make prepush-full`
- `git add -A`
- `git commit -m "fix: address actionable PR comments (loop <n>)"` (skip only if no changes)
- push branch
- re-watch PR CI to green
- resolve satisfied GitHub review threads/comments
- re-run the passive Codex review settle gate on the new latest PR head SHA
- re-fetch unresolved threads/comments
- Continue looping while unresolved actionable items remain and each cycle makes progress.
- Stop only if remaining blockers are external/non-actionable/safety-blocked, or if 2 consecutive cycles fail to reduce the actionable blocker set.

10. Merge PR after green and review gate satisfied:
- Merge only when all are true on the latest PR head SHA:
- required PR CI is green
- Codex review settle gate is satisfied (`approved`, `thumbs_up`, `actionable` resolved, or `carry_forward`)
- no unresolved `P0/P1` review items remain
- Merge non-interactively (repo-default merge strategy or explicitly chosen one).
- Record merged PR URL and merge commit SHA.

11. Switch to main and sync:
- `git checkout main`
- `git pull --ff-only origin main`

12. Monitor post-merge CI on `main`:
- Watch the latest `main` CI run with timeout `25 minutes`.

13. Hotfix loop on post-merge red:
- Run only for actionable or repo-fixable failures.
- For each loop:
- Create branch from updated `main`: `codex/hotfix-<topic>-r<n>`
- Implement the minimal fix set that clears all actionable blockers visible in the failing run.
- Run `make prepush-full`.
- `git add -A`
- `git commit -m "hotfix: <summary> (r<n>)"`
- `git push -u origin <hotfix-branch>`
- Open PR with heredoc EOF body.
- Monitor PR CI to green (25 min timeout).
- Merge PR.
- `git checkout main && git pull --ff-only origin main`
- Monitor post-merge CI again (25 min timeout).
- Continue hotfixing while failures remain actionable and each cycle makes progress.
- Stop only for external/non-actionable blockers, safety blockers, or 2 consecutive no-progress hotfix cycles.

14. Stop conditions:
- CI green on main: success.
- Codex gate unresolved after passive latest-head poll plus PR-wide carry-forward triage, without any Codex `eyes` in-progress signal: stop and report blocker.
- Codex gate remains pending with Codex `eyes` in-progress signal: continue waiting up to the `15 minute` Stage B window; only then stop and report blocker if no terminal signal appears.
- Unresolved pre-merge actionable comments after 2 consecutive no-progress cycles: stop and report blocker.
- Non-actionable or external failure class: stop and report.
- Safety blocker or unexpected repo state that cannot be reconciled safely: stop and report blocker.

## Command Anchors

- `gait doctor --json` before ship to capture machine-readable local readiness evidence.
- `gait pack verify <artifact.zip> --json` when validating artifact integrity in a failing CI path.
- `gait gate eval --policy <policy.yaml> --intent <intent.json> --json` when policy-path checks are implicated.
- Use `gh pr view --json number,headRefOid` and `gh repo view --json nameWithOwner` to seed Codex review inspection.
- Use `gh api` against PR reviews, review comments, issue comments, and their reactions to inspect latest-head Codex signals, `eyes` in-progress reactions, and carry-forward artifacts.

## EOF Rule (Mandatory)

For all PR descriptions/comments, use only heredoc with single-quoted delimiter:

`--body-file - <<'EOF'`
`...text...`
`EOF`

Never use inline `--body "..."` for multi-line PR text.

## Safety Rules

- Never use destructive git commands unless explicitly requested.
- Never amend commits unless explicitly requested.
- Never create duplicate PRs for the same head branch.
- Never post `@codex review` or equivalent review-trigger comments.
- Never merge with unresolved `P0/P1` Codex review items.
- Never leave an implemented Codex review thread unresolved before merge.
- Keep fixes scoped to the CI or review root cause.
- If unexpected repo state appears, stop and ask.

## CI Policy

- Required local gate before push: `make prepush-full` (includes CodeQL in this repo).
- PR CI watch timeout: `25 minutes`.
- Codex review settle polling interval: `15 seconds`.
- Codex review settle initial window: `15 minutes` to find latest-head terminal signals or an `eyes` in-progress signal.
- Codex review settle after `eyes`: poll every `30 seconds` for up to `15 minutes` before blocking if no terminal signal appears.
- Local/PR/hotfix remediation policy: continue while failures are actionable and each cycle makes progress; stop after `2` consecutive no-progress cycles or when the blocker is external or safety-critical.
- Post-merge main CI watch timeout: `25 minutes`.

## Expected Output

- Branch name(s)
- Commit SHA(s)
- PR URL(s)
- CI status per cycle
- Codex review settle status per cycle
- Resolved review thread/comment refs
- Merge commit SHA(s)
- Post-merge CI status on `main`
- If stopped: blocker reason and last failing check
