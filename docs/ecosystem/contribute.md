# Ecosystem Contribution Funnel

Use this path to propose a community adapter or skill and get it listed.

## 1) Open the Right Proposal Issue

- Adapter proposal: `.github/ISSUE_TEMPLATE/adapter.yml`
- Skill proposal: `.github/ISSUE_TEMPLATE/skill.yml`

Required in the issue:

- Problem solved and target framework/tooling
- Minimal runnable example path
- Safety semantics (`allow` vs non-`allow`) and fail-closed behavior
- Test plan and deterministic artifact outputs

## 2) Add Or Update Index Entry

Edit `docs/ecosystem/community_index.json`:

- add one entry with a unique `id`
- point `repo` to a public GitHub URL
- set `source` to `community` unless maintained in this repository
- set initial `status` to `experimental` for new community submissions

Validate locally:

```bash
python3 scripts/validate_community_index.py
```

## 3) Prove Contract Compatibility

Adapter submissions should provide:

- an execution path that evaluates with `gait gate eval`
- deterministic trace/run output paths
- fail-closed behavior on non-`allow` verdicts

Validation commands:

```bash
make lint
make test-adoption
make test-adapter-parity
```

Skill submissions should provide:

- provenance metadata (`source`, `publisher`, digest/signature where applicable)
- no embedded policy logic outside Go core

Validation commands:

```bash
make lint
make test-skill-supply-chain
```

## 4) Review And Listing

A contribution is listed in `docs/ecosystem/awesome.md` once:

- index entry validates in CI
- reviewer confirms deterministic/no-bypass behavior
- documentation includes install and quickstart commands

## 5) Release Automation Artifact

Before a tagged release, generate the ecosystem release summary from the index:

```bash
python3 scripts/render_ecosystem_release_notes.py
```

This produces a deterministic markdown summary under `gait-out/` for release notes and launch distribution.
