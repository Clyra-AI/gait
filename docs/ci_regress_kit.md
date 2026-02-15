---
title: "CI Regress Kit"
description: "Wire Gait regression fixtures into GitHub Actions, GitLab CI, Jenkins, or CircleCI with JUnit output and stable exit codes."
---

# CI Regress Kit

This kit keeps CI adoption "one PR to adopt" while preserving deterministic regress contracts.

Version note: this page is evergreen. Release-specific packaging rollout notes belong in `docs/PLAN_v2.7_distribution.md` and release notes.

## One-PR Adoption Path (Default)

GitHub Actions is the primary lane:

- reusable workflow: `.github/workflows/adoption-regress-template.yml`
- step-level action: `.github/actions/gait-regress/action.yml`

Minimal adoption action:

1. Add the workflow to your repo.
2. Ensure fixture/config inputs are present (or keep deterministic fallback init enabled).
3. Open a PR and verify artifacts under `gait-out/adoption_regress/`.

## Reusable Workflow Contract

The workflow supports:

- `workflow_dispatch`
- `workflow_call`

Inputs:

- `fixture_runpack_path` (default: `fixtures/run_demo/runpack.zip`)
- `config_path` (default: `gait.yaml`)
- `source_run` (default: `run_demo`, deterministic fallback fixture source)

Outputs:

- `regress_status`
- `regress_exit_code`
- `top_failure_reason`
- `next_command`
- `artifact_root`

Stable exit semantics:

- `0`: pass
- `5`: deterministic regression failure
- other: unexpected error passthrough

Deterministic artifact root:

- `gait-out/adoption_regress/`
  - `regress_result.json`
  - `junit.xml`
  - `regress_init_result.json` (only when fallback init runs)
  - `pack_verify_fixture.json` (fixture integrity evidence)

## Composite Action Contract

Use the action for step-level reuse:

```yaml
- uses: ./.github/actions/gait-regress
  with:
    version: latest
    command: regress
    workdir: .
    upload_artifacts: true
    artifact_name: gait-regress-artifacts
```

Supported `command` values:

- `regress`
- `policy-test`

Action outputs:

- `exit_code`
- `summary_path`
- `artifact_path`

## CI Portability Contract (GitLab/Jenkins/Circle)

Non-GitHub CI providers should call one compatibility contract script:

- `scripts/ci_regress_contract.sh`

Portable templates:

- GitLab: `examples/ci/portability/gitlab/.gitlab-ci.yml`
- Jenkins: `examples/ci/portability/jenkins/Jenkinsfile`
- CircleCI: `examples/ci/portability/circleci/config.yml`

Local parity check:

```bash
bash scripts/ci_regress_contract.sh
```

The portability templates are wrappers around this script and must preserve:

- identical regress exit handling (`0`, `5`, passthrough)
- identical artifact root (`gait-out/adoption_regress/`)
- identical fixture fallback behavior (`run_demo` init path)

## Downstream Reuse Example

```yaml
name: downstream-regress

on:
  pull_request:

jobs:
  regress:
    uses: ./.github/workflows/adoption-regress-template.yml
    with:
      fixture_runpack_path: fixtures/run_demo/runpack.zip
      config_path: gait.yaml
      source_run: run_demo
```

## Path-Filtered PR Guidance

Require this lane for changes touching:

- `cmd/gait/**`
- `core/runpack/**`
- `core/regress/**`
- `core/gate/**`
- `schemas/**`
- `docs/integration_checklist.md`
- `docs/ci_regress_kit.md`
- `.agents/skills/**`

Validation commands:

```bash
bash scripts/test_ci_regress_template.sh
bash scripts/test_ci_portability_templates.sh
```

## Frequently Asked Questions

### How do I add Gait regression to CI?

Copy the template workflow (`.github/workflows/adoption-regress-template.yml`) or use the drop-in action (`.github/actions/gait-regress/`). Both produce JUnit output.

### What does exit code 5 mean?

Exit 5 means regression failed — the current run drifted from the baseline fixture. Exit 0 means pass.

### Can I use Gait with GitLab CI?

Yes. The CI regress kit provides portable templates for GitLab, Jenkins, and CircleCI that use the same CLI exit and artifact contract.

### How do I create a regression fixture?

Run `gait regress bootstrap --from <run_id> --junit <path>`. This creates a fixture directory and runs the first regression pass in one command.

### Does regression testing require network access?

No. Regression testing is fully offline. It compares the current run against the baseline fixture using deterministic stubs.
