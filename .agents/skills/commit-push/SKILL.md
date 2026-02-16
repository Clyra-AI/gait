---
name: commit-push
description: Commit and push all unstaged changes on the current branch, open/update PR, monitor CI to green, merge to main, monitor post-merge CI, and run up to 2 hotfix PR loops for actionable failures.
disable-model-invocation: true
---

# PR Ship Loop (Gait)

Execute this workflow for: "commit/push/open PR", "ship this branch", "merge after CI", or "post-merge fix loop."

## Scope

- Repository: `/Users/davidahmann/Projects/gait`
- Works on current local branch, then merges into `main`.
- No GitHub issue creation.
- PR text must use heredoc EOF bodies (no inline `--body` strings).

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

3. Local validation before commit:
- `make prepush-full`
- If it fails, stop and report; do not push.

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

8. Merge PR after green:
- Merge non-interactively (repo-default merge strategy or explicitly chosen one).
- Record merged PR URL and merge commit SHA.

9. Switch to main and sync:
- `git checkout main`
- `git pull --ff-only origin main`

10. Monitor post-merge CI on `main`:
- Watch the latest `main` CI run with timeout `25 minutes`.

11. Hotfix loop on post-merge red (max 2 loops):
- Run only for actionable failures.
- Loop cap: `2`.
- For each loop:
- Create branch from updated `main`: `codex/hotfix-<topic>-r<1|2>`
- Implement minimal fix.
- Run `make prepush-full`.
- `git add -A`
- `git commit -m "hotfix: <summary> (r<1|2>)"`
- `git push -u origin <hotfix-branch>`
- Open PR with heredoc EOF body.
- Monitor PR CI to green (25 min timeout).
- Merge PR.
- `git checkout main && git pull --ff-only origin main`
- Monitor post-merge CI again (25 min timeout).

12. Stop conditions:
- CI green on main: success.
- Non-actionable failure class: stop and report.
- Loop count exceeded (`>2`): stop and report blocker.

## Command Anchors

- `gait doctor --json` before ship to capture machine-readable local readiness evidence.
- `gait pack verify <artifact.zip> --json` when validating artifact integrity in a failing CI path.
- `gait gate eval --policy <policy.yaml> --intent <intent.json> --json` when policy-path checks are implicated.

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
- Keep fixes scoped to CI failure root cause.
- If unexpected repo state appears, stop and ask.

## CI Policy

- Required local gate before push: `make prepush-full` (includes CodeQL in this repo).
- PR CI watch timeout: `25 minutes`.
- Post-merge main CI watch timeout: `25 minutes`.
- Retry/hotfix loop cap: `2`.

## Expected Output

- Branch name(s)
- Commit SHA(s)
- PR URL(s)
- CI status per cycle
- Merge commit SHA(s)
- Post-merge CI status on `main`
- If stopped: blocker reason and last failing check
