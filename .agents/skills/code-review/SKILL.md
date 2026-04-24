---
name: code-review
description: Perform a profile-driven full-repository or scoped code review with severity-ranked findings focused on contracts, boundaries, fail-closed safety, determinism, portability, release integrity, and docs correctness.
disable-model-invocation: true
---

# Code Review

This is a local discovery wrapper for the shared Factory skill at `factory/skills/code-review/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/code-review/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/code-review/SKILL.md` and follow that Factory skill exactly, using the active `gait` repo profile unless the user provides another explicit profile.

Do not treat this wrapper as the source of truth. The Factory skill is authoritative.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.

