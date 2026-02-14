# Test Cadence Guide

This guide defines speed-vs-depth execution for contributors and maintainers.

## Pull Request Cadence (Fast, Required)

Target outcome: quick deterministic feedback with low cycle time.

Run locally before opening PR:

- `make prepush` (`make lint-fast` + `make test-fast`)

Required PR checks on `main`:

- `pr-fast-lint`
- `pr-fast-test`
- `codeql-scan`

Optional local full gate before large/risky PRs:

- `GAIT_PREPUSH_MODE=full git push` (runs `make prepush-full`)

## Mainline Cadence (Broad And Deep)

Heavy validation runs on `main` and merge queue via `.github/workflows/ci.yml`.

Core suites include:

- `make lint`
- `make test`
- `make test-e2e`
- acceptance wedges (`make test-acceptance`, `make test-v1-6-acceptance`, `make test-v1-7-acceptance`, `make test-v1-8-acceptance`, `make test-v2-3-acceptance`, `make test-v2-4-acceptance`)
- PackSpec contract lane (`make test-packspec-tck`)
- adoption and adapter parity suites
- hardening acceptance and contract checks
- release/install smoke paths

## Nightly Cadence (Stability + Drift)

Nightly workflows cover slower/systemic checks:

- `adoption-nightly.yml`
- `hardening-nightly.yml`
- `perf-nightly.yml`
- `windows-lint-nightly.yml`

Nightly objective:

- detect long-horizon drift and performance regressions
- validate less frequent platform/runtime edges
- create follow-up issues before release cut

## Enforcement Policy

- `main` is PR-only with required checks.
- PRs must have required checks green before merge.
- Nightly regressions must be triaged before next release.
- Performance budget failures require either a fix or an explicit reviewed baseline update.
