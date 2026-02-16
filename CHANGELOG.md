# Changelog

All notable changes to Gait are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html) including pre-release tags.

## [Unreleased]

### Added

- _No unreleased entries yet._

## [1.2.2] - 2026-02-16

### Added

- Added productized adoption/distribution surfaces: reusable adoption regress workflow template, Marketplace-ready GitHub Action metadata/docs, and canonical MCP policy/evidence demo coverage.
- Added PackSpec standardization docs: stability contract, producer kit, and compatibility matrix pages.
- Added portable skill packs plus submission variants for incident-to-regression, CI failure triage, and evidence receipt generation workflows.

### Changed

- Refined docs and docs-site information architecture for clearer problem-first guidance, capability naming consistency, and stronger LLM/AEO discoverability.
- Added docs hub navigation/discovery surfaces and crawler-facing resource links for `llms`, `llms.txt`, `llms-full.txt`, and AI sitemap resources.
- Updated docs-site validation pipeline behavior to include required `ripgrep` installation before docs consistency checks.

### Fixed

- Hardened runpack verify contract behavior and integrity checks in read/verify paths while reducing verification-path allocations.
- Fixed Windows pack-producer-kit checksum flow reliability in contracts CI.
- Fixed docs-site favicon and static discovery alias regressions affecting browser rendering and extensionless route availability.

### Upgrade Notes

- No breaking CLI or schema major-version changes.
- Recommended post-upgrade validation:
  - `make test-fast`
  - `make test-acceptance`
  - `make docs-site-check`
  - `make codeql`

## [1.2.1] - 2026-02-15

### Added

- Added voice mode v1 command surface: `gait voice token mint|verify` and `gait voice pack build|verify|inspect|diff`.
- Added voice schemas and fixtures for `CommitmentIntent`, `SayToken`, `CallEvent`, and `CallpackManifest`, plus `pack_type=call` payload support.
- Added reference voice adapter (`examples/integrations/voice_reference/`) and a dedicated acceptance lane (`scripts/test_voice_acceptance.sh`, `make test-voice-acceptance`, CI `voice-acceptance` job).
- Added voice mode documentation and integration/runbook coverage (`docs/voice_mode.md`, checklist/UAT/test cadence updates).

### Changed

- Expanded PackSpec contract docs to include callpack artifacts and `pack build --type call`.
- Expanded adoption smoke coverage to include allow/block/require-approval voice adapter scenarios and callpack artifact checks.

### Fixed

- Fixed speak-gate verification to require a matching gated `tts.emitted` speak receipt/token path and to enforce that the latest prior gate verdict for the same turn/class is `allow`.
- Fixed callpack verification to enforce metadata parity between `callpack_manifest.json` and `call_payload.json`, including `privacy_mode`, `environment_fingerprint`, and `created_at`.
- Fixed regress initialization path to treat callpacks as valid `--from` sources via embedded runpack extraction.

### Upgrade Notes

- No breaking CLI or schema major-version changes; voice mode is additive.
- Recommended post-upgrade validation:
  - `make test-voice-acceptance`
  - `make test-adoption`
  - `make test-adapter-parity`
  - `make codeql`

## [1.2.0] - 2026-02-14

### Added

- Added v2.4 durable job runtime lifecycle commands (`job submit/status/checkpoint/pause/approve/resume/cancel/inspect`) with deterministic state transitions and stop-reason contracts.
- Added PackSpec v1 unified pack lifecycle (`pack build/verify/inspect/diff`) for run and job sources, including deterministic manifesting and offline verification.
- Added v2.5 context-proof evidence envelopes (`context_set_digest`, evidence mode contracts, context conformance checks) across capture, trace, pack, and regress flows.
- Added expanded conformance and hardening lanes for context chaos, PackSpec TCK, runtime SLO/perf checks, and release integrity automation.

### Changed

- Changed runtime convergence so run/job artifacts share a single pack-first verification and inspection surface for CI/audit workflows.
- Changed acceptance/UAT and release gates to include v2.4 and v2.5 capability coverage, including context-proof and PackSpec contract checks.
- Changed docs/contracts and runbooks to reflect durable-job-first operations, unified pack workflows, and context evidence rollout guidance.

### Fixed

- Fixed v2.4/v2.5 rollout regressions discovered in CI and CodeQL passes, including path-hardening and command-budget validation defects.
- Fixed validation gaps where newly shipped context and pack capabilities were not consistently represented across local/CI gate paths.

### Upgrade Notes

- No breaking CLI or schema major-version changes.
- Existing runpack readers remain compatible; new pack/context fields are additive.
- Recommended post-upgrade validation:
  - `make test-v2-4-acceptance`
  - `make test-v2-5-acceptance`
  - `make test-context-conformance`
  - `make test-packspec-tck`

### Contract Impact Summary

- Contract impact is additive: durable job runtime and PackSpec v1 become first-class OSS primitives; context-proof evidence extends verification semantics without removing prior fields.

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
