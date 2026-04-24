---
name: library-sync
description: Distribute approved cataloged assets to allowed environments and record the result. Use when the org library needs to sync pinned skills, templates, standards, or schemas across devices and sandboxes.
disable-model-invocation: true
---

# Library Sync

This is a local discovery wrapper for the shared Factory skill at `factory/skills/library-sync/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/library-sync/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/library-sync/SKILL.md` and follow that Factory skill exactly, using the active `gait` repo profile unless the user provides another explicit profile.

Do not treat this wrapper as the source of truth. The Factory skill is authoritative.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.

