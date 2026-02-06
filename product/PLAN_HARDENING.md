# PLAN HARDENING: Gait (Zero-Ambiguity Reliability and Resilience Plan)

Date: 2026-02-06
Source of truth: `product/PRD.md`, `product/ROADMAP.md`, `product/PLAN_v1.md`, `product/PLAN_ADOPTION.md`
Scope: minimum required hardening for graceful failure, operational resilience, and architecture guardrails for v1 through v1.5

This plan is written to be executed top-to-bottom with minimal interpretation. Each story includes concrete repo paths, commands, and acceptance criteria.

---

## Global Decisions (Locked for hardening)

- Hardening must preserve core contracts:
  - determinism
  - offline-first defaults
  - stable schemas and exit codes
  - fail-closed behavior for high-risk policy paths
- Hardening is implemented in **Go core and CLI first**; Python remains thin.
- No new hosted dependencies are required for correctness.
- Runtime and operational failures must be machine-classifiable, not only human-readable.
- All persistence paths that influence enforcement or verification must be crash-safe.
- Every hardening change must be covered by deterministic tests across Linux/macOS/Windows.

Priority classes:
- `P0`: must ship before broad production rollout
- `P1`: should ship in next milestone
- `P2`: can follow after P0/P1, but should be planned now

---

## Hardening Objectives

1. Graceful failure under invalid input, missing dependencies, partial writes, lock contention, and network instability.
2. Deterministic recovery behavior and stable error semantics for automation.
3. Operational readiness aligned with:
   - Beyond 12-Factor principles (config, telemetry, ops parity, admin processes)
   - AWS Well-Architected pillars (especially reliability, security, operational excellence)
   - Frugal architecture laws (pay for value, use managed simplicity, optimize for constraints)

---

## Current Baseline (Observed)

- Stable exit code contract already exists in CLI (`cmd/gait/verify.go` constants).
- JSON outputs exist for major commands (`--json`) and are widely tested.
- CI covers lint, unit tests, e2e, and acceptance workflows across OS matrix.
- Pre-push checks exist via `.githooks/pre-push` and pre-commit `pre-push` hooks.
- Key security controls exist:
  - explicit unsafe replay interlocks
  - signature verification
  - allowlist/pin/signature for registry remote install

Known systemic gaps to close:
- runtime/system failures are frequently mapped to `exitInvalidInput` instead of specific operational categories
- critical state/artifact writes are not uniformly atomic
- networked registry flow lacks retry/backoff policy for transient failures
- hook activation is documented but not enforced programmatically
- `product/` planning docs are ignored by `.gitignore` and are not versioned in Git

---

## Epic H0: Hardening Governance and NFR Contracts (`P0`)

Objective: make reliability expectations explicit and testable.

### Story H0.1: Define non-functional contracts

Tasks:
- Create hardening contracts doc:
  - `docs/hardening/contracts.md`
- Define explicit NFRs:
  - startup behavior
  - latency budgets for critical paths
  - failure classification categories
  - crash consistency requirements
  - deterministic retry bounds

Acceptance criteria:
- Contracts doc is referenced from `README.md` and `CONTRIBUTING.md`.
- Every hardening epic maps to at least one NFR.

### Story H0.2: Add architecture decision records (ADR) for hardening decisions

Tasks:
- Add ADR folder:
  - `docs/adr/`
- Record ADRs for:
  - error taxonomy and exit mapping
  - atomic write strategy
  - lock contention strategy
  - network retry policy

Acceptance criteria:
- Each P0/P1 hardening choice has an ADR with alternatives and rationale.

### Story H0.3: Reliability risk register

Tasks:
- Add risk register:
  - `docs/hardening/risk_register.md`
- Include severity, likelihood, mitigation owner, and target milestone.

Acceptance criteria:
- Top 10 runtime/operational failure modes are tracked with owners.

---

## Epic H1: Error Taxonomy, Exit Semantics, and Graceful Failure (`P0`)

Objective: classify failures deterministically and expose actionable machine-readable errors.

### Story H1.1: Introduce core error taxonomy

Tasks:
- Add package:
  - `core/errors/` (or `internal/errors/` if preferred)
- Define canonical categories:
  - `invalid_input`
  - `verification_failed`
  - `policy_blocked`
  - `approval_required`
  - `dependency_missing`
  - `io_failure`
  - `state_contention`
  - `network_transient`
  - `network_permanent`
  - `internal_failure`
- Add typed wrappers (`%w` compatible) and category extraction helpers.

Acceptance criteria:
- Core packages return typed/categorized errors where behavior differs.

### Story H1.2: Standardize CLI JSON error envelope

Tasks:
- Add shared CLI envelope helper in:
  - `cmd/gait/`
- Include fields:
  - `ok`
  - `error`
  - `error_code`
  - `error_category`
  - `retryable`
  - `hint`
- Replace ad-hoc error JSON formatting paths in command handlers.

Acceptance criteria:
- All major commands emit consistent error envelope under `--json`.

### Story H1.3: Refine exit code mapping for operational failures

Tasks:
- Preserve existing public contract codes; add new codes only if necessary and documented.
- Ensure non-input operational errors do not collapse into `exitInvalidInput`.
- Add tests for command-level exit code behavior under failure injection.

Repo paths:
- `cmd/gait/*.go`
- `cmd/gait/*_test.go`

Acceptance criteria:
- Failure classes map predictably to stable exits and `error_code`.

---

## Epic H2: Crash-Safe Persistence and Atomic Writes (`P0`)

Objective: remove partial-write corruption risk in state and artifact outputs.

### Story H2.1: Add shared atomic write utilities

Tasks:
- Add utility package:
  - `core/fsx/` or `internal/fsx/`
- Implement:
  - temp file write
  - flush/sync
  - atomic rename
  - permission control
- Ensure cross-platform support for Windows rename behavior.

Acceptance criteria:
- Utility has deterministic tests, including interruption simulation where possible.

### Story H2.2: Migrate critical state writes to atomic path

Targets:
- rate-limit state writes
- approval audit record writes
- credential evidence writes
- registry metadata and pin writes
- regress output and junit writes

Repo paths:
- `core/gate/rate_limit.go`
- `core/gate/approval_audit.go`
- `core/gate/credential_evidence.go`
- `core/registry/install.go`
- `core/regress/run.go`

Acceptance criteria:
- No partial file observed under induced failure tests.

### Story H2.3: Add filesystem permission and ownership assertions

Tasks:
- Add tests asserting file mode expectations for state/artifact outputs.
- Ensure no world-writable outputs.

Acceptance criteria:
- Security-sensitive outputs consistently use least-privilege file modes.

---

## Epic H3: Contention and Concurrency Hardening (`P0`)

Objective: maintain deterministic behavior under concurrent command execution.

### Story H3.1: Harden lock strategy for gate rate-limit state

Tasks:
- Extend lock handling:
  - stale lock detection
  - lock owner metadata (pid/timestamp)
  - bounded retry policy with deterministic timeout
- Improve lock timeout error classification.

Repo paths:
- `core/gate/rate_limit.go`

Acceptance criteria:
- Concurrent gate eval invocations do not corrupt state and fail gracefully when contention exceeds budget.

### Story H3.2: Add concurrent integration tests

Tasks:
- Add tests in:
  - `internal/integration/`
- Simulate multiple concurrent `gate eval` calls against shared state path.

Acceptance criteria:
- Results are deterministic and no state corruption is observed.

---

## Epic H4: Network and Registry Resilience (`P1`)

Objective: make optional remote registry behavior robust under transient failures while staying fail-closed for trust.

### Story H4.1: Add bounded retry/backoff for remote fetch

Tasks:
- Implement retry policy for `core/registry/install.go` remote fetch:
  - retry only transient status/transport errors
  - exponential backoff with jitter cap
  - max attempts deterministic and configurable

Acceptance criteria:
- transient failure tests recover within retry budget
- permanent failure errors are fast and explicit

### Story H4.2: Offline fallback behavior

Tasks:
- If remote install fails and a valid pinned cached manifest exists, expose explicit fallback option.
- Never bypass signature verification.

Acceptance criteria:
- Offline fallback is explicit, deterministic, and audit-visible in output.

### Story H4.3: Tighten remote source policy

Tasks:
- Keep host allowlist mandatory for remote.
- Add explicit `https` requirement by default (with explicit unsafe override if ever allowed).

Acceptance criteria:
- insecure source usage is blocked with clear reason codes.

---

## Epic H5: Diagnostics and Operational Observability (`P1`)

Objective: reduce MTTR with deterministic diagnostics and machine-readable operational signals.

### Story H5.1: Extend doctor with hardening checks

Tasks:
- Add checks for:
  - hooks activation (`core.hooksPath`)
  - registry cache health
  - lock file staleness
  - writable temp directory
  - key source ambiguity/misconfiguration
- Include fix commands where safe.

Repo paths:
- `core/doctor/doctor.go`
- `cmd/gait/doctor.go`

Acceptance criteria:
- `gait doctor --json` reports these checks with actionable fixes.

### Story H5.2: Add structured local operational event logs (opt-in)

Tasks:
- Add optional event sink:
  - command start/end
  - exit code
  - error category
  - elapsed time
- Keep default disabled (privacy and offline-first).

Acceptance criteria:
- Event schema is stable and validated.

### Story H5.3: Correlation identifiers

Tasks:
- Add per-command correlation IDs surfaced in JSON outputs and logs.
- Propagate ID through gate trace and related outputs where applicable.

Acceptance criteria:
- Operators can correlate command output, trace, and error records deterministically.

---

## Epic H6: CI and Verification Hardening (`P0`)

Objective: enforce hardening guarantees continuously and prevent regressions.

### Story H6.1: Add hardening test suite target

Tasks:
- Add `make test-hardening` target.
- Include:
  - failure injection tests
  - contention tests
  - atomic write integrity tests
  - exit-code contract tests

Acceptance criteria:
- CI runs hardening suite on at least Linux; nightly on full OS matrix.

### Story H6.2: Add deterministic golden tests for error envelopes

Tasks:
- Add fixtures for command failure JSON outputs under:
  - `cmd/gait/testdata/`
- Validate stable `error_code`, `category`, and exit code.

Acceptance criteria:
- Any accidental error-surface change fails tests.

### Story H6.3: Hook enforcement check

Tasks:
- Add lint check that warns/fails when `.githooks` is present but not configured.
- Add clear remediation command (`make hooks`).

Acceptance criteria:
- Local developer flows consistently run required pre-push checks.

---

## Epic H7: Supply Chain and Release Integrity Hardening (`P1`)

Objective: reduce release-time risk and improve provenance trust.

### Story H7.1: Pin release workflow tool versions

Tasks:
- Replace floating `@latest` installer usage where feasible with pinned versions.
- Maintain versions in one place for release pipeline reproducibility.

Repo paths:
- `.github/workflows/release.yml`
- `Makefile` (if central version vars are added)

Acceptance criteria:
- release workflow is reproducible and less susceptible to upstream breaking changes.

### Story H7.2: Verify released artifacts in CI acceptance flow

Tasks:
- Add a post-build verification step in release pipeline:
  - verify checksum
  - verify signature/provenance outputs

Acceptance criteria:
- release job fails fast if integrity artifacts are malformed or unverifiable.

---

## Epic H8: Security and Secrets Runtime Hardening (`P1`)

Objective: tighten runtime security behavior under realistic operator mistakes.

### Story H8.1: Key source precedence and ambiguity checks

Tasks:
- Define strict precedence for `--key`, `--key-env`, and mode handling.
- Block ambiguous combinations with deterministic errors.

Acceptance criteria:
- Misconfigured key flags fail with explicit error codes and hints.

### Story H8.2: Credential broker command safety controls

Tasks:
- Add explicit allowlist/docs for command broker usage.
- Add timeout and output-size guard tests.
- Ensure errors never leak sensitive token values.

Repo paths:
- `core/credential/providers.go`
- `core/credential/providers_test.go`

Acceptance criteria:
- command broker failures are safe, bounded, and non-leaky.

### Story H8.3: Redaction conformance tests

Tasks:
- Add tests asserting sensitive payloads are not included in default outputs.

Acceptance criteria:
- default-safe recording posture is enforced by tests.

---

## Epic H9: Repo and Process Hygiene for Hardening (`P0`)

Objective: keep hardening governance auditable and enforceable.

### Story H9.2: Add hardening review checklist to PR template

Tasks:
- Extend `.github/pull_request_template.md` with checks:
  - failure classification impact
  - exit code changes
  - deterministic output impact
  - security implications

Acceptance criteria:
- PRs changing core runtime paths include explicit hardening review responses.

### Story H9.3: Release readiness checklist

Tasks:
- Add hardening gate checklist in:
  - `docs/hardening/release_checklist.md`

Acceptance criteria:
- releases are blocked when mandatory hardening gates are not met.

---

## Epic H10: Framework Alignment (Beyond 12-Factor, AWS WA, Frugal) (`P1`)

Objective: convert framework guidance into implementation checklists and guardrails.

### Story H10.1: Beyond 12-Factor alignment matrix

Tasks:
- Create matrix:
  - `docs/hardening/beyond12factor_matrix.md`
- Cover at minimum:
  - dependency declaration/isolation
  - config and secret handling
  - telemetry and event streams
  - admin processes and operational parity
  - API-first contracts and backward compatibility

Acceptance criteria:
- matrix maps each factor to concrete repo controls and open gaps.

### Story H10.2: AWS Well-Architected mapping

Tasks:
- Create matrix:
  - `docs/hardening/aws_well_architected_matrix.md`
- Map controls to six pillars:
  - operational excellence
  - security
  - reliability
  - performance efficiency
  - cost optimization
  - sustainability

Acceptance criteria:
- each pillar has at least one implemented control and one tracked gap (if any).

### Story H10.3: Frugal architecture mapping

Tasks:
- Create matrix:
  - `docs/hardening/frugal_architecture_matrix.md`
- Map current design and planned controls to frugal laws/principles.

Acceptance criteria:
- map includes practical “keep/do/not-do” decisions for the next two milestones.

---

## Epic H11: Performance and Resource Resilience (`P2`)

Objective: avoid reliability regressions from resource pressure.

### Story H11.1: Resource budget definitions

Tasks:
- Define memory/CPU/IO budgets for key commands:
  - verify
  - gate eval
  - regress run
  - guard pack

Acceptance criteria:
- budgets documented and benchmarked in CI/nightly where feasible.

### Story H11.2: Large input stress tests

Tasks:
- Add stress tests for:
  - large runpacks within configured limits
  - high JSONL record counts
  - concurrent registry operations

Acceptance criteria:
- stress tests demonstrate graceful degradation and bounded failures.

---

## Epic H12: Hardening Acceptance Gate (`P0`)

Objective: make hardening completion verifiable and release-blocking.

Tasks:
- Add hardening acceptance script:
  - `scripts/test_hardening_acceptance.sh`
- Add CI job:
  - `.github/workflows/ci.yml` or dedicated workflow
- Validate:
  - error taxonomy coverage
  - deterministic error envelopes
  - atomic write integrity under failure simulation
  - lock contention behavior
  - network retry behavior classification
  - hooks enforcement check

Acceptance criteria:
- hardening acceptance checks pass in CI before release tagging.

---

## Minimum-Now Sequence (6-Week Execution)

### Weeks 1-2 (P0 core)

- Complete H0, H1, H2 foundations.
- Implement taxonomy + CLI envelope + atomic write utilities.
- Add initial hardening tests.

### Weeks 3-4 (P0 completion)

- Complete H3 and H6.
- Add contention tests, hook enforcement, and hardening CI target.
- Complete H9.1 (`product/` versioning hygiene).

### Weeks 5-6 (P1 close-in)

- Complete H4 and H5.
- Add registry retry/backoff and enhanced doctor diagnostics.
- Start H10 matrices for architecture governance.

Success criteria at end of 6 weeks:
- No critical path writes remain non-atomic.
- Operational failures are machine-classified and not misreported as input errors.
- Hardening acceptance checks run in CI.

---

## Definition of Done (Hardening Stories)

- Includes deterministic tests for both success and failure paths.
- Preserves backward compatibility for public `--json` outputs unless explicitly versioned.
- Does not weaken offline-first defaults.
- Documents operator-facing behavior and remediation steps.
- Adds/updates acceptance checks where reliability posture changes.

