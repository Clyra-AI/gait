---
name: meta-skill-author
description: Create or update factory-style skills, templates, or worker instructions from the repo standards. Use when the team needs a new reusable worker definition and wants it to follow the factory contract format.
disable-model-invocation: true
---

# Meta Skill Author

This is a local discovery wrapper for the shared Factory skill at `factory/skills/meta-skill-author/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/meta-skill-author/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/meta-skill-author/SKILL.md` and follow that Factory skill exactly, using the active `gait` repo profile unless the user provides another explicit profile.

Do not treat this wrapper as the source of truth. The Factory skill is authoritative.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.

