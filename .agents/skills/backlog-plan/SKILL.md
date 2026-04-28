---
name: backlog-plan
description: Convert repo-owned strategic ideas into a timestamped, execution-ready backlog plan using the active repo profile.
disable-model-invocation: true
---

# Backlog Plan

This is a local discovery wrapper for the shared Factory skill at `factory/skills/backlog-plan/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/backlog-plan/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/backlog-plan/SKILL.md` and follow that Factory skill using the active `gait` repo profile unless the user provides another explicit profile.

Project wrapper policy:

- Write generated backlog plans under profile `backlog_plan.output_dir`, currently `product/plans/backlog`.
- Do not write backlog plans to `product/plans/adhoc`; that directory is reserved for one-off adhoc plans.
- Create the output directory when it is missing, then write only the timestamped backlog plan unless the user explicitly requests another file.

Do not treat this wrapper as the source of truth for plan content. The Factory skill is authoritative for the generated plan structure.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.
