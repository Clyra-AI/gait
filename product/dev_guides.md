# Gait Development Guide

Version: 1.0
Status: Normative
Scope: Gait only (`gait/` repository root).

This document is Gait's repo-local development overlay. It maps the shared Clyra
engineering expectations onto the commands, checks, and public contracts that
exist in this repository.

This is a toolchain and process guide. For architecture execution rules,
boundary ownership, and architecture-impacting PR expectations, see
`product/architecture_guides.md`.

## 1) Repository Type

Gait is an offline-first Go CLI and runtime-boundary product for AI agent tool
calls.

Primary implementation surfaces:

- Go CLI and runtime packages under `cmd/gait/` and `core/`
- Python SDK adoption layer under `sdk/python/`
- Markdown docs under `docs/`
- static docs site under `docs-site/`
- local UI shell under `ui/local/`
- examples and integration references under `examples/`

## 2) Source of Truth

Use these files in order when planning or implementing work:

1. `AGENTS.md`
2. `product/PRD.md`
3. `product/ROADMAP.md`
4. active plan file under `product/`
5. `product/dev_guides.md`
6. `product/architecture_guides.md`
7. command tests, schemas, and implementation code

If docs and code disagree, treat tested code and schema contracts as the
implementation source of truth, then update docs in the same change.

## 3) Local Gate Entry Points

Fast local checks:

```bash
make lint-fast
make test-fast
```

Normal pre-push lane:

```bash
make prepush
```

Full validation lane for public-surface, architecture, safety, release, or
workflow changes:

```bash
make prepush-full
```

Security and deeper lanes:

```bash
make codeql
make test-hardening
make test-chaos
make test-runtime-slo
make test-contracts
make test-scenarios
```

## 4) Toolchains

### Go

- Go is authoritative for policy evaluation, artifact schemas, canonicalization,
  hashing, signing, verification, zip packaging, diffing, replay, job runtime,
  MCP boundary behavior, voice token behavior, and CLI output.
- Pin the Go version in `go.mod` and keep `.tool-versions` aligned.
- Use `gofmt` for formatting and `go vet` for fast static checks.
- Keep dependencies exact in `go.mod`; never add floating runtime dependencies.

### Python

- Python is an adoption layer only.
- Keep SDK logic thin: serialization, local command invocation, and ergonomic
  wrappers.
- Do not duplicate policy parsing, verdict logic, signing, hashing, schema
  validation, or replay logic in Python.
- Use `uv`, `ruff`, `mypy`, `bandit`, and `pytest` through existing Makefile
  targets.

### Node and TypeScript

- Node/TypeScript are limited to docs-site and local UI surfaces.
- They are not part of the core CLI or runtime enforcement path.
- Use lockfile-backed installs through `npm ci`.

## 5) Public Contract Surfaces

Treat these as API surfaces:

- CLI commands, flags, help text, JSON output, and exit codes
- schemas under `schemas/v1/`
- pack, runpack, callpack, voice, context, registry, scout, and gateway artifacts
- signed trace and receipt formats
- Python SDK public wrapper API
- examples under `examples/integrations/` that users copy into projects
- docs and docs-site pages that describe integration behavior

Contract changes must be explicit, tested, documented, and migration-aware.

## 6) Determinism Requirements

Preserve deterministic behavior for:

- `verify`
- `diff`
- `replay`
- `regress`
- `gate eval`
- pack and zip generation
- schema validation
- JCS canonical JSON hashing and signing

Tests must use fixtures, temp directories, and stable timestamps where artifact
bytes are asserted.

## 7) Safety Requirements

- Non-`allow` verdicts must not execute side effects.
- High-risk inability to evaluate policy must fail closed.
- Raw capture, real replay, destructive tools, and production-like side effects
  require explicit unsafe flags or profile-controlled trust.
- Secrets, private keys, customer data, and raw sensitive payloads must not be
  committed or logged by default.
- Prefer digests, references, and redaction metadata over raw content.

## 8) Test Matrix

Use the smallest useful lane while developing, then run the required broader lane
before handoff.

| Change scope | Required minimum |
|---|---|
| Small internal Go change | `make lint-fast`, `make test-fast` |
| CLI flags, JSON, exits, help | targeted command tests, `make test-contracts`, `make prepush` |
| Schema or artifact contract | schema/golden tests, `make test-contracts`, relevant e2e |
| Gate, policy, MCP, voice, context safety | targeted fail-closed tests, `make test-hardening`, `make test-chaos` when applicable |
| SDK wrapper behavior | Python tests plus relevant Go command tests |
| Docs or examples | docs checks plus command/example smoke when behavior is described |
| Release or supply-chain surface | `make prepush-full`, release smoke, CodeQL where applicable |

## 9) Docs and Examples

When user-visible behavior changes, update the smallest relevant docs set in the
same change:

- `README.md`
- `docs/`
- `docs-site/public/llms.txt`
- `docs-site/public/llm/*.md`
- `examples/integrations/`
- `CHANGELOG.md`

Keep integration guidance before internals. Users should see how to put Gait at
the execution boundary before reading implementation details.

## 10) Changelog and Versioning

- Update `CHANGELOG.md` `## [Unreleased]` for user-visible behavior, public
  contract wording, CLI/help/JSON/exits, install/distribution UX, docs/governance
  trust surfaces, or explicit release-process changes.
- Keep changelog entries concise and operator-facing.
- Do not finalize versioned changelog sections during implementation work.
- Versioned release finalization belongs to the release workflow or release skill.
- Artifact schemas and CLI behavior must declare compatibility and migration
  expectations when changed.

## 11) Repo Hygiene

- Keep generated artifacts out of tracked source paths unless they are canonical
  fixtures or examples.
- Use `t.TempDir()` or equivalent in tests.
- Do not commit local runpacks, journals, coverage outputs, `.tmp`, caches, or UI
  build output.
- Keep `product/plans/adhoc/` for timestamped one-off plans; do not overwrite
  rolling plan files as part of adhoc planning.

