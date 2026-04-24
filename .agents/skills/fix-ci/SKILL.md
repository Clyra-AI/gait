---
name: fix-ci
description: Diagnose and repair failing CI for an in-scope repository change. Use when the triggering signal is a failing workflow, failing required check, or post-merge regression that can be fixed in-repo.
disable-model-invocation: true
---

# Fix CI

This is a local discovery wrapper for the shared Factory skill at `factory/skills/fix-ci/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/fix-ci/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/fix-ci/SKILL.md` and follow that Factory skill exactly, using the active `gait` repo profile unless the user provides another explicit profile.

Do not treat this wrapper as the source of truth. The Factory skill is authoritative.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.

