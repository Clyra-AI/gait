---
name: cut-release
description: Cut a new Gait release tag directly from main, monitor release/post-release validation, and run up to 2 hotfix PR loops when failures are actionable.
disable-model-invocation: true
---

# Cut Release (Gait)

Execute this workflow for: "cut release", "ship vX.Y.Z", "push tag and monitor release."

## Scope

- Repository: `/Users/davidahmann/Projects/gait`
- Tag source branch: `main` only
- No pre-release branch creation
- No pre-release PR creation
- Branch/PR flow is used only for hotfixes after failed checks
- No changelog editing in this skill

## Input Contract

- Mandatory input argument: `release_version`
- Normalize to `vX.Y.Z`
- If missing, resolve silently from first semantic version token in user request; otherwise use latest tag + patch increment.

## Constants

- `MAX_HOTFIX_LOOPS=2`
- `CI_TIMEOUT_MIN=25`
- `RELEASE_TIMEOUT_MIN=40`
- `POLL_SECONDS=10`

## Safety Rules

- Tag must always be created and pushed from `main`
- `main` must be fast-forward synced with `origin/main` before each tag push
- No force-push to tags
- No destructive git commands
- No commit amend unless explicitly requested
- No changelog modifications
- PR bodies/comments must use EOF heredoc (`--body-file - <<'EOF' ... EOF`)

## Workflow

### Phase 0: Main Sync and Pre-Tag Validation

1. `git fetch origin main`
2. `git checkout main`
3. `git pull --ff-only origin main`
4. Ensure clean worktree (`git status --porcelain` must be empty)
5. Ensure target tag does not already exist locally/remotely
6. Run local release preflight (mirror release workflow gate coverage):
- `make prepush-full`
- `make test-v2-3-acceptance`
- `make test-v2-4-acceptance`
- `make test-packspec-tck`
- `make test-e2e`
- `go test ./internal/integration -count=1`
- `make test-chaos`
- `make test-runtime-slo`
- `make bench-check`
- `make test-v2-5-acceptance`
- `make test-context-conformance`
- `make test-context-chaos`
- `make test-release-smoke`

If any step fails, stop and report blocker.

### Phase 1: Tag and Release Monitor

1. Create annotated tag on `main`:
- `git tag -a <version> -m "<version>"`
2. Push tag:
- `git push origin <version>`
3. Monitor GitHub workflow `release` for that tag until green (`RELEASE_TIMEOUT_MIN`)
4. If release run fails:
- classify failure as actionable, transient/infra, or non-actionable
- transient/infra: rerun workflow once, re-monitor
- actionable: go to hotfix loop
- non-actionable: stop with blocker report

### Phase 2: Post-Release UAT

1. Run full local UAT against released tag:
- `GAIT_UAT_RELEASE_VERSION=<version> bash scripts/test_uat_local.sh`
2. If UAT is green, release is complete.
3. If UAT fails:
- classify actionable vs non-actionable
- actionable: go to hotfix loop
- non-actionable: stop with blocker report

### Phase 3: Hotfix Loop (Only if Needed, Max 2)

For loop `r1..r2`:

1. Sync main:
- `git fetch origin main`
- `git checkout main`
- `git pull --ff-only origin main`

2. Create hotfix branch:
- `git checkout -b codex/release-hotfix-<base-version>-r<rN>`

3. Implement minimal fix for the identified failure.

4. Validate locally:
- `make prepush-full`
- rerun the failing lane locally (release lane or UAT subset as applicable)

5. Commit and push all unstaged files:
- `git add -A`
- `git commit -m "hotfix: release stabilization for <base-version> (r<rN>)"`
- `git push -u origin <hotfix-branch>`

6. Open PR using EOF body:
- `gh pr create --title "hotfix: release stabilization <base-version> (r<rN>)" --body-file - <<'EOF'`
- include: problem, root cause, fix, validation
- `EOF`

7. Monitor PR CI (`ci` and `codeql`) to green (`CI_TIMEOUT_MIN`).

8. Merge PR after green.

9. Sync and monitor post-merge main CI:
- `git checkout main`
- `git pull --ff-only origin main`
- monitor `ci` and `codeql` on `main` (`CI_TIMEOUT_MIN`)
- if post-merge main CI is red and actionable, continue same loop (counts against max)

10. Bump patch version:
- `vX.Y.Z -> vX.Y.(Z+1)`

11. Create/push new tag from `main` and monitor `release` workflow again:
- tag from `main` only
- monitor until green (`RELEASE_TIMEOUT_MIN`)

12. Rerun full UAT for the new tag:
- `GAIT_UAT_RELEASE_VERSION=<new-version> bash scripts/test_uat_local.sh`

13. Exit conditions:
- if release + UAT green: success
- if loop count exceeds 2: stop with blocker report
- if non-actionable failure appears: stop with blocker report

## Command Contract (JSON Required)

Capture release diagnostics using `gait` commands with `--json`, for example:

- `gait doctor --json`
- `gait pack verify gait-out/pack_<id>.zip --json`

## EOF Rule (Mandatory)

All PR body/comment text must be provided with heredoc EOF.  
No inline multi-line `--body` strings.

## Expected Output

- Initial requested version and final shipped version
- All tags pushed (with confirmation each was cut from `main`)
- Release workflow run URL/status per tag
- UAT result per released tag
- Hotfix branch/PR URLs and commit SHAs (if any)
- Loop count used
- Final status: success or blocker with last failing gate
