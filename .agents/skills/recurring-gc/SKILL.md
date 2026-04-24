---
name: recurring-gc
description: Run scheduled cleanup and drift-control work across repo contracts, workspaces, branches, examples, schemas, and expired exceptions. Use for recurring maintenance and hygiene automation.
disable-model-invocation: true
---

# Recurring GC

This is a local discovery wrapper for the shared Factory skill at `factory/skills/recurring-gc/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/recurring-gc/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/recurring-gc/SKILL.md` and follow that Factory skill exactly, using the active `gait` repo profile unless the user provides another explicit profile.

Do not treat this wrapper as the source of truth. The Factory skill is authoritative.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.

