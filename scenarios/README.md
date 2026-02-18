# Tier 11 Scenario Fixtures

Scenario fixtures are human-reviewed behavior specifications for outside-in validation.
All fixtures are offline-first and deterministic.

## Running

- Validate fixture structure and syntax: `bash scripts/validate_scenarios.sh`
- Run all gait scenarios: `bash scripts/run_scenarios.sh gait`
- Run Go scenario harness directly: `go test ./internal/scenarios -count=1 -tags=scenario -v`

## Authorship Rules

- Changes under `scenarios/` are specification changes and require CODEOWNERS review.
- Expected outcome files (`expected.yaml`, `expected-verdicts.jsonl`) define contract behavior.
- Fixtures must be self-contained and must not require network access.

## Scenario Set (gait)

1. `policy-block-destructive`
2. `policy-allow-safe-tools`
3. `dry-run-no-side-effects`
4. `concurrent-evaluation-10`
5. `pack-integrity-round-trip`
6. `delegation-chain-depth-3`
7. `approval-expiry-1s-past`
8. `approval-token-valid`
