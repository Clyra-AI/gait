# Local UAT + Functional Test Plan

This runbook defines how to validate Gait end-to-end on a local machine across all supported install paths.

## Goal

Prove that a user can:

- install Gait via the primary distribution path (release installer)
- optionally install Gait via alternate distribution paths
- execute the core command surface successfully
- verify deterministic contracts (runpack, regress, gate, evidence, signal)
- pass existing quality gates (lint, tests, coverage, acceptance)

## Install Paths In Scope

1. Primary: GitHub release installer (`scripts/install.sh`)
2. Alternate: source build (`go build -o ./gait ./cmd/gait`)
3. Alternate: Homebrew tap (`davidahmann/tap/gait`)

Windows is included in CI matrix validation, but this local script focuses on Linux/macOS hosts.

The UAT script refreshes Homebrew taps before reinstall to avoid stale formula reads during release validation.

## Required Scripts

- `scripts/test_uat_local.sh` (orchestrator; this document's entrypoint)
- `scripts/test_v1_acceptance.sh` (v1 baseline command contract)
- `scripts/test_v1_6_acceptance.sh` (v1.6 wedge/flow checks)
- `scripts/test_v1_7_acceptance.sh` (v1.7 endpoint/provenance/fail-closed checks)
- `scripts/test_v1_8_acceptance.sh` (v1.8 interception/ecosystem checks)
- `scripts/test_v2_3_acceptance.sh` (v2.3 adoption/conformance/distribution gate + metrics snapshot)
- `scripts/test_v2_4_acceptance.sh` (v2.4 job/pack/signing/replay/credential-ttl acceptance gate)
- `scripts/test_mcp_canonical_demo.sh` (canonical MCP allow/block/require-approval boundary demo with per-call pack evidence)
- `scripts/test_v2_5_acceptance.sh` (v2.5 context-evidence and context-policy gate)
- `scripts/test_v2_6_acceptance.sh` (v2.6 activation guidance + durable/policy demo + tour gate)
- `scripts/test_voice_acceptance.sh` (voice mode commitment gate + say-token + callpack gate)
- `scripts/test_context_conformance.sh` (regress context conformance gate)
- `scripts/test_context_chaos.sh` (context chaos and drift-classification gate)
- `scripts/test_ui_acceptance.sh` (localhost UI command/API acceptance gate)
- `scripts/test_ui_e2e_smoke.sh` (UI first-run + regress headless smoke gate)
- `scripts/test_packspec_tck.sh` (PackSpec v1 fixture/TCK determinism and verify contract)
- `scripts/test_release_smoke.sh` (release artifact + core smoke checks)
- `scripts/test_hardening_acceptance.sh` (hardening acceptance + deterministic boundary checks)
- `scripts/test_chaos_exporters.sh`, `scripts/test_chaos_service_boundary.sh`, `scripts/test_chaos_payload_limits.sh`, `scripts/test_chaos_sessions.sh`, `scripts/test_chaos_trace_uniqueness.sh` (v2.2 chaos gates)
- `scripts/test_session_soak.sh` (long-run session durability + contention gate)
- `scripts/test_openclaw_skill_install.sh` (OpenClaw package install path)
- `scripts/test_beads_bridge.sh` (trace-to-beads deterministic bridge)
- `scripts/install.sh` (release installer path)

## Command Coverage (Functional)

The acceptance suites together exercise command families including:

- `demo`, `verify`, `run replay`, `run diff`, `run receipt`
- `run inspect` (human-readable runpack timeline)
- `regress init`, `regress run`, `regress bootstrap`
- trajectory regression assertions (`expected_tool_sequence`, `expected_verdict_sequence`) through regress fixture/run paths
- `policy init`, `policy validate`, `policy fmt`, `policy test`, `policy simulate`
- `keys init`, `keys rotate`, `keys verify`
- `gate eval`, `approve`
- `scout signal`
- `guard pack`, `guard verify`, `incident pack`
- `voice token mint`, `voice token verify`, `voice pack build|verify|inspect|diff`
- `registry install`, `registry verify`
- `mcp bridge/proxy/serve` coverage through adapter and acceptance suites
- MCP canonical boundary coverage includes state-changing per-call PackSpec emission and verification
- `pack export` coverage includes OTEL JSONL emission and PostgreSQL metadata-index SQL sink generation
- v2.2 chaos abuse coverage (boundary abuse, payload limits, exporter corruption, session contention, trace uniqueness)
- long-running session soak coverage (append/checkpoint/verify + contention budget)
- v2.3 blessed lane coverage (OpenAI wrapper flow + reusable CI regress template assumptions)
- v2.5 context evidence/conformance/chaos coverage (capture, policy fail-closed checks, deterministic drift classification)
- v2.6 activation coverage (`demo` next-step guidance, `demo --durable`, `demo --policy`, `tour`, `doctor --summary`)
- intent+receipt conformance coverage (`scripts/test_intent_receipt_conformance.sh`)
- OpenClaw installable skill package path
- Gas Town adapter parity path
- Beads bridge dry-run/live simulation path
- External allowlist-to-policy generation path

## Preconditions

- Go toolchain available
- Python/uv toolchain available for SDK/adapter checks
- Node.js/npm toolchain available for UI unit and docs-site checks
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
GAIT_UAT_RELEASE_VERSION=vX.Y.Z bash scripts/test_uat_local.sh --output-dir ./.uat_local
bash scripts/test_uat_local.sh --skip-brew
bash scripts/test_uat_local.sh --skip-docs-site
```

Legacy fallback when validating an older release tag that predates the extended suites:

```bash
GAIT_UAT_RELEASE_VERSION=<older-tag> \
bash scripts/test_uat_local.sh --baseline-install-paths
```

## Outputs

- Human-readable logs: `.uat_local/logs/*.log`
- Machine-readable summary: `.uat_local/summary.txt`
- Primary path marker in summary: `PRIMARY_INSTALL_PATH_STATUS release-installer PASS`
- Default output is `.uat_local` (outside `gait-out/`) so cleanup steps in downstream suites do not truncate the orchestrator summary.

## Pass Criteria

- All quality gates pass: `make lint`, `make test`, `make test-e2e`, `go test ./internal/integration -count=1`, `make test-adoption`, `make test-adapter-parity`, `scripts/policy_compliance_ci.sh`, `make test-contracts`, `make test-v2-3-acceptance`, `make test-v2-4-acceptance`, `make test-v2-5-acceptance`, `make test-v2-6-acceptance`, `make test-voice-acceptance`, `make test-context-conformance`, `make test-context-chaos`, `make test-ui-acceptance`, `make test-ui-unit`, `make test-ui-e2e-smoke`, `make test-ui-perf`, `make test-packspec-tck`, `make test-hardening-acceptance`, `make test-chaos`, `bash scripts/test_session_soak.sh`
- Runtime SLO budget check passes: `make test-runtime-slo`
- Performance regression checks pass: `make bench-check`
- Docs site lint/build/render checks pass (`make docs-site-lint docs-site-build docs-site-check`) unless explicitly skipped with `--skip-docs-site`
- All install-path command suites pass for:
  - source binary
  - release-installer binary
  - Homebrew binary (unless explicitly skipped)
- All install paths must pass the extended suite by default:
  - `v1.8` acceptance
  - OpenClaw install skill checks
  - Beads bridge checks
- Use `--baseline-install-paths` only for legacy tag compatibility checks.
- Final summary reports no `FAIL` entries

## Failure Handling

On failure:

1. Open failing log under `.uat_local/logs/`.
2. Fix root cause in code/docs/scripts (not by weakening tests).
3. Re-run `bash scripts/test_uat_local.sh`.
4. Only merge when summary is fully green.
