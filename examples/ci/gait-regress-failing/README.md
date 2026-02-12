# Failing `gait-regress` CI Example

This example intentionally creates a failing regress run so the workflow uploads deterministic diff artifacts.

## What it demonstrates

- `gait-regress` action in `regress` mode
- deterministic failure (`exit_code=5`) from fixture drift
- uploaded `gait-ci` artifact bundle for triage

## Files

- `examples/ci/gait-regress-failing/workflow.yml`: copy into `.github/workflows/gait-regress-failing.yml`

## Expected result

The workflow fails at the regress step with stable exit code `5` and uploads artifact folder `gait-regress-failing-artifacts`.
