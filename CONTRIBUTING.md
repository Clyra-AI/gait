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
