# Contributing to Gait

Thanks for your interest in contributing. This repo is optimized for **determinism**, **offline-first** workflows, and **stable artifacts**.

## Quick start

1. Install toolchain versions from `.tool-versions`.
2. Run:
   - `make fmt`
   - `make lint`
   - `make test`

## Pre-commit

Install pre-commit and run:

```
pre-commit install
```

Then verify:

```
pre-commit run --all-files
```

## Pre-push (required)

Enable the repo pre-push hook:

```
make hooks
```

This runs `make lint` and `make test` on every push.

## Code quality

- Go: `gofmt`, `golangci-lint`, `go vet`, `gosec`, `govulncheck`
- Python: `ruff`, `mypy`, `pytest`
- Coverage must be **>= 85%** for Go core and Python SDK.

## Troubleshooting lint/test environment

### `govulncheck` cannot reach `vuln.go.dev`

`govulncheck` requires network access to the Go vulnerability database.

- Verify outbound HTTPS and DNS resolution are allowed.
- In restricted corporate environments, configure a mirror with `GOVULNDB`.
- If module fetches are also blocked, verify `GOPROXY` and `GONOSUMDB` policy settings.

Example:

```
GOVULNDB=https://vuln.go.dev \
go run golang.org/x/vuln/cmd/govulncheck@latest -mode=binary ./gait
```

### `uv run pytest` or `uv run ...` fails due cache/runtime constraints

If `uv` fails in restricted or sandboxed environments, point caches to the repo workspace and retry:

```
mkdir -p .cache/uv .cache/go-build
UV_CACHE_DIR=$PWD/.cache/uv \
GOCACHE=$PWD/.cache/go-build \
make test
```

If `uv` crashes, upgrade to a current version and retry:

```
uv self update
```

## Issues and PRs

- Use the provided issue templates.
- Keep PRs focused and include tests.
- Avoid adding network dependencies to core flows.

## Triage and labels

Use these labels consistently:

- `bug`: defects or regressions
- `feature`: new behavior or enhancements
- `docs`: documentation-only changes
- `security`: security-impacting changes
- `breaking`: incompatible CLI or schema changes
- `good first issue`: newcomer-friendly tasks
- `needs-triage`: default for new issues

Triage flow:

1. Apply `needs-triage` on new issues.
2. Confirm reproducibility or intent, then swap to `bug` or `feature`.
3. Add `security` or `breaking` where relevant.
4. Prioritize with milestone or project board if used.

## Versioning policy

- CLI versioning and artifact schema versioning are tracked independently.
- Within a major version, schema changes are backward-compatible.
- Breaking schema or CLI changes require a major bump.

## Homebrew tap release process (gated)

Homebrew publishing is deferred until the v1 install and contract stability gate is met.

Gate criteria:

- At least one full release cycle with stable install/verify commands and no breaking packaging changes.
- Release includes integrity assets (`checksums.txt`, `checksums.txt.sig`, `checksums.txt.intoto.jsonl`, `sbom.spdx.json`, `provenance.json`).
- Exit-code and schema contracts remain stable for downstream automation.

Formula update workflow:

1. Cut and publish a signed GitHub release tag (`vX.Y.Z`).
2. Compute the release archive SHA256 used by Homebrew formula.
3. Update tap formula `url`, `sha256`, and `version`.
4. Open PR in tap repo and require CI pass on macOS.
5. Merge and verify:
   - `brew update`
   - `brew install <tap>/gait`
   - `gait --help`

Rollback process:

1. Revert formula in tap repo to last known good release.
2. Merge rollback PR with priority.
3. If needed, yank broken GitHub release or mark as superseded in release notes.
4. Open incident issue with root cause and required release hardening actions before next cut.
