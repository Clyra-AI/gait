---
name: cut-release
description: Resolve changelog-derived semver, finalize the release changelog through Factory scripts, tag from the default branch, monitor release/UAT, and run bounded hotfix loops when failures are actionable.
disable-model-invocation: true
---

# Cut Release

This is a local discovery wrapper for the shared Factory skill at `factory/skills/cut-release/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/cut-release/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/cut-release/SKILL.md` and follow that Factory skill exactly, using the active `gait` repo profile unless the user provides another explicit profile.

Do not treat this wrapper as the source of truth. The Factory skill is authoritative.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.

