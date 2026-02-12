# `gait-regress` GitHub Action

Run deterministic Gait checks in CI with a verified release binary.

## Inputs

- `version` (default: `latest`): release tag (for example `v1.0.7`) or `latest`
- `workdir` (default: `.`): directory where command executes
- `command` (default: `regress`): `regress` or `policy-test`
- `args` (default: empty): extra args passed to the selected command
- `upload_artifacts` (default: `true`): upload `./gait-ci` directory
- `artifact_name` (default: `gait-artifacts`): workflow artifact name

## Outputs

- `exit_code`: exit code from the executed command
- `summary_path`: bounded summary text file path
- `artifact_path`: artifact directory path

## Example: Regress In PRs

```yaml
name: gait-regress

on:
  pull_request:

permissions:
  contents: read

jobs:
  regress:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Initialize deterministic fixture
        run: |
          go build -o ./gait ./cmd/gait
          ./gait demo --json >/dev/null
          ./gait regress init --from run_demo --json >/dev/null

      - name: Run gait-regress action
        id: gait
        uses: ./.github/actions/gait-regress
        with:
          version: latest
          command: regress
          workdir: .
          upload_artifacts: true
          artifact_name: gait-regress-artifacts

      - name: Print outputs
        run: |
          echo "exit_code=${{ steps.gait.outputs.exit_code }}"
          echo "summary_path=${{ steps.gait.outputs.summary_path }}"
          echo "artifact_path=${{ steps.gait.outputs.artifact_path }}"
```

## Example: Policy Fixture Test

```yaml
name: gait-policy-test

on:
  pull_request:

permissions:
  contents: read

jobs:
  policy-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run gait-regress action in policy-test mode
        id: gait
        uses: ./.github/actions/gait-regress
        with:
          command: policy-test
          args: >-
            examples/policy/endpoint/allow_safe_endpoints.yaml
            examples/policy/endpoint/fixtures/intent_allow.json
          upload_artifacts: true
          artifact_name: gait-policy-test-artifacts

      - name: Print outputs
        run: |
          echo "exit_code=${{ steps.gait.outputs.exit_code }}"
          cat "${{ steps.gait.outputs.summary_path }}"
```
