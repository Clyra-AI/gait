# Resource Budgets (H11)

This document defines minimum resource and latency budgets for critical commands and the benchmark checks that monitor drift.

## Runtime SLO Budgets (p50/p95/p99)

Runtime command SLO budgets are defined in:

- `perf/runtime_slo_budgets.json`

They are enforced by:

- `scripts/check_command_budgets.py`
- `make bench-budgets`

Gate runtime SLO coverage includes endpoint classes:

- `fs.read`
- `fs.write`
- `fs.delete`
- `proc.exec`
- `net.http`
- `net.dns`

Each command budget enforces:

- p50 latency
- p95 latency
- p99 latency
- max error rate

## Core Benchmark Budgets

Benchmark budgets are defined in `perf/resource_budgets.json` and checked by `scripts/check_resource_budgets.py` against nightly benchmark output (`perf/bench_output.txt`).

Covered benchmark families:

- `BenchmarkEvaluatePolicyTypical`
- `BenchmarkVerifyZipTypical`
- `BenchmarkDiffRunpacksTypical`
- `BenchmarkSnapshotTypical`
- `BenchmarkDiffSnapshotsTypical`
- `BenchmarkVerifyPackTypical`
- `BenchmarkBuildIncidentPackTypical`
- `BenchmarkInstallLocalTypical`
- `BenchmarkVerifyInstalledTypical`
- `BenchmarkDecodeToolCallOpenAITypical`
- `BenchmarkEvaluateToolCallTypical`

## Budget Policy

- Budgets are intentionally conservative to reduce false positives in shared CI environments.
- `BenchmarkInstallLocalTypical` budget reflects full local install integrity work (JCS digest, signature verification, and atomic fsync writes), not a metadata-only path.
- Absolute `max_ns_op` ceilings should stay rounded above repeated release-host medians and close to the baseline regression envelope tracked in `perf/bench_baseline.json`.
- When a benchmark repeatedly lands within a few percent of the regression ceiling on supported release hosts, refresh the baseline or factor in the same change that established the new steady state.
- Local UAT may use `perf/resource_budgets_uat.json` via `make bench-uat-check` to validate absolute release-host perf ceilings without re-running the stricter baseline-regression gate that `make bench-check` enforces earlier in release validation.
- Tightening a budget requires:
  1. Baseline refresh in `perf/bench_baseline.json`
  2. Updated budget rationale in this document
  3. Successful nightly run proving stability
- Repeated budget violations require a tracked issue and owner before release.
