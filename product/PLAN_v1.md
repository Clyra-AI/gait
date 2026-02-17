# PLAN v1: Gait (Zero-Ambiguity Build Plan)

Date: 2026-02-05
Source of truth: `PRD.md` and `ROADMAP.md`
Scope: v1 only (Runpack, Regress, Gate, Doctor, minimal adapters)

This plan is written to be executed top-to-bottom with minimal interpretation. Each story includes concrete repo paths, commands, and acceptance criteria.

---

## Global Decisions (Locked for v1)

- Core runtime and CLI: **Go** single static binary (`cmd/gait`)
- Wrapper SDK: **Python** (thin adoption layer only) under `sdk/python/`
- Canonicalization for any hashed/signed JSON: **RFC 8785 (JCS)**
- Schema format for persisted artifacts: **JSON Schema Draft 2020-12**
- Cryptography: `ed25519` signatures, `sha256` digests
- Default mode: **offline-first**, **stub replay**, **reference receipts** (no raw sensitive content)
- Safety: any real tool execution or raw capture requires explicit `--unsafe-*` flags and is surfaced in JSON output
- CLI ergonomics: flags may be placed before or after positional arguments for major commands (for example `gait verify run_demo --json`).
- Agent UX: major commands support `--explain` with concise intent text.
- Coverage target: **>= 85% line coverage** for Go core packages and Python SDK (enforced in CI)

---

## Repository Layout (Created in Epic 0)

```
.
├─ AGENTS.md
├─ PRD.md
├─ ROADMAP.md
├─ cmd/gait/                     # Go CLI entrypoint
├─ core/                         # Go core modules (authoritative)
│  ├─ adapters/                  # interfaces + 1 reference adapter
│  ├─ doctor/
│  ├─ export/                    # interfaces only (optional)
│  ├─ gate/
│  ├─ guard/                     # reserved v1.1+ interface package (v1)
│  ├─ jcs/                       # RFC 8785 canonicalization
│  ├─ mcp/                       # reserved v1.1+ interface package (v1)
│  ├─ policytest/
│  ├─ registry/                  # local-only backend v1
│  ├─ regress/
│  ├─ runpack/
│  ├─ schema/
│  │  └─ v1/                     # Go types matching schemas/
│  ├─ scout/                     # reserved v1.1+ interface package (v1)
│  ├─ sign/
│  └─ zipx/                      # deterministic zip writer/reader
├─ schemas/v1/                   # JSON Schemas (2020-12)
│  ├─ guard/                     # reserved v1.1+ schemas shipped in v1
│  ├─ registry/                  # reserved v1.1+ schemas shipped in v1
│  └─ scout/                     # reserved v1.1+ schemas shipped in v1
├─ sdk/python/gait/              # Python thin wrapper SDK
├─ examples/                     # runnable examples (offline/stubbed)
├─ product/PLAN_v1.md
└─ .github/workflows/            # CI and release
```


Additional v1 guardrails (enterprise-facing):
- Gate evaluates **only structured intent fields** (tool name, args, targets, and context). Policy must not parse or depend on natural-language prompts or retrieved content.
- High-risk side effects should be modeled as explicit tool-call intents with declared targets/destinations. Avoid free-form “do anything” tools in production modes.
- Attach lightweight provenance metadata to intents (user-supplied vs tool output vs external content) so policy can block unsafe data flows deterministically. Reserve deeper taint tracking for v1.2.

---

## Epic 0: Foundations, Scaffold, and Repo Stewardship

Objective: make the repo buildable, testable, linted, and releasable before writing product logic.

### Story 0.1: Scaffold the repo structure

Tasks:
- Create directories exactly as listed in “Repository Layout”.
- Add `README.md` with the PRD funnel order: promise → install → `gait demo` → artifact → verify → next steps.
- Add `LICENSE` (use **Apache-2.0**).
- Add `CONTRIBUTING.md`, `SECURITY.md`, and `CODE_OF_CONDUCT.md`.

Acceptance criteria:
- `README.md` exists and contains a “Start here” section with a single path.
- Repo contains the directory tree above.

### Story 0.2: Toolchain and dependency management

Tasks:
- Add `.tool-versions` (asdf-compatible) and pin:
  - `golang 1.25.x`
  - `python 3.13.x`
- Go module init at repo root:
  - `go mod init github.com/Clyra-AI/gait`
- Python SDK uses `uv`:
  - `sdk/python/pyproject.toml`
  - `sdk/python/uv.lock`

Acceptance criteria:
- `go.mod` exists and `go test ./...` runs (even if only skeleton tests exist).
- `sdk/python/pyproject.toml` exists and `uv run -m pytest` works (with at least 1 placeholder test).

### Story 0.3: Local developer commands (single entrypoint)

Tasks:
- Add `Makefile` with these targets (names exact):
  - `make fmt` (Go `gofmt`, Python `ruff format`)
  - `make lint` (Go + Python linters)
  - `make test` (unit tests + coverage)
  - `make test-e2e` (CLI-level tests)
  - `make build` (local build of `gait`)
- Add `scripts/` only if needed; prefer `Makefile` targets.

Acceptance criteria:
- `make fmt && make lint && make test` succeeds on a clean checkout.

### Story 0.4: Pre-commit hooks (fast, deterministic)

Tasks:
- Add `.pre-commit-config.yaml` with hooks for:
  - whitespace/eol checks
  - Go formatting (`gofmt`) and `go mod tidy` check
  - Python formatting/lint (`ruff`)
  - (optional) detect secrets (lightweight, local)
- Document in `CONTRIBUTING.md` how to enable pre-commit.

Acceptance criteria:
- `pre-commit run --all-files` passes on a clean repo.

### Story 0.5: CI (GitHub Actions)

Tasks:
- Add `.github/workflows/ci.yml`:
  - OS matrix: `ubuntu-latest`, `macos-latest`, `windows-latest`
  - Jobs:
    - `lint` (Go + Python)
    - `test` (Go + Python; enforce coverage)
    - `build` (compile `cmd/gait`)
  - Path filtering so docs-only changes skip heavy jobs.
- Enforce coverage >= 85%:
  - Go: compute total coverage from `go test -coverprofile`
  - Python: `pytest --cov` for `sdk/python/gait`

Acceptance criteria:
- CI runs on PR and main push.
- CI fails if coverage < 85%.

### Story 0.6: Release supply chain (integrity for the Go binary)

Tasks:
- Add `.github/workflows/release.yml` triggered by tags `v*`.
- Use `goreleaser` to build multi-platform artifacts.
- Generate:
  - checksums
  - SBOMs (Syft)
  - vulnerability scan report (Grype)
  - signed attestations/provenance (Cosign)
- Keep release signing separate from **runpack/trace signing** (product crypto).

Acceptance criteria:
- Tagging `v0.1.0` produces build artifacts for macOS/Linux/Windows and publishes checksums + SBOM.

### Story 0.7: Repo stewardship (operational readiness)

Tasks:
- Add GitHub templates:
  - `.github/ISSUE_TEMPLATE/bug.yml`
  - `.github/ISSUE_TEMPLATE/feature.yml`
  - `.github/pull_request_template.md`
- Add `CODEOWNERS` (if desired).
- Define labels and triage in `CONTRIBUTING.md`.
- Add a short “versioning policy” section: CLI and artifact schemas versioned independently but compatible within major.

Acceptance criteria:
- New issues/PRs use templates; repo has a documented triage flow.

---

## Epic 1: Schemas, Canonicalization, and Versioning

Objective: ship the artifact and inter-process contracts first, with validators and golden tests.

### Story 1.1: Define v1 schemas (JSON Schema 2020-12)

Create schema files:
- `schemas/v1/runpack/manifest.schema.json`
- `schemas/v1/runpack/run.schema.json`
- `schemas/v1/runpack/intent.schema.json` (one JSONL record)
- `schemas/v1/runpack/result.schema.json` (one JSONL record)
- `schemas/v1/runpack/refs.schema.json`
- `schemas/v1/gate/trace_record.schema.json`
- `schemas/v1/gate/intent_request.schema.json`
- `schemas/v1/gate/gate_result.schema.json`
- `schemas/v1/policytest/policy_test_result.schema.json`
- `schemas/v1/regress/regress_result.schema.json`
- Reserved v1.1+ schemas shipped in v1:
  - `schemas/v1/scout/inventory_snapshot.schema.json`
  - `schemas/v1/guard/pack_manifest.schema.json`
  - `schemas/v1/registry/registry_pack.schema.json`

Rules:
- Every artifact includes: `schema_id`, `schema_version`, `created_at`, `producer_version`.
- Add reserved fields for v1.1+ where the PRD calls them out.

Acceptance criteria:
- All schemas validate with a 2020-12 compatible validator in CI.

### Story 1.2: Go types and schema validators

Tasks:
- Add Go structs under `core/schema/v1/...` matching schema files 1:1.
- Add `core/schema/validate/` with a validator that can validate:
  - a JSON file against a schema
  - a JSONL stream against a record schema
- Use a Draft 2020-12 compatible Go validator library (v1 decision):
  - `github.com/kaptinlin/jsonschema`
- Add unit tests using fixtures under `core/schema/testdata/`.

Acceptance criteria:
- `go test ./core/schema/...` passes and validates both valid and invalid fixtures.

### Story 1.3: RFC 8785 (JCS) canonicalization

Tasks:
- Implement `core/jcs`:
  - `CanonicalizeJSON([]byte) ([]byte, error)`
  - `DigestJCS([]byte) (sha256_hex, error)`
- Base implementation on a well-tested RFC 8785 library (v1 decision):
  - `github.com/gowebpki/jcs` (wrap it behind `core/jcs`)
- Add fixtures derived from RFC examples under `core/jcs/testdata/`.

Acceptance criteria:
- Canonicalization output is stable across OSes in CI.

---

## Epic 2: Deterministic Packaging and Cryptographic Signing

Objective: make artifacts verifiable offline and reproducible.

### Story 2.1: Deterministic zip writer/reader

Tasks:
- Implement `core/zipx` to write `runpack_<run_id>.zip` deterministically:
  - stable entry ordering
  - fixed timestamps
  - stable permissions
  - explicit compression settings
- Add unit tests:
  - same inputs → identical zip bytes
  - different inputs → different manifest digest

Acceptance criteria:
- A test that generates the same runpack twice produces identical `sha256` over the zip bytes.

### Story 2.2: Signing and verification primitives

Tasks:
- Implement `core/sign`:
  - dev mode: ephemeral key generation with warnings
  - prod mode: load key by path/env; require explicit configuration
  - sign/verify functions for:
    - manifest
    - trace records
- Define a small signature envelope format (JSON) that includes:
  - `alg`, `key_id`, `sig`, `signed_digest`

Acceptance criteria:
- Offline verification of a signed manifest succeeds and tampering is detected.

### Story 2.3: CLI verify command

Tasks:
- Implement `gait verify <run_id|path>`:
  - validate zip integrity and file hashes
  - validate signatures
  - emit stable `--json` result
  - exit code `0` on success, `2` on verification failed, `6` invalid input

Acceptance criteria:
- `gait verify` is deterministic offline and returns stable JSON.

---

## Epic 3: Runpack Recording and Demo (First 5 Minutes)

Objective: deliver the viral first win: offline `gait demo` → runpack → verify.

### Story 3.1: Runpack data model and writer

Tasks:
- Implement `core/runpack`:
  - `RecordRun(...)` to produce a runpack zip with:
    - `manifest.json`
    - `run.json`
    - `intents.jsonl`
    - `results.jsonl`
    - `refs.json`
- Implement `gait run record --input <run_record.json>` as the primary capture CLI path for normalized run data.
- Enforce default “reference receipts only” capture mode in manifest.

Acceptance criteria:
- Recording produces a valid runpack zip that validates against schemas and verifies signatures.

### Story 3.2: Offline demo command

Tasks:
- Implement `gait demo`:
  - runs a deterministic toy agent simulation with exactly 3 tool calls
  - writes output to `./gait-out/` by default
  - prints:
    - `run_id=...`
    - `bundle=...`
    - ticket footer line
- Add `gait verify <run_id>` success path as part of demo output.

Acceptance criteria:
- `gait demo` completes in < 60 seconds on a laptop, offline, with no keys.

---

## Epic 4: Replay (Stub by Default) and Diff

Objective: make incidents reproducible and comparable without re-running real tools.

### Story 4.1: Stub replay engine

Tasks:
- Implement `gait run replay <run_id|path>`:
  - default: stub all tool calls using recorded results
  - must be deterministic and offline
  - require explicit `--unsafe-real-tools` (or per-tool flags) to execute real tools
  - exit code `8` if unsafe replay requested without explicit flags

Acceptance criteria:
- Stub replay produces the same outputs across repeated invocations on the same runpack.

### Story 4.2: Deterministic diff

Tasks:
- Implement `gait run diff <run_id_A> <run_id_B>`:
  - produce stable diff JSON (sorted keys, stable ordering)
  - support `--privacy=metadata` to avoid payload diffs
  - write `diff.json` (optional) and print bounded summary

Acceptance criteria:
- Diff output is stable across OSes and repeated runs.

---

## Epic 5: Regress (Incidents Become CI Tests)

Objective: convert runpacks into fixtures and run deterministic graders with stable outputs.

### Story 5.1: Fixture initialization

Tasks:
- Implement `gait regress init --from <run_id|path>`:
  - creates `gait.yaml` in current directory
  - creates `fixtures/<fixture_name>/...` containing a pinned runpack or references to it
  - emits stable JSON and next commands
- Repo paths:
  - CLI: add `regress` subcommands under `cmd/gait/` (new file `cmd/gait/regress.go` is the reference layout)
  - Core: `core/regress/`
  - Schemas/types: `schemas/v1/regress/`, `core/schema/v1/regress/`
- Commands:
  - `gait demo` (seed a runpack for local testing; writes to `./gait-out/`)
  - `gait regress init --from <run_id|path> --json`
  - `go test ./...`

Acceptance criteria:
- Running init yields a fixture layout that can be executed by `gait regress run`.

### Story 5.2: Deterministic graders framework (v1)

Tasks:
- Implement `core/regress` graders as Go interfaces.
- Ship deterministic graders:
  - schema validation grader
  - “expected verdict / exit code” grader
  - diff-based grader with explicit tolerance rules
- Disallow non-deterministic graders in v1 unless behind an explicit opt-in flag.
- Repo paths:
  - Core: `core/regress/`
  - Testdata: `core/regress/testdata/` (goldens and fixtures)
  - Shared helpers: `internal/testutil/` (introduced in Epic 10.2)
- Commands:
  - `go test ./core/regress/...`
  - `gait regress run --json` (once Story 5.1 is complete)

Acceptance criteria:
- `gait regress run` produces `regress_result.json` and stable exit codes:
  - `0` success, `5` regress failed, `6` invalid input

### Story 5.3: CI integration outputs

Tasks:
- Add optional JUnit output:
  - `--junit=./junit.xml`
- Ensure output is stable and bounded.
- Repo paths:
  - CLI: `cmd/gait/` (`regress run` flags + output wiring)
  - Core: `core/regress/` (reporters/formatters)
- Commands:
  - `gait regress run --junit=./junit.xml --json`

Acceptance criteria:
- CI can fail deterministically on meaningful drift and provide machine-readable reports.

---

## Epic 6: Gate (Runtime Enforcement) + Approvals + Traces

Objective: enforce fail-closed tool-call policy at the execution boundary.

### Story 6.1: Intent normalization and hashing

Tasks:
- Define `IntentRequest` (schema + Go type) with:
  - tool name
  - structured args
  - declared targets/destinations (for high-risk tools, policy must be able to match on explicit targets, not free-form text)
  - optional arg provenance metadata (user vs tool output vs external content), kept lightweight and deterministic in v1
  - env/context fields (identity, workspace, risk class, etc.)
  - normalized form used for hashing
- Implement normalization rules once in Go and test with fixtures.
- Repo paths:
  - Schemas/types: `schemas/v1/gate/intent_request.schema.json`, `core/schema/v1/gate/`
  - Core: `core/gate/` (normalization + hashing; policy evaluation must not live in Python)
- Commands:
  - `go test ./core/gate/...` (fixtures must pass across OSes)

Acceptance criteria:
- Same semantic intent results in identical normalized digest.

### Story 6.2: YAML policy model and evaluator (Go-only)

Tasks:
- Implement `core/gate` policy evaluation:
  - verdicts: `allow`, `block`, `dry_run`, `require_approval`
  - reason codes + violations list
  - deterministic evaluation for a given policy+intent
  - policies evaluate only structured intent fields (tool, args, targets, provenance, and context). Prompts and retrieved content are never policy inputs.
  - fail-closed option for high-risk tools when required fields (policy, targets, provenance) are missing or cannot be evaluated
- Implement `gait gate eval --policy <file> --intent <file>` (CLI helper).
- Repo paths:
  - CLI: add `gate` subcommands under `cmd/gait/` (new file `cmd/gait/gate.go` is the reference layout)
  - Core: `core/gate/` (parser + evaluator)
  - Schemas/types: `schemas/v1/gate/`, `core/schema/v1/gate/`
- Commands:
  - `gait gate eval --policy ./policy.yaml --intent ./intent.json --json`
  - `go test ./core/gate/...`

Acceptance criteria:
- Policy evaluation is deterministic and produces stable JSON.
- Fail-closed behavior can be enabled for high-risk tools.

### Story 6.3: Trace record emission and signing

Tasks:
- For every gate decision, emit `trace_<trace_id>.json`:
  - include policy digest, intent digest, verdict, latency, optional approval ref
  - sign the trace record
- Provide `gait trace verify <path>` helper (optional in v1; required by v1.1 evidence packs).
- Repo paths:
  - CLI: add `trace` helpers under `cmd/gait/` (new file `cmd/gait/trace.go` is the reference layout)
  - Core: `core/gate/` (trace emission) and `core/sign/` (sign/verify)
  - Schemas/types: `schemas/v1/gate/trace_record.schema.json`, `core/schema/v1/gate/`
- Commands:
  - `gait trace verify ./trace_<trace_id>.json --json`

Acceptance criteria:
- Trace record verification detects tampering offline.

### Story 6.4: Approval tokens

Tasks:
- Implement `gait approve` to mint scoped approval tokens:
  - binds to intent digest + policy digest + TTL + scope
  - includes approver identity (as configured) and reason code
- Gate validates token before allowing execution when verdict requires approval.
- Repo paths:
  - CLI: add `approve` under `cmd/gait/` (new file `cmd/gait/approve.go` is the reference layout)
  - Core: `core/gate/` (token mint/verify) and `core/sign/`
- Commands:
  - `gait approve --intent-digest ... --policy-digest ... --ttl 1h --json`

Acceptance criteria:
- Expired or mismatched tokens are rejected deterministically with stable error codes.

---

## Epic 7: Policy Test (Security Review and Rollout)

Objective: enable policy authoring and rollout without production risk.

### Story 7.1: CLI policy test command

Tasks:
- Implement `gait policy test <policy.yaml> <intent_fixture.json>`:
  - deterministic evaluation
  - stable JSON output (and bounded summary)
  - exit codes: `0` allow, `3` block, `4` approval required, `6` invalid input
- Repo paths:
  - CLI: add `policy` subcommands under `cmd/gait/` (new file `cmd/gait/policy.go` is the reference layout)
  - Core: `core/gate/` (evaluator) and `core/policytest/` (runner/reporting)
  - Schemas/types: `schemas/v1/policytest/`, `core/schema/v1/policytest/`
- Commands:
  - `gait policy test ./policy.yaml ./fixtures/intent.json --json`

Acceptance criteria:
- Same inputs produce identical outputs across OSes.

---

## Epic 8: Doctor (First-5-Minutes Reliability)

Objective: eliminate onboarding friction and make failures actionable.

### Story 8.1: Environment diagnostics

Tasks:
- Implement `gait doctor`:
  - checks filesystem permissions, output dirs, key config, schema availability
  - emits stable JSON + concise summary
  - includes safe copy/paste fix commands where applicable
- Repo paths:
  - CLI: add `doctor` under `cmd/gait/` (new file `cmd/gait/doctor.go` is the reference layout)
  - Core: `core/doctor/`
- Commands:
  - `gait doctor --json`

Acceptance criteria:
- Doctor runs offline and produces stable output.
- Exit code `7` indicates a non-fixable missing dependency.

---

## Epic 9: Python SDK (Thin Wrapper) + Reference Adapter

Objective: provide a minimal adoption surface without duplicating core logic.

### Story 9.1: Python SDK skeleton

Tasks:
- Implement `sdk/python/gait/`:
  - `capture_intent(...)` to build `IntentRequest` (including optional targets and provenance metadata set by the adapter)
  - `evaluate_gate(...)` to call local `gait` (subprocess) and parse `--json`
  - `write_trace(...)` to persist trace record emitted by Go (Python must not author it)
- Add typed models mirroring the JSON schemas (generated or hand-written).
- Repo paths:
  - Python: `sdk/python/gait/`
  - Go binary boundary: `cmd/gait/` and `gait --json` outputs
- Commands:
  - `uv run -m pytest` (Python)
  - `gait gate eval --json` (Go; called by Python)

Acceptance criteria:
- Python SDK can evaluate gate decisions via local Go binary without policy logic.

### Story 9.2: Reference adapter (one high-quality path)

Tasks (v1 decision: implement fully; do not add more in v1):
- Implement a generic, framework-agnostic “tool decorator” adapter in `sdk/python/gait/` and provide a concrete example under `examples/`.
- Ensure the reference adapter demonstrates the recommended pattern for high-risk tools:
  - LLM produces typed intent
  - Gate evaluates intent (fail-closed in high-risk mode)
  - deterministic executor performs side effects (no “LLM in the loop” for privileged actions)

Acceptance criteria:
- Reference adapter supports: runpack capture, gate enforcement, and regress fixture creation.

---

## Epic 10: Test Strategy (Unit, Integration, E2E, Perf) and Coverage Enforcement

Objective: make regressions impossible to ship silently; keep trust through determinism.

### Story 10.1: Unit tests (Go + Python)

Tasks:
- Go: `*_test.go` in each `core/*` package.
- Python: `sdk/python/tests/` with `pytest`.
- Enforce coverage >= 85% in CI for:
  - Go packages under `core/...` and `cmd/gait/...`
  - Python package `sdk/python/gait`

Acceptance criteria:
- CI fails if either language coverage < 85%.

### Story 10.2: Integration tests (artifact-level)

Tasks:
- Add `internal/testutil/` for temp dirs and golden fixtures.
- Test flows:
  - record → verify → replay(stub) → diff
  - gate eval → trace verify
  - policy test exit codes

Acceptance criteria:
- Integration tests run offline and pass on all OSes in CI.

### Story 10.3: E2E tests (CLI)

Tasks:
- Add CLI tests that execute the built binary:
  - `gait demo` produces a runpack zip and ticket footer line
  - `gait verify` succeeds
- Ensure tests are deterministic and do not depend on the user’s home directory (use temp dirs).

Acceptance criteria:
- `make test-e2e` passes on all OSes in CI.

### Story 10.4: Performance and regression checks

Tasks:
- Add Go benchmarks for:
  - gate evaluation latency on typical policies
  - verify/diff runtime on typical runpacks
- Add a lightweight perf gate in CI (nightly):
  - record benchmark baselines and fail on large regressions

Acceptance criteria:
- Benchmarks exist and run deterministically; regressions are visible and actionable.

---

## Epic 11: Documentation, Examples, and Operational Playbooks

Objective: match the PRD’s OSS conversion funnel and reduce support load.

### Story 11.1: README + docs ladder

Tasks:
- `README.md` includes:
  - “Start here” and a single install path
  - `gait demo` and sample output
  - “paste into ticket” footer semantics
  - “incident → regress” walkthrough
  - “gate high-risk tools” walkthrough
  - an enterprise-facing explanation of why Gate exists (instruction/data collision and prompt-injection), with a concrete blocked example

Acceptance criteria:
- A new user can go from install → demo → verify in < 5 minutes.

### Story 11.2: Examples (offline-safe)

Tasks:
- Add `examples/` that run without secrets:
  - stub replay
  - policy test
  - regress run
- Add at least one “prompt injection” style example that demonstrates deterministic blocking at the tool boundary via Gate policy (no network required).

Acceptance criteria:
- Every example documents exact commands and expected outputs.

---

## Epic 12: Release Readiness and v1 Acceptance Checklist

Objective: meet the PRD acceptance criteria and ship a trustworthy v1.

Tasks:
- Implement a “v1 acceptance” CI job that runs:
  - `gait demo`
  - `gait verify`
  - stub replay determinism test
  - regress init + run
  - policy test flow (allow + block + require_approval)
- Confirm exit codes match PRD contract.

Acceptance criteria:
- All PRD acceptance criteria sections pass via automated checks.

---

## Definition of Done (applies to every story)

- Code is formatted and linted (`make fmt`, `make lint`).
- Tests added/updated and passing (`make test`, plus integration/e2e where relevant).
- Any new schema/artifact has:
  - JSON Schema definition under `schemas/v1/`
  - matching Go type under `core/schema/v1/`
  - validator + golden fixtures
- `--json` outputs are stable and covered by tests.
- `--explain` is supported for major user-facing commands and covered by tests.
- No new network dependencies are introduced in core workflows.
