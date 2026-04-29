# Gait Skill Wrappers

`.agents/skills/` contains local discovery wrappers for shared Factory development-process skills, local Factory maintenance, and Gait-specific local skills.

Shared wrapper and maintenance skills kept in this project:

- `adhoc-plan`
- `app-audit`
- `backlog-plan`
- `branches-clean`
- `code-review`
- `commit-push`
- `cut-release`
- `factory-sync`
- `plan-implement`

Project-local skills kept in this project:

- `ci-failure-triage`
- `evidence-receipt-generation`
- `gait-capture-runpack`
- `gait-incident-to-regression`
- `gait-policy-test-rollout`
- `incident-to-regression`
- `pr-comments`

The shared Factory skill at `factory/skills/<name>/SKILL.md` is authoritative for Factory-backed wrappers. `factory-sync` is a local maintenance wrapper for updating the Factory submodule pointer.

Validation:

```bash
python3 scripts/validate_repo_skills.py
```
