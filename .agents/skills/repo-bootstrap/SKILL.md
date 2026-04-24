---
name: repo-bootstrap
description: Bootstrap a new repository from a PRD plus org-wide standards and templates. Use when a project is new, the main structured input is a PRD, and the goal is to generate the repo operating pack and first execution plan.
disable-model-invocation: true
---

# Repo Bootstrap

This is a local discovery wrapper for the shared Factory skill at `factory/skills/repo-bootstrap/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/repo-bootstrap/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/repo-bootstrap/SKILL.md` and follow that Factory skill exactly, using the active `gait` repo profile unless the user provides another explicit profile.

Do not treat this wrapper as the source of truth. The Factory skill is authoritative.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.

