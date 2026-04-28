---
name: adhoc-plan
description: Convert user-provided recommendations into a timestamped, execution-ready backlog plan using the active repo profile. Use when a repo needs a one-off plan that should not overwrite PLAN_NEXT.
disable-model-invocation: true
---

# Adhoc Plan

This is a local discovery wrapper for the shared Factory skill at `factory/skills/adhoc-plan/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/adhoc-plan/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/adhoc-plan/SKILL.md` and follow that Factory skill using the active `gait` repo profile unless the user provides another explicit profile.

Project wrapper policy:

- Write generated plans under profile `plan_output_dir`, currently `product/plans/adhoc`.
- Treat the PR as plan-only: run lightweight plan validation only, and do not run full repo validation.
- Do not poll, wait for, or inspect PR CI, and do not monitor post-merge CI or run post-merge hotfix loops for plan-only adhoc-plan PRs.
- If branch protection, review, permissions, or policy blocks merge, report the blocker instead of waiting on CI.

Do not treat this wrapper as the source of truth for plan content. The Factory skill is authoritative for the generated plan structure.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.
