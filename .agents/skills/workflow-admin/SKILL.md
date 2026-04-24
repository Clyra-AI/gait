---
name: workflow-admin
description: Maintain and debug the autonomous runtime contract and orchestration settings. Use when a repository needs a new or updated WORKFLOW.md, approval changes, trust-mode changes, model-routing changes, grading changes, workspace hook changes, or operational troubleshooting.
disable-model-invocation: true
---

# Workflow Admin

This is a local discovery wrapper for the shared Factory skill at `factory/skills/workflow-admin/SKILL.md`.

Before using this skill:

1. Verify `factory/skills/workflow-admin/SKILL.md` exists.
2. If it is missing, stop and ask the user to run:

```bash
git submodule update --init factory
```

Then read `factory/skills/workflow-admin/SKILL.md` and follow that Factory skill exactly, using the active `gait` repo profile unless the user provides another explicit profile.

Do not treat this wrapper as the source of truth. The Factory skill is authoritative.

Gait repository skill contract:

- When command evidence is needed, prefer `gait doctor --json` or another active-profile `gait ... --json` command from `factory/profiles/gait.yaml`.
- Require `--json` output for machine-readable command evidence.

