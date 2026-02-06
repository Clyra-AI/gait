# Resource Budgets (H11)

This document defines minimum resource and latency budgets for critical commands and the benchmark checks that monitor drift.

## Command Budgets (Median, local baseline profile)

| Command | Budget (median) | Notes |
| --- | --- | --- |
| `gait verify <run_id> --json` | <= 1200 ms | Demo-sized runpack verification path. |
| `gait gate eval --policy <p> --intent <i> --json` | <= 1200 ms | Single-intent policy evaluation including trace emit. |
| `gait regress run --json` | <= 3000 ms | Default fixture set from `run_demo` via `regress init`. |
| `gait guard pack --run <run_id> --json` | <= 3000 ms | Default evidence pack generation path. |

Command budgets are evaluated in nightly performance workflow with `scripts/check_command_budgets.py`.

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
- Tightening a budget requires:
  1. Baseline refresh in `perf/bench_baseline.json`
  2. Updated budget rationale in this document
  3. Successful nightly run proving stability
- Repeated budget violations require a tracked issue and owner before release.
