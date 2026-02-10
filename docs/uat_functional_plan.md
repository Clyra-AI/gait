# Local UAT + Functional Test Plan

This runbook defines how to validate Gait end-to-end on a local machine across all supported install paths.

## Goal

Prove that a user can:

- install Gait via each distribution path
- execute the core command surface successfully
- verify deterministic contracts (runpack, regress, gate, evidence, signal)
- pass existing quality gates (lint, tests, coverage, acceptance)

## Install Paths In Scope

1. Source build (`go build -o ./gait ./cmd/gait`)
2. GitHub release installer (`scripts/install.sh`)
3. Homebrew tap (`davidahmann/tap/gait`)

Windows is included in CI matrix validation, but this local script focuses on Linux/macOS hosts.

## Required Scripts

- `scripts/test_uat_local.sh` (orchestrator; this document's entrypoint)
- `scripts/test_v1_acceptance.sh` (v1 baseline command contract)
- `scripts/test_v1_6_acceptance.sh` (v1.6 wedge/flow checks)
- `scripts/test_v1_7_acceptance.sh` (v1.7 endpoint/provenance/fail-closed checks)
- `scripts/test_v1_8_acceptance.sh` (v1.8 interception/ecosystem checks)
- `scripts/test_release_smoke.sh` (release artifact + core smoke checks)
- `scripts/test_openclaw_skill_install.sh` (OpenClaw package install path)
- `scripts/test_beads_bridge.sh` (trace-to-beads deterministic bridge)
- `scripts/install.sh` (release installer path)

## Command Coverage (Functional)

The acceptance suites together exercise command families including:

- `demo`, `verify`, `run replay`, `run diff`, `run receipt`
- `regress init`, `regress run`, `regress bootstrap`
- `policy init`, `policy test`
- `gate eval`, `approve`
- `scout signal`
- `guard pack`, `guard verify`, `incident pack`
- `registry install`, `registry verify`
- `mcp bridge/proxy/serve` coverage through adapter and acceptance suites
- OpenClaw installable skill package path
- Gas Town adapter parity path
- Beads bridge dry-run/live simulation path
- External allowlist-to-policy generation path

## Preconditions

- Go toolchain available
- Python/uv toolchain available for SDK/adapter checks
- `gh` authenticated (required for release installer path)
- Homebrew installed (required for brew path)
- Network access for release asset and brew fetch

## Execution

Run from repo root:

```bash
bash scripts/test_uat_local.sh
```

Options:

```bash
GAIT_UAT_RELEASE_VERSION=vX.Y.Z bash scripts/test_uat_local.sh --output-dir ./gait-out/uat_local
bash scripts/test_uat_local.sh --skip-brew
```

Full-contract run across all install paths (use only when release/brew tag includes current epics):

```bash
GAIT_UAT_RELEASE_VERSION=<released-tag-with-v1.8> \
bash scripts/test_uat_local.sh --full-contracts-all-paths
```

## Outputs

- Human-readable logs: `gait-out/uat_local/logs/*.log`
- Machine-readable summary: `gait-out/uat_local/summary.txt`

## Pass Criteria

- All quality gates pass: `make lint`, `make test`, `make test-e2e`, `make test-adoption`, `make test-contracts`, `make test-hardening-acceptance`
- Runtime SLO budget check passes: `make test-runtime-slo`
- All install-path command suites pass for:
  - source binary
  - release-installer binary
  - Homebrew binary (unless explicitly skipped)
- Source binary must pass extended suite:
  - `v1.8` acceptance
  - OpenClaw install skill checks
  - Beads bridge checks
- If `--full-contracts-all-paths` is enabled, release-installer and Homebrew binaries must also pass the extended suite.
- Final summary reports no `FAIL` entries

## Failure Handling

On failure:

1. Open failing log under `gait-out/uat_local/logs/`.
2. Fix root cause in code/docs/scripts (not by weakening tests).
3. Re-run `bash scripts/test_uat_local.sh`.
4. Only merge when summary is fully green.
