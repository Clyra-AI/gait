---
name: factory-sync
description: Update the local Factory submodule pointer to latest factory/main through a small PR before running Factory-backed skills.
disable-model-invocation: true
---

# Factory Sync

Update this repository's `factory` submodule to the latest `factory/main` commit
through an auditable PR.

Use this skill before a Factory-backed wrapper skill when the user wants the
latest shared skill behavior.

## Workflow

1. Verify the current repo has a clean worktree.
2. Sync the default branch:
   - `git fetch origin main`
   - `git checkout main`
   - `git pull --ff-only origin main`
3. Create or switch to `codex/update-factory-submodule`.
4. Update Factory:
   - `git submodule update --init factory`
   - `git submodule update --remote factory`
5. If `git status --short factory .gitmodules` is empty:
   - report `factory already at latest main`
   - return the current `git -C factory rev-parse HEAD`
   - do not create a PR
6. If the submodule pointer changed:
   - ensure only `factory` and optionally `.gitmodules` changed
   - `git add factory .gitmodules`
   - `git commit -m "chore: update factory submodule"`
   - `git push -u origin codex/update-factory-submodule`
   - create a PR against `main`
   - wait for required checks
   - merge without deleting the branch
7. Sync local `main` after merge and return the merged Factory commit.

## Safety Rules

- Do not edit files inside `factory`.
- Do not run product implementation work in this skill.
- Do not hide unrelated dirty files; stop and report them.
- Do not force-push unless explicitly required by a stale branch and clearly reported.
- Use machine-readable command output when useful, for example `wrkr scan --json`,
  `axym collect --dry-run --json`, or `gait doctor --json` depending on the active repo.

## Output

- synced Factory commit SHA
- PR URL or `already up to date`
- merge SHA when a PR was merged
- next suggested command, such as `Use $adhoc-plan ...`
