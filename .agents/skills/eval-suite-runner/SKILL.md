---
name: eval-suite-runner
description: Run continuous evaluation suites for workflow, skill, routing, or contract changes. Use for typical, edge, adversarial, and replay coverage before promoting workflow-affecting changes.
disable-model-invocation: true
---

# Eval Suite Runner

This is a local discovery wrapper for the shared Factory skill at `factory/skills/eval-suite-runner/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/eval-suite-runner/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/eval-suite-runner/SKILL.md` and follow that Factory skill exactly, using the active `gait` repo profile unless the user provides another explicit profile.

Do not treat this wrapper as the source of truth. The Factory skill is authoritative.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.

