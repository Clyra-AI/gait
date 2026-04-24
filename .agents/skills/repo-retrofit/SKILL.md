---
name: repo-retrofit
description: Retrofit an existing repository into the factory model. Use when a repository exists but lacks the repo operating pack, workflow contract, or consistent autonomous execution rules.
disable-model-invocation: true
---

# Repo Retrofit

This is a local discovery wrapper for the shared Factory skill at `factory/skills/repo-retrofit/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/repo-retrofit/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/repo-retrofit/SKILL.md` and follow that Factory skill exactly, using the active `gait` repo profile unless the user provides another explicit profile.

Do not treat this wrapper as the source of truth. The Factory skill is authoritative.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.

