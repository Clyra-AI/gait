# TESTS_GAPS Fix Plan

Date: 2026-02-14  
Owner: Engineering

## Scope

This plan closes test-matrix and coverage gaps found during repo audit, plus checks and addresses failures in GitHub Actions run:

- [Run 22018642257](https://github.com/davidahmann/gait/actions/runs/22018642257)

## Gaps and Actions

| Gap | Evidence | Action | Status |
|---|---|---|---|
| Release gate lagging v2.4 contract | `product/PLAN_2.4.md` release gates vs `release.yml` only gating v2.3 | Add a `v2_4_gate` release workflow job that runs v2.4 acceptance + TCK + e2e + integration + chaos + perf budgets. Make release depend on both v2.3 and v2.4 gates. | Implemented |
| UI acceptance not part of CI/UAT quality gate | `test-ui-acceptance` exists but was not wired into CI and UAT orchestrator | Add dedicated CI `ui-acceptance` job. Add `quality_ui_acceptance` step to `scripts/test_uat_local.sh`. Update UAT plan docs. | Implemented |
| Package-level Go coverage floor not enforced | Existing checks only enforced aggregate coverage | Add package coverage checker script and enforce >=75% package coverage in `make test` and CI test lanes. | Implemented |
| Failing CI run due missing tracked PackSpec fixture | Run `22018642257` failed in `v2-4-acceptance` and `packspec-tck` with missing `fixtures/packspec_tck/v1/run_record_input.json` | Move PackSpec fixture to tracked `scripts/testdata/packspec_tck/v1/run_record_input.json` and update scripts/docs to use it. | Implemented |
| Low package coverage (`core/errors`, `internal/testutil`) | Coverage audit showed both under 75% | Add focused unit tests in both packages to lift package coverage above floor. | Implemented |
| Docs-site route/link rendering checks beyond lint/build | Plan asks for deeper link/render checks | Added `scripts/check_docs_site_validation.py`, CI docs validation step + artifact, and UAT `docs-site-check` wiring. | Implemented |
| UI frontend unit/e2e and UI performance budget lane | `PLAN_UI.md` requests frontend unit/e2e/perf budget lanes | Added Vitest-based frontend unit tests, `scripts/test_ui_e2e_smoke.sh` CI lane, and UI budget enforcement (`perf/ui_budgets.json`, `scripts/check_ui_budgets.py`, nightly workflow + perf-nightly integration). | Implemented |
| Runtime perf coverage missing job/pack lifecycle commands | Runtime SLO budgets previously focused on demo/regress/gate/session | Extended command budget matrix and runtime budgets with `job_submit`, `job_checkpoint_add`, `job_approve`, `job_resume`, `pack_build_job`, and `pack_verify_job`. | Implemented |

## Verification Commands

Run locally from repo root:

```bash
make lint-fast
make test
make test-e2e
go test ./internal/integration -count=1
make test-v2-4-acceptance
make test-packspec-tck
make test-ui-acceptance
make test-ui-unit
make test-ui-e2e-smoke
make test-ui-perf
make docs-site-check
make test-chaos
make test-runtime-slo
make bench-check
make codeql
```

## Exit Criteria

- All commands above pass locally.
- PR CI lanes pass, including new `ui-acceptance` and updated coverage/release gates.
- No regressions against v2.4 acceptance and PackSpec TCK.
