# Changelog

All notable changes to Gait are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html) including pre-release tags.

## [Unreleased]

### Added

- _No unreleased entries yet._

## [1.0.8] - 2026-02-12

### Added

- Added runpack-first landing and demo assets, including a deterministic hero capture script (`scripts/record_runpack_hero_demo.sh`) and 20-second terminal GIF/MP4 outputs.
- Added `gait-regress` action v2 contract with release-binary checksum verification, dual command modes (`regress`, `policy-test`), bounded summaries, artifact uploads, and documented outputs.
- Added Python SDK run-level session capture ergonomics (`run_session`) with deterministic run-record emission support and session-aware adapter tests/examples.
- Added CI example workflow for deterministic failing regress scenarios with uploaded diff artifacts (`examples/ci/gait-regress-failing`).

### Changed

- Updated README above-the-fold messaging to lead with verifiable runpacks, quick demo/verify flow, and optional gate positioning below the fold.
- Expanded CI regress kit documentation and contract tests to validate the new composite action interface.
- Updated Python reference adapter docs/examples to the one-decorator + one-context-manager onboarding path.

## [1.0.7] - 2026-02-12

### Fixed

- Fixed CodeQL high-severity log-injection findings in `cmd/gait/run_session.go` by hardening log-field handling around user-controlled values.
- Fixed CodeQL warnings in `scripts/check_command_budgets.py` by removing duplicate variable definitions and clarifying assignment flow.

## [1.0.6] - 2026-02-12

### Added

- Added v2.2 OSS hardening controls for prime-time operation, including stricter MCP/service boundary behavior, session durability paths, and expanded chaos/runtime acceptance coverage.
- Added v2.3 adoption/conformance surfaces: lane scorecard automation, Intent+Receipt conformance contract/docs/tests, CI regress template validation, and blessed OpenAI Agents integration guidance.
- Added release/distribution automation artifacts for v2.3 metrics snapshots and ecosystem release note rendering in CI/release flows.

### Changed

- Expanded CI/UAT workflows to enforce v2.2 and v2.3 gates (contracts, adoption, ecosystem automation, regression template, and full install-path validation).
- Updated wrapper quickstart and acceptance flows to emit deterministic key-value checkpoints suitable for automation parsing and adoption timing measurement.
- Refined integration, launch, and ecosystem docs to align with the blessed lane strategy (coding-agent wrapper + GitHub Actions CI) and adapter expansion guardrails.

### Fixed

- Fixed remaining CI blockers from v2.3 rollout by aligning v1.6 acceptance quickstart assertions with current output contract and hardening ecosystem automation output directory handling.
- Resolved session-path and contention reliability defects addressed during v2.2 hardening and CodeQL-driven safety remediation.

## [1.0.5] - 2026-02-11

### Added

- Added v2.1 runtime surfaces for sessionized execution evidence, including session journal/checkpoint/chain schemas and CLI workflows (`run session` + chain verify/diff flows).
- Added delegated execution controls across Gate policy evaluation, delegation token/audit artifacts, and policy matching constraints for delegated identities/scope/depth.
- Added policy helper and session guard test coverage to keep repository-wide release gates at >= 85% while exercising new adoption/hardening branches.

### Changed

- Expanded UAT and local quality coverage paths for plan-audit and release readiness validation.
- Updated docs and integration examples to reflect sessionized runpack evidence and delegated runtime enforcement patterns.

### Fixed

- Hardened session journal/lock file path handling to resolve CodeQL `go/path-injection` findings in `core/runpack/session.go`.

## [1.0.4] - 2026-02-11

### Added

- Added `gait policy simulate` for deterministic baseline-versus-candidate policy comparison across fixture corpora, including rollout-stage recommendation output.
- Added signing key lifecycle commands: `gait keys init`, `gait keys rotate`, and `gait keys verify`.
- Added normative artifact-graph contract documentation (`docs/contracts/artifact_graph.md`) and docs-site navigation exposure.
- Added Python SDK decorator ergonomics via `gate_tool` with framework-style examples for OpenAI/LangChain tool wrappers.

### Changed

- Expanded policy authoring and integration docs with policy simulation and key workflow guidance.
- Expanded acceptance/UAT coverage to exercise new policy simulation and key lifecycle paths.
- Updated homepage quickstart to the one-command regression bootstrap path.

## [1.0.3] - 2026-02-11

### Added

- Added `gait run inspect` for deterministic, human-readable run timeline inspection with `--json` output support.
- Added transport-specific `gait mcp serve` endpoints for SSE (`/v1/evaluate/sse`) and streamable HTTP/NDJSON (`/v1/evaluate/stream`), with shared policy-evaluation semantics.
- Added a dedicated nightly Windows lint workflow (`.github/workflows/windows-lint-nightly.yml`) to preserve coverage off the merge-path fast lane.

### Changed

- Updated docs and UAT coverage for `run inspect` and transport-aware MCP serve modes.
- Removed Windows lint from the primary CI lint matrix to reduce per-push CI latency.

## [1.0.2] - 2026-02-10

### Fixed

- Hardened path handling for CodeQL "Uncontrolled data used in path expression" findings across MCP, runpack, trace, and filesystem helpers.
- Stabilized Windows write-error tests by forcing deterministic directory-write failures across platforms.
- Improved UAT/release reliability around latest-release resolution and cross-platform script behavior.

## [1.0.1-rc4] - 2026-02-09

### Fixed

- CI/release reliability on Windows lock contention.
- Release asset download resilience in the release workflow.

## [1.0.1-rc3] - 2026-02-09

### Changed

- Release publish job permissions (`id-token`, `contents`) for signing and artifact upload.

## [1.0.1-rc2] - 2026-02-09

### Fixed

- Release workflow now adds Go bin directory to `PATH` for tool installation steps.

## [1.0.1-rc1] - 2026-02-09

### Fixed

- Mermaid diagram rendering in docs.
- SEO/AEO documentation assets and metadata quality.

## [1.0.0] - 2026-02-09

### Added

- Initial public OSS release of Gait v1 primitives (`runpack`, `regress`, `gate`, `doctor`).
- Signed artifacts, deterministic regression loop, and policy-gated execution boundary.
