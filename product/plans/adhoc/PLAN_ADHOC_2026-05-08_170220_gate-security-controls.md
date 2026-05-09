# PLAN ADHOC: Gate Security Controls

Date: 2026-05-08
Source: user-provided recommendations for high-risk credential posture, freeze windows, kill switches, policy explain, broker recipes, proof bundles, trust graduation, and sandbox metadata
Profile: `gait`
Scope: Gate policy controls and proof surfaces for high-risk agent tool calls
Plan path: `product/plans/adhoc/PLAN_ADHOC_2026-05-08_170220_gate-security-controls.md`

This plan converts the supplied recommendations into an execution-ready backlog.
It does not overwrite `product/PLAN_NEXT.md` or any rolling plan file.

## Global Decisions (Locked)

- Go remains authoritative for policy parsing, evaluation, fail-closed behavior,
  schemas, canonicalization, signing, proof packing, trace emission, CLI JSON,
  and state-file validation.
- Python remains an adoption layer only. It may pass structured intent metadata
  to `gait`, but it must not decide credential, freeze-window, sandbox,
  kill-switch, or approval outcomes.
- High-risk and production policy changes default to fail closed. Missing or
  unreadable required state blocks when a strict policy or `oss-prod` profile
  depends on that state.
- Credential controls must record references, issuer/source/access type, TTL,
  scope, target/run/job binding, and request digests. They must not record raw
  credential material.
- Freeze-window evaluation must be deterministic. Tests use explicit evaluation
  times, normalize comparisons to UTC, and use IANA timezones only from policy
  config.
- Local state files for kill switches and other policy state must be atomically
  read/written, schema validated, and safe under concurrent readers.
- Structured explain JSON is a stable public contract and must be schema-backed
  before downstream docs or examples claim machine-readable explain support.
- All new artifacts are additive within `schemas/v1/` unless an explicit
  migration and compatibility test proves otherwise.
- Docs and examples remain offline-first and stub-safe by default. Provider
  broker recipes may describe real providers, but tests use deterministic
  stubs and do not require network or cloud accounts.

## Current Baseline (Observed)

Observed command evidence:

- `gait doctor --json`
- Result: `ok=true`, `status=warn`, failed checks `0`, warnings `1`.
- Warning: local registry cache is not initialized. Workdir, output directory,
  job runtime durability, temp directory, rate-limit lock, binary readiness, key
  source configuration, repo schema files, hooks path, and onboarding assets
  passed.

Observed implementation baseline:

- `core/gate/policy.go` already defines rule fields for
  `block_standing_credentials`, `require_jit_credential`,
  `max_credential_ttl_seconds`, allowed credential sources/issuers/access
  types, broker references/scopes, approvals, delegation, context evidence,
  rate limits, destructive budgets, endpoint policy, and dataflow policy.
- `schemas/v1/gate/policy.schema.json` already includes the credential and
  broker policy fields, but it has no `freeze_window` config and no kill-switch
  policy/state contract.
- `core/gate/credential_policy.go` blocks missing, unknown, static, standing,
  inherited, and cloud-admin credential postures when policy enables those
  checks, and validates target/run/job credential bindings.
- `core/gate/broker_receipt.go` validates broker request digests, scope, TTL,
  JIT access type, source, issuer, credential ref, and target/run/job binding.
- `cmd/gait/policy_templates/baseline-highrisk.yaml` currently requires
  approvals and broker credentials for writes, but it does not yet make JIT
  credential posture the default high-risk example.
- `cmd/gait/explain.go` currently emits prose-only explain output. `gait gate
  eval --explain --json` is not yet a stable machine-readable contract.
- `core/jobruntime/runtime.go` supports job-level emergency stop. `cmd/gait/mcp.go`
  checks stopped `job_id` for MCP dispatch, but Gate does not yet have
  generalized agent/tool/path/environment kill-switch state.
- `core/schema/v1/gate/types.go` and `schemas/v1/gate/intent_request.schema.json`
  model intent targets and credential context, but there is no first-class
  sandbox metadata object for `proc.exec` policy enforcement.
- `core/credential/broker.go` and `core/credential/providers.go` support stub,
  env, and command brokers with bounded command output and privacy-safe refs,
  but provider-oriented broker recipes and adapters are not yet a documented
  product surface.
- `docs/contracts/` already contains contracts for action context, broker
  receipts, credential provenance, artifact graph, and related primitives. It
  does not yet include contracts for freeze windows, kill switches, sandbox
  policy, policy explain JSON, or authorization bundles.

## Exit Criteria

- The high-risk policy template blocks standing/static credentials by default
  for covered write, deploy, and destructive paths, and allows only scoped,
  target-bound, valid-TTL JIT credentials from configured issuers/sources.
- Freeze-window policy can block or require approval for production-impacting
  actions with deterministic timezone and evaluation-time behavior.
- Generalized kill switches can stop matching agent IDs, identities, tools,
  targets, paths, and environments immediately, with stable reason codes and
  fail-closed behavior when strict state is unavailable.
- `gait gate eval --explain --json` emits schema-validated JSON covering
  verdict, matched rules, missing fields, approvals, credential posture,
  broker/JIT state, freeze-window state, kill-switch state, sandbox state,
  fail-closed reasons, and proof refs.
- Broker recipes exist for AWS STS, GitHub OIDC, Vault, GCP/Azure federation,
  and Okta/CyberArk-style brokering without storing raw secrets or requiring
  network in tests.
- A named authorization proof bundle links decision trace, policy digest,
  intent digest, approval audit, credential evidence, freeze/kill/sandbox
  evidence, action outcome, and verification metadata, and verifies offline.
- Trust graduation has named templates and fixtures for observe, dry-run,
  read-only allow, approval-gated write, brokered write, and blocked
  destructive stages.
- High-risk `proc.exec` or generated-code execution blocks when sandbox
  metadata is missing, malformed, expired, or too permissive for the active
  policy.

## Public API and Contract Map

- CLI contracts:
  - `gait gate eval --policy <policy.yaml> --intent <intent.json> --json`
  - `gait gate eval --policy <policy.yaml> --intent <intent.json> --explain --json`
  - `gait gate eval ... --kill-switch-state <state.json> --json`
  - `gait gate eval ... --evaluation-time <rfc3339> --json` for deterministic
    freeze-window tests and replay.
  - New kill-switch management command under `gait kill-switch ... --json` or
    `gait gate kill-switch ... --json`, with final command naming locked by
    command tests before implementation.
  - New authorization bundle command under `gait pack ... --profile gate-authorization`
    or a narrowly scoped `gait gate bundle ... --json`; pick one command path in
    the first implementation PR and document it everywhere.
- Schema contracts:
  - `schemas/v1/gate/policy.schema.json`
  - `schemas/v1/gate/intent_request.schema.json`
  - `schemas/v1/gate/gate_result.schema.json`
  - `schemas/v1/gate/trace_record.schema.json`
  - `schemas/v1/gate/broker_credential_record.schema.json`
  - New `schemas/v1/gate/policy_explain.schema.json`
  - New `schemas/v1/gate/kill_switch_state.schema.json`
  - New `schemas/v1/gate/authorization_bundle.schema.json`
  - New sandbox metadata fields or schema under `schemas/v1/gate/`
- Go contracts:
  - `core/gate/` owns freeze-window, kill-switch, credential, broker, sandbox,
    explain, reason-code ordering, and fail-closed policy decisions.
  - `core/credential/` owns broker request/response normalization and
    provider-adapter safety.
  - `core/pack/` owns authorization bundle packaging, manifest hashing,
    verification, and tamper detection.
  - `cmd/gait/` owns stable CLI flags, help text, JSON output, and exit codes.
  - `core/schema/v1/gate/` owns public Go structs for new schema-backed
    artifacts.
- Documentation contracts:
  - Public behavior changes update `README.md`, `docs/`,
    `docs-site/public/llms.txt`, `docs-site/public/llm/`, examples, and
    `CHANGELOG.md` where relevant.
  - Contract docs are added for freeze windows, kill switches, structured
    policy explain, authorization bundles, credential brokers, sandbox policy,
    and trust graduation.

## Docs and OSS Readiness Baseline

- First-screen docs must keep the execution-boundary story clear: define policy,
  evaluate structured intent, enforce or simulate safely, and verify artifacts
  offline.
- Examples must be copy-pasteable without cloud accounts. Provider broker
  examples can include real command shape, but default fixtures use local
  deterministic receipts.
- Docs must not claim provider credentials are issued by Gait. Gait validates
  broker receipts, credential refs, policy state, and proof linkage.
- Any new JSON output must be shown with short examples and schema references,
  not only prose.
- Docs and examples must avoid raw secrets, raw tokens, private keys, real
  customer data, and real destructive targets.

## Recommendation Traceability

| User recommendation | Planned coverage |
|---|---|
| 1. Default High-Risk Credential Posture | Story GSC1.1 |
| 2. Freeze Window Policy | Story GSC1.2 |
| 3. Generalized Kill Switch | Story GSC2.1 |
| 4. Structured Policy Explain | Story GSC2.2 |
| 5. First-Class Broker Recipes | Story GSC3.1 |
| 6. Authorization Proof Bundle | Story GSC3.2 |
| 7. Trust Graduation Controls | Story GSC4.1 |
| 8. Sandbox Metadata For `proc.exec` | Story GSC1.3 |

## Test Matrix Wiring

- Fast lane:
  - `make lint-fast`
  - `make test-fast`
- Core CI lane:
  - targeted Go tests under `core/gate`, `core/credential`, `core/pack`,
    `core/schema`, `core/jobruntime`, and `cmd/gait`
  - `make test-contracts`
- Acceptance lane:
  - `make test-scenarios`
  - offline policy fixture suite for static credential block, JIT allow, active
    freeze block, kill-switch block, sandbox block, and authorization bundle
    verification
- Cross-platform lane:
  - GitHub Actions OS matrix for Linux, macOS, and Windows
  - state-file tests use `t.TempDir()` and portable paths
- Risk lane:
  - `make test-hardening`
  - `make test-chaos` for unreadable state, concurrent updates, malformed
    schema, missing broker receipts, and tamper scenarios
  - `make test-runtime-slo` when explain or state lookups affect hot-path
    evaluation latency
- Release/UAT lane:
  - `make prepush-full`
  - release smoke only when CLI flags, public templates, or proof bundle command
    behavior are release-facing
- Gating rule:
  - Stories that change schemas, CLI JSON, exit codes, proof verification, Gate
    verdicts, or fail-closed behavior require targeted failing tests first,
    golden fixtures, `make test-contracts`, and `make prepush-full` before
    implementation handoff.

## Minimum-Now Sequence

1. Tighten the high-risk credential posture in the default template and golden
   policy fixtures.
2. Add sandbox metadata requirements for high-risk `proc.exec` before expanding
   broad execution examples.
3. Add freeze-window policy with deterministic evaluation-time tests.
4. Add generalized kill-switch state and CLI/Gate/MCP enforcement.
5. Add structured policy explain JSON that exposes credential, freeze,
   kill-switch, sandbox, approval, and proof states.
6. Add broker recipes and provider receipt adapters using deterministic stubs.
7. Add the authorization proof bundle profile and offline verify path.
8. Add trust graduation docs, fixtures, and templates after the enforcement
   primitives are in place.

## Wave 1: P0 Enforcement Defaults

### Story GSC1.1: Default high-risk credential posture

Recommendation:
Make the high-risk default policy block standing/static credentials and require
JIT brokered credentials for covered high-risk write, deploy, and destructive
paths.

Why:
The credential primitives exist, but the default policy template does not yet
make "replace standing privilege with JIT control" the operator's default demo
posture.

Strategic direction:
Turn existing credential policy fields into an opinionated high-risk baseline
and prove the block/allow cases with golden policy fixtures.

Expected benefit:
Security reviewers can run the baseline template and immediately see static PAT,
cloud-admin, inherited, and unknown credentials block while STS/OIDC/Vault-style
JIT credentials allow only when scoped and bounded.

Tasks:

- Update `cmd/gait/policy_templates/baseline-highrisk.yaml` high-risk write,
  deploy, and destructive rules with `block_standing_credentials: true`,
  `require_jit_credential: true`, `max_credential_ttl_seconds`,
  `allowed_credential_sources`, `allowed_credential_issuers`,
  `allowed_credential_access_types: [jit]`, and broker scopes.
- Add or update high-risk credential fixture intents under `examples/policy/`
  for GitHub PAT/static, AWS IAM user/cloud-admin, inherited env credential,
  AWS STS, GitHub OIDC, Vault dynamic, and unknown provenance.
- Add golden tests in `core/gate/credential_policy_test.go`,
  `core/gate/broker_receipt_test.go`, and command tests under `cmd/gait/` that
  assert stable verdicts, reason codes, matched rule names, and exit codes.
- Ensure `schemas/v1/gate/policy.schema.json` and
  `schemas/v1/gate/broker_credential_record.schema.json` keep the credential
  fields documented and validation-compatible.
- Update `docs/contracts/credential_provenance.md`,
  `docs/contracts/broker_receipt.md`, `docs/policy_authoring.md`, README
  snippets, docs-site LLM exports, and `CHANGELOG.md`.

Repo paths:

- `cmd/gait/policy_templates/baseline-highrisk.yaml`
- `core/gate/policy.go`
- `core/gate/credential_policy.go`
- `core/gate/broker_receipt.go`
- `core/gate/credential_evidence.go`
- `schemas/v1/gate/policy.schema.json`
- `schemas/v1/gate/broker_credential_record.schema.json`
- `examples/policy/credentials/`
- `docs/contracts/credential_provenance.md`
- `docs/contracts/broker_receipt.md`
- `docs/policy_authoring.md`
- `CHANGELOG.md`

Run commands:

- `go test ./core/gate ./core/credential ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make prepush-full`

Test requirements:

- Static PAT blocks with `standing_credential_disallowed` or
  `credential_not_jit`.
- Cloud-admin and inherited credentials block under the high-risk baseline.
- Missing credential evidence blocks with stable reason codes.
- AWS STS, GitHub OIDC, and Vault dynamic fixtures allow only with JIT access
  type, allowed issuer/source, valid TTL, scope, request digest, target binding,
  and run/job binding.
- Raw credential-like args remain rejected and absent from traces.

Matrix wiring:
Fast lane for targeted Gate tests; core CI lane for command/schema contracts;
risk lane for fail-closed and broker receipt mismatch cases; cross-platform lane
for fixture path portability.

Acceptance criteria:

- `baseline-highrisk.yaml` demonstrates JIT-only high-risk write/deploy/destruct
  paths.
- Golden fixtures cover PAT/static/cloud-admin block and STS/OIDC/Vault-style
  JIT allow.
- Existing policies without the new default template fields remain
  backward-compatible.
- Changelog and docs describe the operator-facing default change.

Changelog impact: required
Changelog section: Security
Draft changelog entry: Tightened the high-risk baseline policy to block standing or static credentials and require scoped JIT brokered credentials for covered write, deploy, and destructive actions.
Semver marker override: [semver:minor]
Contract/API impact: additive template, fixture, docs, and reason-code coverage using existing policy fields.
Versioning/migration impact: no schema break; template behavior changes only when users regenerate or adopt the high-risk baseline.
Architecture constraints: Gate policy evaluation stays in Go core; brokers emit refs and receipts, never raw credentials.
ADR required: no
TDD first failing test(s): PAT block, cloud-admin block, missing credential block, STS allow, GitHub OIDC allow, Vault dynamic allow.
Cost/perf impact: low
Chaos/failure hypothesis: malformed, missing, expired, or mismatched broker receipt must block in strict high-risk evaluation without leaking secret material.

### Story GSC1.2: Freeze-window policy

Recommendation:
Add first-class freeze-window rules for production-impacting actions.

Why:
The product story requires answering whether an active freeze window applies, and
Gate currently has no native freeze-window primitive.

Strategic direction:
Represent freeze windows as typed policy configuration evaluated by Go core with
explicit time inputs in tests and UTC-normalized comparisons.

Expected benefit:
Production-impacting actions can block or require approval during release
freezes, incident freezes, quarter-end freezes, or operator-defined windows.

Tasks:

- Add a `freeze_window` rule config to `core/gate/policy.go` with timezone,
  windows, optional reason, effect `block|require_approval`, target
  environments, and target risk classes.
- Add `core/gate/freeze_window.go` and `core/gate/freeze_window_test.go` for
  deterministic window evaluation, timezone loading, UTC normalization, and
  reason-code generation.
- Add schema support in `schemas/v1/gate/policy.schema.json`, plus Go schema
  types if public artifact structs are needed.
- Add `--evaluation-time <rfc3339>` or equivalent deterministic test hook to
  `cmd/gait/gate.go` for `gate eval`, with command tests proving replayable
  output.
- Add freeze-window state to trace and explain outputs where policy evaluation
  needs proof refs or matched-window details.
- Add `docs/contracts/freeze_windows.md` and policy examples for production
  deploy block and require-approval modes.

Repo paths:

- `core/gate/policy.go`
- `core/gate/freeze_window.go`
- `core/gate/freeze_window_test.go`
- `cmd/gait/gate.go`
- `schemas/v1/gate/policy.schema.json`
- `schemas/v1/gate/trace_record.schema.json`
- `docs/contracts/freeze_windows.md`
- `examples/policy/freeze_windows/`
- `CHANGELOG.md`

Run commands:

- `go test ./core/gate ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make prepush-full`

Test requirements:

- Fixed evaluation time inside active freeze blocks.
- Fixed evaluation time outside window preserves normal verdict.
- `require_approval` effect returns approval-gated verdict with stable reason.
- IANA timezone conversion handles DST-boundary fixture deterministically.
- Unknown timezone, malformed window, missing evaluation state in strict mode,
  or unsupported effect fails closed.

Matrix wiring:
Fast lane for freeze evaluator unit tests; core CI lane for CLI/schema/golden
contracts; risk lane for malformed window and missing strict state; cross-platform
lane because timezone behavior must be portable.

Acceptance criteria:

- Policies can express active freeze windows for specific environments and risk
  classes.
- Tests use explicit times and do not depend on wall-clock time.
- Gate output includes stable freeze-window reason codes and matched rule data.
- Docs include block and require-approval examples.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added Gate freeze-window policy rules for deterministic production-impacting action blocks or approval gates.
Semver marker override: [semver:minor]
Contract/API impact: additive policy schema, CLI test-time flag, trace/explain fields, and reason codes.
Versioning/migration impact: additive; existing policies without `freeze_window` are unchanged.
Architecture constraints: freeze evaluation happens in Go core using structured policy and explicit evaluation time in tests.
ADR required: yes
TDD first failing test(s): active block, active require approval, inactive allow, DST boundary, invalid timezone fail closed.
Cost/perf impact: low
Chaos/failure hypothesis: ambiguous or invalid freeze-window config in production/high-risk mode must block rather than silently ignore the freeze state.

### Story GSC1.3: Sandbox metadata for `proc.exec`

Recommendation:
Require sandbox metadata before allowing generated-code execution or process
execution.

Why:
`proc.exec` is recognized as an endpoint class, but Gate does not yet enforce
network mode, writable paths, environment exposure, timeout, or filesystem
isolation.

Strategic direction:
Make sandbox posture a structured intent and policy input for high-risk
execution rather than a prose assumption in wrappers or docs.

Expected benefit:
High-risk execution can be blocked unless the caller proves it is bounded by
policy-approved network, filesystem, environment, timeout, and isolation
settings.

Tasks:

- Add `sandbox` metadata to `core/schema/v1/gate/types.go` and
  `schemas/v1/gate/intent_request.schema.json`, with fields for network mode,
  writable paths, read-only roots, env exposure mode, timeout seconds,
  filesystem isolation, user/privilege mode, and sandbox evidence ref/digest.
- Normalize sandbox metadata in `core/gate/intent.go` and reject raw environment
  values or secret-like sandbox metadata.
- Add policy requirements in `core/gate/policy.go` and
  `core/gate/sandbox_policy.go` for `proc.exec` and generated-code execution.
- Add trace/explain fields or proof refs that record sandbox posture digest,
  not raw environment data.
- Add docs and examples under `docs/contracts/sandbox_policy.md` and
  `examples/policy/sandbox/`.

Repo paths:

- `core/gate/intent.go`
- `core/gate/policy.go`
- `core/gate/sandbox_policy.go`
- `core/gate/sandbox_policy_test.go`
- `core/schema/v1/gate/types.go`
- `schemas/v1/gate/intent_request.schema.json`
- `schemas/v1/gate/policy.schema.json`
- `schemas/v1/gate/trace_record.schema.json`
- `docs/contracts/sandbox_policy.md`
- `examples/policy/sandbox/`
- `CHANGELOG.md`

Run commands:

- `go test ./core/gate ./core/schema/... ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make test-chaos`
- `make prepush-full`

Test requirements:

- High-risk `proc.exec` without sandbox metadata blocks.
- Network mode broader than policy allows blocks.
- Writable path outside allowed prefixes blocks.
- Raw env exposure blocks unless policy explicitly allows a safe mode.
- Timeout above maximum blocks.
- Valid sandbox metadata allows when other policy requirements pass.

Matrix wiring:
Fast lane for sandbox policy tests; core CI lane for schema/CLI contracts; risk
lane for fail-closed malformed metadata; cross-platform lane for path handling.

Acceptance criteria:

- `proc.exec` and generated-code actions have policy-checkable sandbox posture.
- Strict high-risk policies fail closed when sandbox metadata is missing or too
  permissive.
- Trace/explain/proof surfaces expose sandbox status without raw secret data.
- Docs clearly state Gait verifies metadata and receipts, not OS sandbox runtime
  implementation.

Changelog impact: required
Changelog section: Security
Draft changelog entry: Added sandbox metadata policy enforcement for high-risk `proc.exec` and generated-code actions.
Semver marker override: [semver:minor]
Contract/API impact: additive intent schema, policy schema, trace/explain fields, and reason codes.
Versioning/migration impact: additive; existing intents remain valid unless a policy requires sandbox metadata.
Architecture constraints: Go core evaluates sandbox evidence; wrappers cannot certify sandbox sufficiency.
ADR required: yes
TDD first failing test(s): missing sandbox block, permissive network block, unsafe writable path block, env exposure block, valid sandbox allow.
Cost/perf impact: low
Chaos/failure hypothesis: malformed sandbox metadata in `oss-prod` must block and must not serialize raw environment contents.

## Wave 2: P0/P1 Runtime Stops And Explainability

### Story GSC2.1: Generalized kill switch

Recommendation:
Extend emergency stop from job-only to agent, identity, tool, target, path, and
environment kill switches.

Why:
Current emergency stop blocks MCP calls only for stopped `job_id`; operators
need immediate stops for covered agents, tools, paths, or environments.

Strategic direction:
Add a local schema-backed kill-switch state file evaluated by Gate and shared
with MCP surfaces, while preserving job-level emergency stop behavior.

Expected benefit:
Security and platform teams can stop risky execution paths immediately without
editing policy files or waiting for code redeploys.

Tasks:

- Add `schemas/v1/gate/kill_switch_state.schema.json` with entries keyed by
  `agent_id`, `identity`, `tool_name`, `target_kind`, `target_value`,
  `environment`, optional path/workspace prefixes, reason, actor, created time,
  expiry, and enabled flag.
- Add `core/gate/kill_switch.go` and tests for state loading, schema validation,
  matching, expiry, deterministic reason codes, and fail-closed unavailable
  state.
- Add `--kill-switch-state <state.json>` to `gait gate eval`, project config
  defaults, and MCP proxy evaluation where policy/profile requires the state.
- Add `cmd/gait/kill_switch.go` or a `gait gate kill-switch` subcommand for
  add/list/disable/expire operations with JSON output and atomic writes.
- Preserve existing job emergency stop behavior in `core/jobruntime/runtime.go`
  and reuse reason-code conventions where sensible.
- Add docs and examples under `docs/contracts/kill_switch.md` and
  `examples/policy/kill_switch/`.

Repo paths:

- `core/gate/kill_switch.go`
- `core/gate/kill_switch_test.go`
- `cmd/gait/gate.go`
- `cmd/gait/kill_switch.go`
- `cmd/gait/mcp.go`
- `core/jobruntime/runtime.go`
- `schemas/v1/gate/kill_switch_state.schema.json`
- `docs/contracts/kill_switch.md`
- `examples/policy/kill_switch/`
- `CHANGELOG.md`

Run commands:

- `go test ./core/gate ./core/jobruntime ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make test-chaos`
- `make prepush-full`

Test requirements:

- Matching `agent_id`, identity, tool, target, path, and environment entries
  block with stable reason codes.
- Expired and disabled entries do not block.
- Unreadable or invalid state blocks in `oss-prod` or strict high-risk mode.
- Atomic state updates preserve schema validity.
- MCP still blocks job emergency stop and additionally blocks generalized
  kill-switch matches.

Matrix wiring:
Fast lane for matcher tests; core CI lane for CLI/schema contracts; risk lane
for unavailable state and concurrency; cross-platform lane for atomic state file
handling.

Acceptance criteria:

- Gate and MCP share generalized kill-switch semantics.
- Operators have JSON CLI output for kill-switch state management.
- Strict profiles fail closed when required kill-switch state cannot be loaded.
- Job-level emergency stop behavior remains backward-compatible.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added generalized kill switches for agents, identities, tools, targets, paths, and environments with strict fail-closed state handling.
Semver marker override: [semver:minor]
Contract/API impact: additive schema, CLI command/flags, Gate/MCP reason codes, and explain fields.
Versioning/migration impact: additive; existing job emergency stop remains supported.
Architecture constraints: kill-switch enforcement happens in Go core and boundary adapters, not wrappers.
ADR required: yes
TDD first failing test(s): agent stop, tool stop, target stop, environment stop, expired no-op, unavailable strict-state block.
Cost/perf impact: medium
Chaos/failure hypothesis: partially written or unreadable state file must not permit high-risk execution in strict mode.

### Story GSC2.2: Structured policy explain JSON

Recommendation:
Replace prose-only `--explain` for Gate decisions with stable JSON explain
output.

Why:
Security needs a machine-readable answer for why an action was allowed, blocked,
or approval-gated.

Strategic direction:
Keep human explain behavior additive, but define `gait gate eval --explain --json`
as a schema-backed decision explanation artifact.

Expected benefit:
CI, tickets, runbooks, security review, and proof bundles can consume a stable
explain object without scraping prose.

Tasks:

- Add `schemas/v1/gate/policy_explain.schema.json` and Go structs under
  `core/schema/v1/gate/`.
- Change `cmd/gait/explain.go` and `cmd/gait/gate.go` so `gate eval --explain
  --json` performs policy evaluation and emits structured explain output, while
  simple top-level `--explain` prose remains available.
- Extend `core/gate/policy.go` or a new `core/gate/explain.go` to assemble
  verdict, matched rules, missing fields, approval requirement, broker/JIT
  requirement, credential posture, freeze-window state, kill-switch state,
  sandbox state, fail-closed reasons, proof refs, and deterministic reason-code
  ordering.
- Add golden tests for allow, block, require approval, broker missing, freeze
  active, kill-switch active, sandbox missing, and malformed input.
- Update docs and examples for machine-readable explain.

Repo paths:

- `cmd/gait/explain.go`
- `cmd/gait/gate.go`
- `core/gate/policy.go`
- `core/gate/explain.go`
- `core/gate/explain_test.go`
- `core/schema/v1/gate/types.go`
- `schemas/v1/gate/policy_explain.schema.json`
- `docs/contracts/policy_explain.md`
- `docs/policy_authoring.md`
- `CHANGELOG.md`

Run commands:

- `go test ./core/gate ./core/schema/... ./cmd/gait`
- `make test-contracts`
- `make test-scenarios`
- `make prepush-full`

Test requirements:

- `--explain --json` emits schema-valid JSON for all verdict classes.
- Output order is deterministic for matched rules, missing fields, proof refs,
  reasons, and violations.
- Explain distinguishes "blocked by policy", "approval required", "fail closed",
  "broker missing", "freeze active", "kill switch active", and "sandbox missing".
- Existing prose explain commands do not regress.

Matrix wiring:
Fast lane for explain assembly tests; core CI lane for CLI/schema/golden
contracts; acceptance lane for scenario fixtures; risk lane for fail-closed
explain details.

Acceptance criteria:

- Security tools can parse `gait gate eval --explain --json` without relying on
  human prose.
- Explain output is stable, versioned, schema-backed, and fixture-tested.
- Docs show example JSON and the schema path.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added schema-backed JSON policy explain output for Gate evaluations.
Semver marker override: [semver:minor]
Contract/API impact: new public JSON schema and CLI behavior.
Versioning/migration impact: additive; prose explain remains available for existing users.
Architecture constraints: explain is derived from authoritative Go evaluation results, not recomputed by CLI wrappers.
ADR required: no
TDD first failing test(s): allow explain, block explain, approval explain, broker missing explain, freeze active explain, kill-switch explain, sandbox missing explain.
Cost/perf impact: medium
Chaos/failure hypothesis: explanation assembly failure must not turn a blocked or undecidable action into allow.

## Wave 3: P1 Broker And Proof Artifacts

### Story GSC3.1: First-class broker recipes

Recommendation:
Add documented broker recipes and thin built-in adapters for common JIT systems.

Why:
The command broker is flexible, but the product story names AWS STS, GitHub
OIDC, Vault, GCP/Azure, and Okta/CyberArk-style brokering.

Strategic direction:
Keep core offline-safe and provider-neutral, while adding deterministic stubs,
receipt parsers, docs, and examples that make real provider integration obvious.

Expected benefit:
Operators can wire their existing JIT credential systems to Gait without
inventing receipt shape or accidentally leaking secrets into traces.

Tasks:

- Define provider adapter interfaces in `core/credential/` for receipt
  normalization without making network calls mandatory in core tests.
- Add thin adapters or receipt parsers for AWS STS, GitHub OIDC, Vault dynamic,
  GCP STS, Azure federated credentials, and generic command/file receipts.
- Add deterministic test stubs for each provider style with source, issuer,
  scope, TTL, request digest, target binding, run binding, and job binding.
- Add examples under `examples/credential-brokers/` that shell out or read
  provider-issued receipts and emit only refs/metadata.
- Add `docs/contracts/credential_brokers.md` with provider recipes, privacy
  rules, failure behavior, and offline test strategy.
- Keep command broker timeout/output size/error behavior covered in
  `core/credential/providers_test.go`.

Repo paths:

- `core/credential/broker.go`
- `core/credential/providers.go`
- `core/credential/providers_test.go`
- `core/gate/credential_evidence.go`
- `core/gate/broker_receipt.go`
- `examples/credential-brokers/`
- `docs/contracts/credential_brokers.md`
- `CHANGELOG.md`

Run commands:

- `go test ./core/credential ./core/gate ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make prepush-full`

Test requirements:

- Each provider-style stub emits a valid receipt with no raw credential value.
- Receipt parser rejects missing issuer/source/scope/TTL/request digest when
  policy requires them.
- Command/file receipt adapters reject oversized, malformed, or secret-bearing
  payloads.
- Broker request digest and binding mismatches block deterministically.

Matrix wiring:
Fast lane for credential package tests; core CI lane for Gate receipt contracts;
risk lane for malformed and oversized broker outputs; docs/example smoke for
offline recipes.

Acceptance criteria:

- Provider recipes exist for the named JIT systems and are runnable offline with
  stubs.
- Gait stores only credential refs, issuer/source metadata, scope, TTL, request
  digest, and binding proof.
- Docs make clear that real provider setup is optional and outside core
  correctness.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added first-class credential broker recipes and deterministic provider-style receipt stubs for common JIT systems.
Semver marker override: [semver:minor]
Contract/API impact: additive provider receipt examples and optional adapter interfaces.
Versioning/migration impact: additive; existing stub/env/command broker modes continue to work.
Architecture constraints: provider-specific logic normalizes receipts only; Gate still makes all allow/block decisions.
ADR required: no
TDD first failing test(s): AWS STS receipt allow, GitHub OIDC receipt allow, Vault dynamic receipt allow, missing issuer block, oversized command output block.
Cost/perf impact: low
Chaos/failure hypothesis: provider receipt parser failure must produce safe unavailable/invalid evidence and never fall back to a raw credential.

### Story GSC3.2: Authorization proof bundle

Recommendation:
Add a named proof bundle that links decision trace, approval audit, credential
evidence, freeze/kill/sandbox evidence, and action outcome.

Why:
The pieces exist, but operators need one object proving decision, approval,
credential use, and outcome together.

Strategic direction:
Define an authorization bundle artifact or PackSpec call profile that references
the existing evidence chain by digest and verifies offline through Gait.

Expected benefit:
Incident tickets and audits can attach one portable proof object instead of a
loose set of traces, receipts, and logs.

Tasks:

- Add `schemas/v1/gate/authorization_bundle.schema.json` with trace ID, policy
  digest, intent digest, approval audit path/digest, credential evidence
  path/digest, freeze-window decision, kill-switch decision, sandbox evidence
  digest, run/result record, outcome receipt, pack manifest digest, and
  signature verification metadata.
- Add Go types in `core/schema/v1/gate/types.go` or a new gate schema file.
- Add bundle builder and verifier under `core/pack/` or a small
  `core/gate/authorization_bundle.go` that delegates deterministic packaging to
  `core/pack/`.
- Add CLI wiring for the selected command path and command tests for build,
  verify, missing component, tamper, and schema validation.
- Update `core/gate/trace.go` relationships so bundle references and proof refs
  are deterministic.
- Add `docs/contracts/authorization_bundle.md` and examples for ticket-ready
  proof.

Repo paths:

- `core/schema/v1/gate/types.go`
- `core/gate/trace.go`
- `core/pack/`
- `cmd/gait/pack.go`
- `cmd/gait/gate.go`
- `schemas/v1/gate/authorization_bundle.schema.json`
- `docs/contracts/authorization_bundle.md`
- `examples/policy/authorization_bundle/`
- `CHANGELOG.md`

Run commands:

- `go test ./core/gate ./core/pack ./core/schema/... ./cmd/gait`
- `make test-contracts`
- `make test-scenarios`
- `make test-hardening`
- `make prepush-full`

Test requirements:

- Bundle build is byte-stable from identical inputs.
- Bundle verify passes offline for valid evidence.
- Tampered trace, credential evidence, approval audit, sandbox evidence, or
  outcome record fails verification.
- Missing required proof refs fail closed for strict high-risk bundle profiles.
- JSON in bundle digests uses JCS canonicalization.

Matrix wiring:
Fast lane for bundle unit tests; core CI lane for schema/pack/golden contracts;
acceptance lane for end-to-end proof scenario; risk lane for tamper and missing
component cases; release lane if command behavior is public.

Acceptance criteria:

- One named authorization bundle links decision, approval, credential,
  freeze/kill/sandbox state, and outcome evidence.
- `gait pack verify <artifact.zip> --json` or the chosen verify command detects
  tampering offline.
- Docs provide a ticket-ready proof workflow.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added an authorization proof bundle that links Gate decisions, approvals, credential evidence, policy state, sandbox posture, and action outcome for offline verification.
Semver marker override: [semver:minor]
Contract/API impact: new artifact schema, pack profile/command contract, proof refs, and verification behavior.
Versioning/migration impact: additive; existing traces and packs remain valid.
Architecture constraints: deterministic packaging and verification stay in Go core; bundle JSON digests use JCS.
ADR required: yes
TDD first failing test(s): valid bundle verify, trace tamper fail, credential evidence tamper fail, missing approval audit fail, byte-stable rebuild.
Cost/perf impact: medium
Chaos/failure hypothesis: incomplete or tampered evidence must be visible as verification failure, never silently downgraded to a warning.

## Wave 4: P2 Rollout And Operator Adoption

### Story GSC4.1: Trust graduation controls

Recommendation:
Make "read-only first, write later" an explicit policy rollout pattern.

Why:
Current policy can express this, but there is no named Gait workflow for
graduating safe paths and reducing approval fatigue.

Strategic direction:
Ship templates, fixtures, docs, and approved-script promotion tests for staged
policy rollout.

Expected benefit:
Teams can adopt Gait incrementally: observe first, allow safe reads, gate writes,
require brokered JIT for writes, and block destructive operations by default.

Tasks:

- Add policy templates and examples for stages: observe, dry-run, read-only
  allow, approval-gated write, brokered write, and blocked destructive.
- Add fixtures under `examples/policy/trust_graduation/` covering promotion from
  approved script/pattern to broader allowlist with deterministic evidence.
- Add tests in `core/gate/approved_scripts_test.go` and policy command tests for
  approved script/pattern promotion, expiry, scope mismatch, and rollback to
  approval-gated mode.
- Update `docs/policy_rollout.md`, `docs/policy_authoring.md`, README snippets,
  docs-site LLM exports, and `CHANGELOG.md`.
- Document which stages require changelog entries, approval audit, broker
  evidence, and authorization bundles.

Repo paths:

- `docs/policy_rollout.md`
- `docs/policy_authoring.md`
- `core/gate/approved_scripts.go`
- `core/gate/approved_scripts_test.go`
- `cmd/gait/policy_templates/`
- `examples/policy/trust_graduation/`
- `docs-site/public/llms.txt`
- `docs-site/public/llm/`
- `README.md`
- `CHANGELOG.md`

Run commands:

- `go test ./core/gate ./cmd/gait`
- `make test-contracts`
- `make test-scenarios`
- `make prepush-full`

Test requirements:

- Observe/dry-run stages do not execute side effects.
- Read-only allow does not permit write/destructive operations.
- Approval-gated write requires valid approvals.
- Brokered write requires valid JIT credential evidence.
- Approved script/pattern promotion allows only matching digest, scope, and TTL.
- Expired or mismatched promotion falls back to approval or block per policy.

Matrix wiring:
Fast lane for approved-script tests; core CI lane for command/template
contracts; acceptance lane for staged rollout scenario; docs/example smoke for
copy-paste policy paths.

Acceptance criteria:

- Users have named rollout stages with runnable policies and fixtures.
- Promotion from approved pattern to broader policy is deterministic and
  reversible.
- Docs explain how to reduce approval fatigue without weakening fail-closed
  controls.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added trust graduation templates and fixtures for staged Gate rollout from observe mode through brokered write and blocked destructive controls.
Semver marker override: [semver:minor]
Contract/API impact: additive templates, examples, docs, and policy fixture coverage.
Versioning/migration impact: additive; no existing policy behavior changes.
Architecture constraints: named workflow uses existing Go policy evaluation and approved-script verification.
ADR required: no
TDD first failing test(s): read-only stage blocks write, approval-gated write requires token, brokered write requires JIT, approved pattern scope mismatch blocks.
Cost/perf impact: low
Chaos/failure hypothesis: promotion metadata expiry or signature failure must never silently expand write permissions.

## Explicit Non-Goals

- Do not add a hosted control plane, dashboard, or network-required service.
- Do not make Gait issue cloud credentials. Gait validates broker receipts and
  credential references.
- Do not move enforcement decisions into Python, docs examples, shell wrappers,
  or provider-specific code.
- Do not store raw credentials, tokens, private keys, environment payloads, or
  customer data in traces, bundles, examples, or fixtures.
- Do not weaken existing job emergency stop behavior while adding generalized
  kill-switch scopes.
- Do not break existing v1 schemas, Gate verdict names, or exit code contracts.

## Definition of Done

- Every story starts with targeted failing tests and lands with deterministic
  fixtures, schema validation, and golden output where public JSON changes.
- `make lint-fast`, `make test-fast`, story-specific targeted tests,
  `make test-contracts`, required risk lanes, and `make prepush-full` pass
  before implementation handoff.
- Public CLI/help/JSON/schema/doc changes are reflected in `README.md`, `docs/`,
  docs-site LLM exports, examples, and `CHANGELOG.md`.
- All generated test artifacts use temp directories or tracked fixture paths
  only. No local runpacks, credentials, cache files, or state files are
  committed by accident.
- Authorization evidence can be verified offline, and tampering is detected with
  stable machine-readable failure output.
- High-risk missing-state, unreadable-state, malformed-state, missing broker,
  missing sandbox, active freeze, active kill-switch, and invalid credential
  cases fail closed with stable reason codes and no side effect.
