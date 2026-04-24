---
name: library-admin
description: Maintain the org-level library manifest and cookbook. Use when catalog entries, pinning, trust classes, compatibility scope, install profiles, adapter mappings, or writeback policy need to change.
disable-model-invocation: true
---

# Library Admin

This is a local discovery wrapper for the shared Factory skill at `factory/skills/library-admin/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/library-admin/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/library-admin/SKILL.md` and follow that Factory skill exactly, using the active `gait` repo profile unless the user provides another explicit profile.

Do not treat this wrapper as the source of truth. The Factory skill is authoritative.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.

