---
name: branches-clean
description: Delete all non-main local and configured-remote branches one-by-one with safety checks and final prune/report.
disable-model-invocation: true
---

# Branches Clean (Gait)

Execute this workflow when asked to clean up branches by deleting all non-`main` branches locally and on a configured remote (default `origin`).

## Scope

- Repository: `/Users/davidahmann/Projects/gait`
- Target: all non-`main` branches
- Deletion style: one-by-one only (no grouped delete commands)
- Applies to:
- local branches
- remote `<remote>/*` branches (default `origin`)

## Input Contract

- `mode`: `execute` only
- `force_local_delete`: always `true` for this skill
- Optional:
- `remote`: defaults to `origin`

## Safety Rules

- Never delete `main`.
- Always switch to `main` before deletion.
- Always sync main before deletion:
- `git fetch <remote> main`
- `git checkout main`
- `git pull --ff-only <remote> main`
- No grouped branch deletion commands.
- Delete branches one-by-one only.

## Command Anchors (JSON Required)

- Capture machine-readable diagnostics before destructive actions:
  - `gait doctor --json`
  - `gait gate eval --policy examples/policy/base_low_risk.yaml --intent examples/policy/intents/intent_read.json --json`

## Workflow

1. Preflight:
- confirm repo is valid git repo
- resolve current branch
- verify `<remote>/main` exists
- switch to `main` and sync fast-forward
- fetch/prune remote refs (`git fetch --prune <remote>`)

2. Build delete candidates:
- Local candidates: all local branches except `main`
- Remote candidates: all `<remote>/<branch>` except `<remote>/main`
- Exclude symbolic refs (`<remote>/HEAD`) and `<remote>` root alias (`<remote>`) if present.

3. Execute deletion (always):
- Local deletion one-by-one:
- `git branch -D <branch>`
- Remote deletion one-by-one:
- `git push <remote> --delete <branch>`
- Never run `git push <remote> --delete <remote>`; `<remote>` is not a branch.
- if already deleted/not found, record as skipped and continue

4. Final reconciliation:
- `git fetch --prune <remote>`
- list remaining local branches
- list remaining `<remote>/*` branches

5. Output summary:
- deleted local branches
- deleted remote branches
- skipped/failed branches with reasons
- final remaining branch lists

## Command Discipline

Use one-by-one commands only, such as:

- `git branch -D <name>`
- `git push <remote> --delete <name>`

Do not use batched deletion forms.

## Shell Portability

- Do not rely on `mapfile`.
- Build branch candidate lists with portable `while IFS= read -r` loops.
- Ensure commands work under default macOS shell environments (`zsh`, older `bash`).

## Failure Handling

- Continue on per-branch failures; do not abort whole run.
- Record each failure with command + stderr reason.
- If remote delete returns `remote ref does not exist`, record as `skipped` and continue.
- If `<remote>/main` is missing, or if cannot switch/sync `main`, stop immediately.

## Expected Output

- `Mode`: execute
- `Local candidates`: list + count
- `Remote candidates`: list + count
- `Deleted local`: list
- `Deleted remote`: list
- `Skipped/failed`: list with reasons
- `Remaining local`: list
- `Remaining remote`: list
