# Gait: Agent Instructions (repo-wide)

This file gives coding assistants and contributors the project-wide rules for building **Gait**.

## What Gait is (v1)

Gait is an offline-first, default-safe CLI that makes production AI agent runs **controllable and debuggable by default** via:

- **Runpack**: record, verify, diff, replay (stub by default)
- **Regress**: turn runpacks into deterministic CI regressions
- **Gate**: evaluate tool-call intent against YAML policy, with approvals and signed traces
- **Doctor**: first-5-minutes diagnostics (stable JSON + fixes)
- **Scout**: local snapshot/diff/signal reporting for drift and incident clustering
- **Guard / Incident**: deterministic evidence bundles, verification, retention, and incident packaging

Supporting OSS surfaces shipped in v1:

- **Registry**: signed/pinned skill pack install + verify workflows
- **MCP bridge/proxy**: tool-call boundary adapters that route through Gate policy evaluation

The durable product contract is **artifacts and schemas**, not a hosted UI.

## Non-negotiable contracts

- **Determinism**: `verify`, `diff`, and **stub replay** must be deterministic given the same artifacts.
- **Offline-first**: core workflows must not require network access.
- **Default privacy**: record reference receipts by default (no raw sensitive content unless explicitly enabled).
- **Fail-closed safety**: in “production/high-risk” modes, inability to evaluate policy blocks execution.
- **Schema stability**: artifacts and `--json` outputs are versioned and remain backward-compatible within a major.
- **Stable exit codes**: treat exit codes as API surface; add new codes only intentionally.

## Architecture boundaries

- **Go is authoritative** for: schemas, canonicalization, hashing, signing/verification, zip packaging, diffing, stub replay, policy evaluation, and CLI output.
- **Python is an adoption layer only**: capture intent, call local Go, return structured results. No policy parsing/logic in Python.
- **Wrappers and sidecars are transport only**: all enforce/allow/block decisions come from Go (`gait gate eval`), never framework-local logic.
- **Node/TypeScript are not part of the v1 core**. If used later, keep it in adapters or tooling, not the core CLI path.

Current reference adapter set (keep parity): `openai_agents`, `langchain`, `autogen`, `openclaw`, `autogpt`, and the canonical sidecar path.

## Canonicalization, hashing, and artifacts

- Any JSON that participates in a digest, signature, cache key, or diff MUST be canonicalized using **RFC 8785 (JCS)** before hashing/signing.
- Zip artifacts must be **byte-stable** when regenerated from identical inputs:
  - deterministic file ordering
  - stable timestamps (fixed epoch)
  - stable file modes/ownership metadata
  - explicit compression settings
- Never hash “pretty printed” JSON or platform-dependent encodings.

## Security and privacy

- Never commit secrets, tokens, private keys, or real customer data.
- Avoid logging sensitive payloads; prefer digests + redaction metadata.
- All “unsafe” operations (real tool replay, raw capture, destructive tools) require explicit flags and must be obvious in help text and JSON outputs.
- Use standard crypto primitives (ed25519, sha256) from well-reviewed libraries; no custom crypto.

## Engineering standards

### Go

- Format: `gofmt` always; keep code idiomatic and boring.
- Errors: wrap with `%w`; return typed sentinel errors only when they improve caller behavior.
- Concurrency: keep it explicit; no background goroutines without lifecycle control.
- Time/locale: avoid locale-dependent formatting; timestamps should be RFC 3339/UTC or fixed epochs as defined by schema.
- IO: be careful with filesystem permissions; artifacts should be readable by the user but not world-writable by default.

### Python (wrapper SDK)

- Keep Python “thin”: serialization, subprocess/FFI boundary, and ergonomics only.
- Prefer strict typing; keep the public wrapper API small and stable.
- Tooling targets: `uv`, `ruff`, `mypy`, `pytest`.

## Tooling expectations (don’t pin versions here)

- CI should run Go linting + security scans (e.g. `golangci-lint`, `go vet`, `gosec`, `govulncheck`) and Python checks for wrapper code (`ruff`, `mypy`, `bandit`, `pytest`).
- Prefer a cross-platform CI matrix (macOS/Linux/Windows) and path-filtered workflows for speed.
- Releases should produce checksums, SBOMs, and signed provenance/attestations; treat release integrity separately from runpack/trace signing.
- Keep pre-commit and pre-push enforcement active (`make hooks`), with pre-push running `make lint` and `make test`.

## Tests (what to add as the repo grows)

- Prefer table-driven tests and golden fixtures for:
  - `--json` output stability
  - exit codes
  - schema validation and migrations
  - JCS canonicalization and digest stability
  - zip determinism (same inputs => same bytes)
  - policy evaluation determinism
- Maintain deterministic integration/e2e/acceptance suites for adapters, policy compliance, hardening, and release smoke paths.
- Keep coverage gates at **>= 85%** for Go core/CLI and Python SDK in CI.
- Tests must be offline and hermetic by default (no network, no cloud accounts).

## Repo hygiene

- Keep dependencies minimal, especially in core (`cmd/gait` and `core/*`).
- Avoid adding dashboards/services in v1; keep scope on the execution path.
- When introducing a new artifact/schema:
  - version it explicitly
  - add validation + golden fixtures
  - document upgrade/migration behavior

## Issue tracking (optional)

This repo may use **bd (beads)** for local, personal task tracking. It is not required for contributors.

- If `bd` is available, use it to find and track work:
  - `bd ready` (unblocked work)
  - `bd show <id>` / `bd list`
  - `bd create "Title" --type task --priority 2`
  - `bd dep add <blocked_id> <blocker_id>` (dependencies)
  - `bd close <id>`
- For up-to-date workflow context: `bd prime`
- Note: bd may be configured in stealth mode (local `.beads/` state excluded from git). Do not commit beads artifacts unless explicitly requested.

## Working with this file

- Keep these instructions short, concrete, and current.
- If a subdirectory needs specialized rules, add another `AGENTS.md` there (it scopes to that subtree).
