# Test Cadence Guide (Epic A5.2)

This guide defines speed-vs-depth test execution for Gait adopters.

## PR Cadence (Fast, Deterministic)

Run on every pull request:

- `make lint`
- `make test`
- `make test-adoption` for onboarding and integration-path smoke checks
- `bash scripts/policy_compliance_ci.sh` for canonical policy fixtures and reason-code summaries
- `gait regress run --json --junit=...` when regress fixtures are in scope

PR objective:

- Catch correctness, security, and schema regressions quickly.
- Keep cycle time low enough for frequent iteration.

## Nightly Cadence (Broad And Deep)

Run nightly:

- `make lint`
- `make test`
- `make test-e2e`
- `make test-acceptance`
- `make bench-check`

Nightly objective:

- Exercise slower integration/e2e paths.
- Detect performance drift and environmental issues outside PR runtime budgets.

## Reference Workflow

Use `.github/workflows/adoption-nightly.yml` as a sample nightly profile.

## Minimum Enforcement Policy

- PR merges require green deterministic checks.
- Nightly failures create follow-up issues before next release cut.
- Performance regression failures require either:
  - benchmark baseline update with reviewer sign-off, or
  - code fix restoring baseline.
