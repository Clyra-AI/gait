---
name: branches-clean
description: Delete all non-main local and configured-remote branches one-by-one with safety checks, dry-run support, and final prune/report.
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

- `mode`: `dry-run` or `execute`
- Optional:
- `force_local_delete`: `false` by default
- `remote`: defaults to `origin`

If `mode` is missing, default to `dry-run`.

## Safety Rules

- Never delete `main`.
- Always switch to `main` before deletion.
- Always sync main before deletion:
- `git fetch <remote> main`
- `git checkout main`
- `git pull --ff-only <remote> main`
- No grouped branch deletion commands.
- Delete branches one-by-one only.
- In `dry-run`, do not delete anything.

## Command Anchors (JSON Required)

- Capture machine-readable diagnostics before destructive actions:
  - `gait doctor --json`
  - `gait gate eval --policy <policy.yaml> --intent <intent.json> --json`

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
- Exclude symbolic refs (`<remote>/HEAD`)

3. Dry-run report (always):
- print local branches that would be deleted
- print remote branches that would be deleted
- print counts for local/remote

4. Execute deletion only if `mode=execute`:
- Local deletion one-by-one:
- try `git branch -d <branch>`
- if fails and `force_local_delete=true`, retry `git branch -D <branch>`
- otherwise record failure and continue
- Remote deletion one-by-one:
- `git push <remote> --delete <branch>`
- if already deleted/not found, record as skipped and continue

5. Final reconciliation:
- `git fetch --prune <remote>`
- list remaining local branches
- list remaining `<remote>/*` branches

6. Output summary:
- deleted local branches
- deleted remote branches
- skipped/failed branches with reasons
- final remaining branch lists

## Command Discipline

Use one-by-one commands only, such as:

- `git branch -d <name>`
- `git branch -D <name>` (only if `force_local_delete=true`)
- `git push <remote> --delete <name>`

Do not use batched deletion forms.

## Failure Handling

- Continue on per-branch failures; do not abort whole run.
- Record each failure with command + stderr reason.
- If `<remote>/main` is missing, or if cannot switch/sync `main`, stop immediately.

## Expected Output

- `Mode`: dry-run or execute
- `Local candidates`: list + count
- `Remote candidates`: list + count
- `Deleted local`: list
- `Deleted remote`: list
- `Skipped/failed`: list with reasons
- `Remaining local`: list
- `Remaining remote`: list
