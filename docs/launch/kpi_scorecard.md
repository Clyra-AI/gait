# Launch KPI Scorecard

Track these metrics for every launch cycle (day 1, day 7, day 30).

## Distribution

- Star velocity (24h, 7d)
- Unique repo visitors (24h, 7d)
- HN thread engagement (comments, rank window, clicks)

## Activation

- Install success rate (`install.sh` completion / attempts)
- Median time from install to first `gait demo` success
- Median time from install to first `gait regress bootstrap`

## Product Use

- Weekly runs with `runpack` + `verify`
- Weekly repositories with at least one regress fixture
- Weekly repositories with gate policy checks enabled

## Ecosystem Pull

- New adapter/skill proposals opened (`adapter.yml`, `skill.yml`)
- Net new entries in `docs/ecosystem/community_index.json`
- Adapter parity pass rate for official integrations

## Launch Gates

Healthy launch baseline:

- Install success >= 90%
- Median install-to-demo <= 5 minutes
- Median install-to-regress <= 15 minutes
- At least 3 qualified ecosystem proposals in first 30 days

If below baseline:

- Prioritize onboarding fixes (`README`, `docs/install.md`, `scripts/install.sh`)
- tighten first-win docs before adding new surface area
