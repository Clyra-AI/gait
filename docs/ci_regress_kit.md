# CI Regress Kit (v2.3 Blessed CI Lane)

This kit makes incident-to-regression checks turnkey in CI.

Primary drop-in path:

- `.github/actions/gait-regress/action.yml` (composite action, v2 contract)

Canonical default path:

- `.github/workflows/adoption-regress-template.yml`

## Reusable Workflow Contract

The workflow supports both:

- `workflow_dispatch`
- `workflow_call`

### Inputs

- `fixture_runpack_path` (default: `fixtures/run_demo/runpack.zip`)
- `config_path` (default: `gait.yaml`)
- `source_run` (default: `run_demo`, used for deterministic fallback fixture generation)

### Outputs (workflow_call)

- `regress_status`
- `regress_exit_code`
- `top_failure_reason`
- `next_command`
- `artifact_root`

### Deterministic Artifact Root

- `gait-out/adoption_regress/`
  - `regress_result.json`
  - `junit.xml`
  - `regress_init_result.json` (when fallback init is used)

The workflow also uploads fixture/config artifacts for triage.

## Summary-First Failure Triage

Job summary publishes:

- status
- exit code
- top failure reason
- next command
- artifact root and artifact paths

You can start triage from summary without opening raw logs first.

Stable regress exit semantics are preserved:

- `0`: pass
- `5`: deterministic regression failure
- other: unexpected error passthrough

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

## Composite Action (Step-Level Reuse)

Use the composite action when you want step-level control inside an existing job:

```yaml
- uses: ./.github/actions/gait-regress
  with:
    version: latest
    workdir: .
    command: regress
    args: "--config gait.yaml"
    upload_artifacts: true
    artifact_name: gait-regress-artifacts
```

Supported `command` values:

- `regress`: runs `gait regress run --json --junit ...`
- `policy-test`: runs `gait policy test ... --json`

Action outputs:

- `exit_code`
- `summary_path`
- `artifact_path`

The action downloads release binaries, verifies `checksums.txt`, prints a bounded summary (including ticket footer when available), uploads `./gait-ci` artifacts, and fails the job on non-zero exit code.

Reference examples:

- `.github/actions/gait-regress/README.md`
- `examples/ci/gait-regress-failing/workflow.yml`

## Compatibility Shell Snippet (Non-GitHub CI)

Use this for Jenkins/Buildkite/CircleCI style runners:

```bash
set -euo pipefail

go build -o ./gait ./cmd/gait
mkdir -p gait-out/adoption_regress

if [[ ! -f fixtures/run_demo/runpack.zip || ! -f gait.yaml ]]; then
  ./gait demo
  ./gait regress init --from run_demo --json > gait-out/adoption_regress/regress_init_result.json
fi

set +e
./gait regress run --json --junit=./gait-out/adoption_regress/junit.xml > ./gait-out/adoption_regress/regress_result.json
status=$?
set -e

if [[ "$status" -eq 0 ]]; then
  echo "regress pass"
elif [[ "$status" -eq 5 ]]; then
  echo "regress fail (stable exit code 5)"
  exit 5
else
  echo "unexpected regress exit code: $status"
  exit "$status"
fi

./gait policy test examples/policy/endpoint/allow_safe_endpoints.yaml examples/policy/endpoint/fixtures/intent_allow.json --json
set +e
./gait policy test examples/policy/endpoint/block_denied_endpoints.yaml examples/policy/endpoint/fixtures/intent_block.json --json
block_status=$?
./gait policy test examples/policy/endpoint/require_approval_destructive.yaml examples/policy/endpoint/fixtures/intent_destructive.json --json
approval_status=$?
set -e
if [[ "$block_status" -ne 3 ]]; then
  echo "endpoint block fixture exit mismatch: $block_status"
  exit 1
fi
if [[ "$approval_status" -ne 4 ]]; then
  echo "endpoint approval fixture exit mismatch: $approval_status"
  exit 1
fi

bash scripts/test_skill_supply_chain.sh
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

This keeps adoption-critical changes gated while avoiding unnecessary runs on unrelated docs-only edits.

Downstream simulation test command:

```bash
bash scripts/test_ci_regress_template.sh
```
