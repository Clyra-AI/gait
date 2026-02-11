# Changelog

All notable changes to Gait are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html) including pre-release tags.

## [Unreleased]

### Added

- _No unreleased entries yet._

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
