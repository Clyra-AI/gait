# Gait: Agent Instructions (repo-wide)

This file gives coding assistants and contributors the project-wide rules for building **Gait**.

## What Gait is

Gait is an offline-first durable agent runtime that dispatches long-running jobs, captures signed evidence, and enforces fail-closed policy at the tool boundary.

Core primitives:

- **Jobs**: dispatch multi-step agent work with checkpoints, pause/resume/cancel, approval gates, and deterministic stop reasons (`gait job submit/status/checkpoint/pause/resume/cancel/approve/inspect`)
- **Packs**: unified portable artifact envelope (PackSpec v1) for run, job, and call evidence with Ed25519 signatures and SHA-256 manifest (`gait pack verify/diff`)
- **Gate**: evaluate structured tool-call intent against YAML policy with fail-closed enforcement (`gait gate eval`)
- **Regress**: convert incidents into deterministic CI regression fixtures with JUnit output (`gait regress bootstrap`)
- **Voice**: gate high-stakes spoken commitments before utterance via signed SayToken capability tokens and callpack artifacts (`gait voice token mint/verify`, `gait voice pack build/verify/diff`)
- **Context Evidence**: deterministic proof of what context the model was working from, with privacy-aware envelopes and fail-closed enforcement when evidence is missing
- **Doctor**: first-5-minutes diagnostics (stable JSON + fixes) (`gait doctor --json`)

Supporting surfaces:

- **Registry**: signed/pinned skill pack install + verify workflows
- **MCP proxy/bridge/serve**: transport-aware boundary adapters (`stdio`, `SSE`, streamable HTTP) that route through Gate policy evaluation
- **Scout**: local snapshot/diff/signal reporting for drift and incident clustering

The durable product contract is **artifacts and schemas**, not a hosted UI.

## Non-negotiable contracts

- **Determinism**: `verify`, `diff`, and **stub replay** must be deterministic given the same artifacts.
- **Offline-first**: core workflows must not require network access.
- **Default privacy**: record reference receipts by default (no raw sensitive content unless explicitly enabled).
- **Fail-closed safety**: in “production/high-risk” modes, inability to evaluate policy blocks execution.
- **Schema stability**: artifacts and `--json` outputs are versioned and remain backward-compatible within a major.
- **Stable exit codes**: treat exit codes as API surface; add new codes only intentionally.

## Architecture boundaries

- **Go is authoritative** for: schemas, canonicalization, hashing, signing/verification, zip packaging, diffing, stub replay, policy evaluation, job lifecycle, voice token minting/verification, and CLI output.
- **Python is an adoption layer only**: capture intent, call local Go, return structured results. No policy parsing/logic in Python. Keep SDK ergonomics thin (`ToolAdapter`, minimal decorators), not framework replacement.
- **Wrappers and sidecars are transport only**: all enforce/allow/block decisions come from Go (`gait gate eval`, `gait mcp proxy`, `gait mcp serve`), never framework-local logic.
- **Next.js docs site** (`docs-site/`): static export deployed to GitHub Pages. Reads markdown from `docs/` via gray-matter frontmatter. No runtime dependencies.
- **Node/TypeScript**: limited to docs site and local UI shell (`ui/local/`). Not part of the core CLI path.

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
- Keep git hooks active (`make hooks`) with pre-push running `make prepush` by default (`make lint-fast` + `make test-fast`), and use `GAIT_PREPUSH_MODE=full git push` for full local gates; keep `.pre-commit-config.yaml` aligned with current checks if pre-commit is used locally.

## Key paths

- `cmd/gait/` — CLI entry points and command wiring
- `core/gate/` — policy evaluation, voice token minting/verification
- `core/jobruntime/` — durable job lifecycle (submit, checkpoint, pause, resume, cancel)
- `core/pack/` — PackSpec v1 pack/verify/diff
- `core/runpack/` — legacy runpack capture/verify/diff/replay
- `core/contextproof/` — context evidence envelopes
- `core/regress/` — regression bootstrap and execution
- `core/mcp/` — MCP proxy/bridge/serve adapters
- `schemas/v1/` — JSON schemas for all artifact types
- `sdk/python/` — Python wrapper SDK
- `docs/` — markdown docs (consumed by docs-site via gray-matter)
- `docs-site/` — Next.js static docs site
- `examples/integrations/` — reference adapters (voice, langchain, openai, etc.)

## Source of truth for assistants

- CLI behavior and flag contracts: `cmd/gait/*` usage/help text and command tests in `cmd/gait/*_test.go`.
- Artifact schemas and compatibility rules: `schemas/*`, `core/*` validators, and contract tests.
- Acceptance/UAT coverage scope: `docs/uat_functional_plan.md` and `scripts/test_*acceptance*.sh`.
- If docs and code disagree, treat code/tests as source of truth and patch docs in the same change.

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

## What to watch for in changes

- **Behavior regressions**: any change to `verify`, `diff`, `replay`, `regress`, or `gate eval` output must not break existing golden fixtures or exit code contracts. Run `make test-fast` at minimum before pushing.
- **Safety / fail-closed controls**: never weaken a non-allow → non-execute path. If gate evaluation cannot reach a verdict, the default must remain block. Same for voice (no SayToken → no gated speech) and context evidence (missing envelope → fail-closed for high-risk actions).
- **Determinism**: changes to JSON serialization, zip packaging, hashing, or signing must preserve byte-for-byte reproducibility. Test with existing fixtures, not just new ones.
- **CI / portability**: CI templates live in `.github/workflows/` and must work across Go versions declared in `go.mod`. Don't add platform-specific paths or hardcoded tool versions without matrix coverage.
- **Docs vs CLI drift**: if you change a command's flags, arguments, or exit codes, update `docs/`, `docs-site/public/llms.txt`, `docs-site/public/llm/*.md`, and `README.md` in the same change. Treat `cmd/gait/*` usage strings as the source of truth.
- **Test artifact leaks**: tests that create artifacts (packs, tokens, journals) must use `t.TempDir()` or equivalent. Never write test output to the source tree.
- **Shell quoting in PRs**: when using `gh pr create --body`, backticks and `$()` in the body text get interpreted by the shell. Use a `<<'EOF'` heredoc (single-quoted delimiter) to pass the body safely. Same applies to commit messages containing backticks.

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
