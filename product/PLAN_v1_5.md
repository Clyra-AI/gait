# PLAN v1.5: Gait Skills (Execution Plan + Delivered Targets)

Date: 2026-02-06  
Source: `product/ROADMAP.md` (`v1.5 "Gait Skills"`)

## Objective

Ship an installable skill set that teaches agents to use Gait safely and deterministically without adding product logic outside the CLI.

## Design Constraints

1. Skills call `gait` and parse `--json`.
2. Skills embed no credentials or standing permissions.
3. Skills default to safe and deterministic paths.
4. Skills depend on stable schemas and exit code contracts.

## File-by-File Targets

### Skills (installable artifacts)

- `.agents/skills/gait-capture-runpack/SKILL.md`
- `.agents/skills/gait-capture-runpack/agents/openai.yaml`
- `.agents/skills/gait-incident-to-regression/SKILL.md`
- `.agents/skills/gait-incident-to-regression/agents/openai.yaml`
- `.agents/skills/gait-policy-test-rollout/SKILL.md`
- `.agents/skills/gait-policy-test-rollout/agents/openai.yaml`

### Tooling and integration

- `scripts/install_repo_skills.sh`
- `scripts/validate_repo_skills.py`
- `Makefile` (`lint` and `skills-validate`)
- `.github/workflows/ci.yml` (`lint` job skills validation)

### Documentation

- `README.md` (`4.7) Gait Skills (v1.5)`)

## Acceptance Criteria

1. Exactly three skills are shipped and installable.
2. Each skill contains valid frontmatter and `agents/openai.yaml` metadata.
3. Skill guidance enforces `--json`, deterministic defaults, and no credential embedding.
4. Repo lint and CI fail if skills metadata/shape regress.
5. README documents installation and included skills.

## Execution Status

- Implemented and integrated in this batch.
