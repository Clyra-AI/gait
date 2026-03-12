# CI Portability Templates

GitHub Actions is the primary lane through `.github/workflows/adoption-regress-template.yml`.

These templates wrap the same CI contract script:

- `scripts/ci_regress_contract.sh`

Provider templates:

- GitLab: `examples/ci/portability/gitlab/.gitlab-ci.yml`
- Jenkins: `examples/ci/portability/jenkins/Jenkinsfile`
- CircleCI: `examples/ci/portability/circleci/config.yml`

All templates preserve:

- stable regress exit handling (`0`, `5`, passthrough)
- deterministic artifacts under `gait-out/adoption_regress/`
- deterministic fixture fallback init from `run_demo`
- parity with the explicit `gait capture` + `gait regress add` handoff flow
