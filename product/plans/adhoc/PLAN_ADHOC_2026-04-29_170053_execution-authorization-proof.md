# PLAN ADHOC: Execution Authorization Proof

Date: 2026-04-29
Source: user-provided Gait Work Plan
Profile: `gait`
Scope: execution-time policy enforcement and authorization proof for privileged AI agent actions
Plan path: `product/plans/adhoc/PLAN_ADHOC_2026-04-29_170053_execution-authorization-proof.md`

This plan converts the supplied North Star into an execution-ready backlog. It
does not overwrite `product/PLAN_NEXT.md` or any rolling plan file.

## North Star

Gait is the execution-time policy enforcement and authorization proof point for
privileged AI agent actions.

It decides whether a covered action may execute now, under current identity,
credential, policy, approval, target, context, sandbox, delegation,
freeze-window, and kill-switch state. It emits verifiable proof of the decision
and outcome.

## Global Decisions (Locked)

- Go remains authoritative for schemas, canonicalization, hashing, signing,
  pack verification, policy evaluation, job lifecycle, broker evidence,
  outcome receipts, and CLI JSON output.
- Python remains a thin adoption layer only. It may serialize intent context and
  call local `gait`, but must not own policy, signing, credential, proof, or
  verdict logic.
- New authorization evidence must be additive to v1 schemas unless an explicit
  migration is documented and tested.
- Any JSON included in a digest, signature, cache key, proof bundle, or diff
  must use RFC 8785 JCS canonicalization before hashing or signing.
- `oss-prod` is the strict production profile for fail-closed behavior. Missing
  high-risk context, policy state, budget state, broker evidence, sandbox
  evidence, or required BOM evidence blocks execution.
- Raw secrets are invalid intent data. Gait records credential references,
  issuer/source metadata, scopes, TTL, target/run/job binding, and digests, not
  credential material.
- The named product artifact is an authorization proof bundle that can be
  verified offline with one command.
- Existing job-level stop behavior remains required. Broader kill-switch scopes
  are added in a forward-compatible schema and enforced by supported surfaces.
- The V1 demo path is the minimum product proof: static credential blocks,
  approval-without-JIT blocks, approval plus brokered scoped JIT allows, kill
  switch blocks the next matching action, and the evidence pack verifies
  offline.

## Current Baseline (Observed)

Observed command evidence:

- `go run ./cmd/gait doctor --json`
- Result: `ok=true`, `status=warn`, failed checks `0`, warnings `2`.
- Warnings were fixable local setup warnings for missing `gait-out` and missing
  registry cache. Repo checkout checks, schema files, hooks path, job runtime
  durability, and gate rate-limit lock checks passed.

Observed implementation baseline:

- `core/schema/v1/gate/types.go` already defines gate trace, intent request,
  target metadata, approval token/audit, delegation token/audit, broker
  credential record, skill provenance, and basic intent context fields.
- `core/gate/intent.go` already normalizes target kind, value, operation,
  sensitivity, endpoint class/domain, discovery method, destructive hints,
  provenance, skill provenance, delegation, context, and canonical intent
  digests.
- `core/gate/policy.go` already uses strict YAML parsing and supports policy
  rules for endpoint constraints, dataflow, context evidence, approvals,
  delegation, broker requirements, rate limits, destructive budgets, skill
  provenance, WRKR-derived context, and fail-closed required fields.
- `core/credential/providers.go` already supports `stub`, `env`, and `command`
  broker modes, command timeout/output limits, credential refs, and env-token
  digest-derived references without recording raw token values.
- `core/gate/context_wrkr.go` already loads WRKR inventory and enriches intent
  auth context with tool name, data class, endpoint class, and autonomy level.
- `core/pack/pack.go` and `core/pack/proof.go` already build deterministic
  packs with manifests, source payloads, context envelopes, proof records, hash
  verification, and optional signature verification.
- Existing behavior is not yet a complete authorization product surface: action
  context is incomplete, agent identity lifecycle is not first-class, standing
  credential controls are shallow, broker evidence is not fully bound to
  issuer/scope/TTL/target/run/job, explain JSON is not a stable machine
  contract, proof bundles do not yet link all authorization evidence, outcome
  receipts are not first-class for allowed/non-executed results, sandbox
  metadata is not enforced for generated-code execution, and the V1 demo path is
  not packaged as an end-to-end acceptance suite.

## Exit Criteria

- Missing required high-risk action context fails closed in `oss-prod` with
  deterministic reason codes and no side effect.
- Expired, revoked, undeclared, unapproved, ownerless, or manifest-mismatched
  agents block where policy requires active approved identity.
- High-risk actions using static PATs, cloud-admin standing credentials, env
  standing credentials, inherited standing credentials, or unknown credential
  provenance block when standing privilege is disallowed.
- Broker-required policies block when broker evidence is missing, failed,
  expired, out of scope, out of TTL, target-mismatched, or run/job-mismatched.
- Structured explain JSON is deterministic, schema-validated, golden-tested,
  and sufficient for machine consumers without prose parsing.
- Authorization proof bundles link decision traces, approval audit, broker
  receipt, context evidence, sandbox evidence, delegation audit, outcome
  receipt, and pack digest, and verify offline with tamper detection.
- Blocked, approval-required, and dry-run results prove non-execution. Allowed
  results prove decision plus executor outcome receipt.
- `proc.exec` and generated-code execution without acceptable sandbox metadata
  block in high-risk policy paths.
- Job kill switch behavior remains supported; broader stop scopes are
  schema-stable and enforced consistently by `gate eval` and MCP surfaces where
  implemented.
- V1 demo acceptance path passes through one scripted offline flow and produces
  a proof pack that `gait pack verify <artifact.zip> --json` verifies.

## Public API and Contract Map

- CLI contracts:
  - `gait gate eval --policy <policy.yaml> --intent <intent.json> --json`
  - `gait gate eval ... --profile oss-prod --json`
  - `gait gate eval ... --credential-broker off|stub|env|command --json`
  - `gait gate eval ... --wrkr-inventory <inventory.json> --json`
  - New or extended proof-building command, preferably under `gait pack`, must
    still verify through `gait pack verify <artifact.zip> --json`.
  - New explain JSON must be reachable without changing existing human
    `--explain` behavior; use an additive JSON object or additive flag.
- Schema contracts:
  - `schemas/v1/gate/intent_request.schema.json`
  - `schemas/v1/gate/gate_result.schema.json`
  - `schemas/v1/gate/trace_record.schema.json`
  - `schemas/v1/gate/approval_audit_record.schema.json`
  - `schemas/v1/gate/broker_credential_record.schema.json`
  - New additive authorization proof, outcome receipt, sandbox evidence,
    agent identity, kill switch, and action BOM schemas under `schemas/v1/`.
- Go contracts:
  - `core/gate/` owns policy evaluation, fail-closed checks, explain assembly,
    identity/credential/sandbox/context/delegation/budget decisions, and reason
    code ordering.
  - `core/credential/` owns broker request/response normalization and safety.
  - `core/pack/` owns proof bundle packing, manifest hashing, verification, and
    tamper detection.
  - `core/jobruntime/` owns job stop state and broad stop state persistence if
    shared with job lifecycle.
  - `core/schema/` owns schema validation and golden fixtures.
- Documentation contracts:
  - Public behavior changes update `README.md`, `docs/`, `docs-site/public/llms.txt`,
    `docs-site/public/llm/`, `examples/integrations/`, and `CHANGELOG.md` where
    relevant.

## Docs and OSS Readiness Baseline

- Keep first-screen docs integration-first: users should see how to put Gait at
  the execution boundary, pass structured context, and verify proof before
  reading internals.
- Add contract docs for action context, credential provenance, broker receipt,
  structured explain JSON, authorization proof bundles, outcome receipts,
  sandbox metadata, and kill switches.
- Examples must be offline and stub-safe by default. Real credential or real
  execution examples require explicit unsafe wording and must not include
  secrets.
- Docs and examples must not claim enforcement that is not implemented in Go
  core or CLI.
- Keep OSS trust posture clear: artifact schemas, offline verification, stable
  exits, privacy defaults, and fail-closed production behavior are product
  contracts.

## Recommendation Traceability

| User recommendation | Planned coverage |
|---|---|
| 1. Action Context Contract | Story A1.1 |
| 2. Agent Identity Lifecycle | Story A1.2 |
| 3. Credential Provenance And Standing Privilege | Story A1.3 |
| 4. Brokered JIT Access Requirement | Story A1.4 |
| 5. ABAC/PBAC Policy Attributes | Story A3.1 |
| 6. Structured Policy Explain | Story A2.1 |
| 7. Authorization Proof Bundle | Story A2.2 |
| 8. Approval And Segregation Of Duties | Story A3.2 |
| 9. Kill Switch / Emergency Stop | Story A2.5 |
| 10. Circuit Breakers And Budgets | Story A3.3 |
| 11. Context And Memory Boundary Rules | Story A3.4 |
| 12. Sandbox Requirement For Generated Code | Story A2.4 |
| 13. Action Outcome Evidence | Story A2.3 |
| 14. Wrkr / Agent Action BOM Consumption | Story A3.5 |
| 15. V1 Demo Acceptance Path | Story A4.1 |

## Test Matrix Wiring

- Fast lane:
  - `make lint-fast`
  - `make test-fast`
- Core CI lane:
  - targeted Go package tests under `core/gate`, `core/credential`,
    `core/pack`, `core/schema`, `core/jobruntime`, and `cmd/gait`
  - `make test-contracts`
- Acceptance lane:
  - `make test-scenarios`
  - new offline authorization demo acceptance script
- Cross-platform lane:
  - existing GitHub Actions OS matrix for Linux, macOS, and Windows
  - path and atomic-state tests must use `t.TempDir()`
- Risk lane:
  - `make test-hardening`
  - `make test-chaos` for fail-closed, lock contention, tamper, and missing-state
    scenarios
  - `make test-runtime-slo` for budget/counter and proof verify overhead where
    touched
- Release/UAT lane:
  - `make prepush-full`
  - `bash scripts/test_uat_local.sh` when the final demo becomes release-facing
- Gating rule:
  - Stories that change schemas, CLI JSON, exit behavior, proof verification,
    policy decisions, or fail-closed behavior require targeted failing tests
    first, `make test-contracts`, and `make prepush-full` before handoff.

## Minimum-Now Sequence

1. Implement high-risk action context and `oss-prod` fail-closed required fields.
2. Add credential provenance and standing-privilege blocking.
3. Strengthen brokered JIT request/response binding.
4. Add structured explain JSON.
5. Add authorization proof bundle and outcome receipt linkage.
6. Add sandbox requirement for high-risk `proc.exec`.
7. Add job kill switch parity plus forward-compatible broader stop schema.
8. Ship the V1 demo acceptance path.
9. Layer ABAC/PBAC, SOD, budgets, memory boundary rules, and BOM consumption.

## Wave 1: P0 Authorization Inputs And Fail-Closed Primitives

### Story A1.1: Action context contract for high-risk intents

Priority: P0

Recommendation:
Define the structured context every enforceable high-risk action can carry and
make required fields fail closed in `oss-prod`.

Why:
Policy cannot safely authorize privileged agent actions if identity, run, job,
workflow, repo, target, endpoint, credential, approval, context evidence, and
BOM evidence are absent or unverifiable.

Strategic direction:
Make structured intent the durable authorization substrate, not prompt text,
wrapper-local state, or ad hoc flags.

Expected benefit:
High-risk actions become machine-checkable and proof-ready before any credential
or tool execution occurs.

Tasks:
- Extend gate intent schema and Go types with additive action context fields:
  `agent_id`, `agent_identity`, `run_id`, `workflow_id`, `repo`, `environment`,
  `credential_ref`, `credential_source`, `credential_access_type`,
  `credential_issuer`, `credential_ttl_seconds`, `approval_ref`,
  `wrkr_inventory_ref`, and `agent_action_bom_ref`.
- Preserve existing `job_id`, `session_id`, `request_id`,
  `context_set_digest`, target kind/value/operation/sensitivity, and
  `endpoint_class` behavior.
- Add `fail_closed.required_high_risk_fields` or equivalent profile-aware
  policy support for the required high-risk field set.
- Bind required context fields into `intent_digest`, trace output, and proof
  surfaces where they are authorization inputs.
- Reject raw credential-shaped fields in intent args and auth context where a
  credential reference is required.
- Add docs contract for action context and privacy-safe credential references.

Repo paths:
- `core/schema/v1/gate/types.go`
- `schemas/v1/gate/intent_request.schema.json`
- `schemas/v1/gate/trace_record.schema.json`
- `core/gate/intent.go`
- `core/gate/policy.go`
- `cmd/gait/gate.go`
- `core/schema/testdata/`
- `core/gate/testdata/`
- `docs/contracts/action_context.md`
- `README.md`
- `CHANGELOG.md`

Run commands:
- `go test ./core/gate ./core/schema/... ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make prepush-full`

Test requirements:
- Missing each required high-risk field blocks in `oss-prod` with stable reason
  codes.
- The same missing fields do not unexpectedly break lower-risk standard profile
  fixtures unless policy explicitly requires them.
- Raw secret-like credential values are rejected and absent from trace and pack
  output.
- Intent digest changes when a bound authorization context field changes.

Matrix wiring:
- Fast lane: targeted `go test ./core/gate ./cmd/gait`.
- Core CI lane: schema/golden tests plus `make test-contracts`.
- Risk lane: `make test-hardening` for fail-closed missing-context paths.
- Cross-platform lane: schema and command tests in OS matrix.

Acceptance criteria:
- Missing required high-risk context fails closed in `oss-prod`.
- Context fields are bound into digest/signature surfaces where appropriate.
- Raw secrets are rejected and never recorded.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added high-risk action context enforcement for `oss-prod` with deterministic missing-field reason codes and privacy-safe credential references.
Semver marker override: [semver:minor]
Contract/API impact: additive schema fields, policy fields, trace fields, reason codes, and JSON output.
Versioning/migration impact: additive v1 schema evolution; existing fixtures remain valid unless `oss-prod` policy requires high-risk fields.
Architecture constraints: Go core owns context normalization and policy evaluation; wrappers only pass structured fields.
ADR required: yes
TDD first failing test(s): table-driven `oss-prod` gate eval fixtures for each missing required high-risk field and a raw-secret rejection fixture.
Cost/perf impact: low
Chaos/failure hypothesis: malformed or partial context must block with stable JSON and no trace leakage of sensitive payloads.

### Story A1.2: Agent identity lifecycle policy

Priority: P0

Recommendation:
Block expired, revoked, undeclared, unapproved, or manifest-mismatched agents.

Why:
Privileged action authorization must prove which agent identity acted, whether
that identity was declared, who owns it, and whether it was still approved and
active.

Strategic direction:
Make agent identity lifecycle a first-class policy input alongside user identity
and credential provenance.

Expected benefit:
Security teams can revoke, expire, and owner-bind agent execution without
rewriting adapters or trusting runtime naming conventions.

Tasks:
- Add agent identity context/schema fields for lifecycle state, owner,
  manifest digest, publisher, source, issued/approved/expires timestamps, and
  revocation status.
- Add policy controls for allowed agent IDs, denied/revoked agent IDs, required
  manifest digest, allowed manifest publishers/sources, required lifecycle
  states (`approved`, `active`), agent expiry, and owner requirement.
- Normalize identity reason codes such as `agent_revoked`, `agent_unknown`,
  `agent_expired`, `agent_manifest_mismatch`, `agent_owner_missing`, and
  `agent_lifecycle_state_invalid`.
- Ensure `oss-prod` blocks unknown agents when policy requires declared agents.
- Add trace/explain fields for identity evidence status and deterministic
  reason ordering.

Repo paths:
- `core/gate/agent_identity.go`
- `core/gate/policy.go`
- `core/gate/intent.go`
- `core/schema/v1/gate/types.go`
- `schemas/v1/gate/intent_request.schema.json`
- `schemas/v1/gate/gate_result.schema.json`
- `schemas/v1/gate/trace_record.schema.json`
- `cmd/gait/gate.go`
- `docs/contracts/agent_identity.md`
- `examples/policy/`
- `CHANGELOG.md`

Run commands:
- `go test ./core/gate ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make prepush-full`

Test requirements:
- Revoked agent blocks.
- Unknown agent blocks in `oss-prod` when declaration is required.
- Expired manifest blocks.
- Wrong manifest digest blocks.
- Valid active approved agent allows when all other policy controls pass.

Matrix wiring:
- Fast lane: targeted gate tests.
- Core CI lane: JSON schema and CLI golden tests.
- Risk lane: `make test-hardening` for fail-closed identity state.
- Cross-platform lane: no absolute paths; fixtures only.

Acceptance criteria:
- Revoked agent blocks.
- Unknown agent blocks in `oss-prod`.
- Expired manifest blocks.
- Traces include deterministic identity reason codes.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added agent identity lifecycle policy controls for approved active agents, revocation, manifest digests, publishers, expiry, and owner requirements.
Semver marker override: [semver:minor]
Contract/API impact: additive policy, intent, trace, explain, and reason-code contract.
Versioning/migration impact: additive; migration docs explain default behavior when policy does not opt into declared agent enforcement.
Architecture constraints: identity decisions remain in `core/gate`; SDKs and adapters cannot self-attest allow decisions.
ADR required: yes
TDD first failing test(s): revoked, unknown, expired, manifest mismatch, owner missing, and valid approved agent fixtures.
Cost/perf impact: low
Chaos/failure hypothesis: missing or corrupt agent manifest evidence in `oss-prod` must block rather than degrade to identity-only allow.

### Story A1.3: Credential provenance and standing privilege controls

Priority: P0

Recommendation:
Make standing privilege policy-checkable and block high-risk static or standing
credentials where policy disallows them.

Why:
Approving a tool action is not enough if the action uses long-lived broad
credentials. The credential posture must be visible, bounded, and enforceable.

Strategic direction:
Turn credential evidence into a policy input and proof artifact, while keeping
Gait out of raw secret storage.

Expected benefit:
High-risk actions can distinguish standing PATs and inherited admin access from
JIT, scoped, target-bound credentials.

Tasks:
- Extend credential context with access type, source, issuer, subject, scope,
  TTL, credential ref, owner, target binding, run binding, and job binding.
- Add policy controls: `block_standing_credentials`,
  `allowed_credential_sources`, `allowed_credential_issuers`,
  `allowed_credential_access_types`, `max_credential_ttl_seconds`, and
  `require_jit_credential`.
- Normalize credential source enums for GitHub PAT/OIDC, AWS IAM user/STS,
  Vault dynamic, GCP STS, Azure federated, Kubernetes service account, env, and
  unknown.
- Block static PAT, cloud admin, unknown, standing, static, or inherited
  credentials on high-risk actions when policy requires JIT or blocks standing
  privilege.
- Update broker credential record to include provenance without recording
  secrets.
- Add docs and examples for credential provenance JSON.

Repo paths:
- `core/gate/credential_policy.go`
- `core/gate/credential_evidence.go`
- `core/credential/broker.go`
- `core/schema/v1/gate/types.go`
- `schemas/v1/gate/broker_credential_record.schema.json`
- `schemas/v1/gate/intent_request.schema.json`
- `schemas/v1/gate/gate_result.schema.json`
- `cmd/gait/gate.go`
- `examples/policy/credentials/`
- `docs/contracts/credential_provenance.md`
- `CHANGELOG.md`

Run commands:
- `go test ./core/gate ./core/credential ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make prepush-full`

Test requirements:
- High-risk action using static PAT blocks.
- High-risk action using cloud admin standing credential blocks.
- Unknown source/access type blocks under strict policy.
- JIT credential allows only when issuer, scope, TTL, target binding, and
  run/job binding match.
- Credential evidence records refs and provenance but no raw credential bytes.

Matrix wiring:
- Fast lane: gate and credential package tests.
- Core CI lane: schema/golden and CLI JSON tests.
- Risk lane: `make test-hardening` for raw-secret rejection and fail-closed
  missing provenance.
- Cross-platform lane: env tests use isolated variables and `t.TempDir()`.

Acceptance criteria:
- High-risk action using static PAT/cloud admin/standing credential blocks.
- JIT credential allows only if issuer, scope, TTL, target, and run/job binding
  match.
- Credential evidence records provenance without storing secrets.

Changelog impact: required
Changelog section: Security
Draft changelog entry: Added credential provenance policy controls that block standing or static credentials for high-risk actions and record privacy-safe credential evidence.
Semver marker override: [semver:minor]
Contract/API impact: additive policy controls, schema fields, reason codes, and evidence record fields.
Versioning/migration impact: additive; existing broker evidence remains valid but may be insufficient under strict policies.
Architecture constraints: credential checks live in Go core; brokers return references and receipts, not secrets.
ADR required: yes
TDD first failing test(s): static PAT block, cloud-admin standing block, unknown source block, valid JIT allow, TTL mismatch block, target binding mismatch block.
Cost/perf impact: low
Chaos/failure hypothesis: unavailable credential provenance state in `oss-prod` must block with `credential_evidence_missing` or more specific reason.

### Story A1.4: Brokered JIT access contract and validation

Priority: P0

Recommendation:
Require scoped temporary access through a broker receipt without making Gait the
credential issuer.

Why:
Gait should verify that a trusted broker supplied appropriately scoped temporary
access, but it should not own credential issuance or store credential material.

Strategic direction:
Make `stub`, `env`, and `command` broker modes conform to one typed receipt
contract that can be packed and verified.

Expected benefit:
Teams can adopt JIT enforcement with local stubs first, then swap in real
brokers without changing the gate decision surface.

Tasks:
- Define broker request and broker response receipt schemas with issuer, scope,
  credential ref, TTL, issued/expires timestamps, subject, target binding, run
  binding, job binding, and request digest.
- Normalize and validate receipts for `stub`, `env`, and `command` providers.
- Fail closed when broker evidence is missing, command fails, output is
  malformed, scope mismatches, TTL exceeds policy, credential ref mismatches, or
  target/run/job bindings mismatch.
- Add broker receipt refs to trace, explain JSON, credential evidence, and proof
  bundle candidate inputs.
- Keep command broker explicit, bounded, allowlist-aware, and non-leaky.

Repo paths:
- `core/credential/broker.go`
- `core/credential/providers.go`
- `core/gate/credential_evidence.go`
- `core/gate/policy.go`
- `core/schema/v1/gate/types.go`
- `schemas/v1/gate/broker_request.schema.json`
- `schemas/v1/gate/broker_credential_record.schema.json`
- `cmd/gait/gate.go`
- `docs/contracts/broker_receipt.md`
- `examples/brokers/`
- `CHANGELOG.md`

Run commands:
- `go test ./core/credential ./core/gate ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make prepush-full`

Test requirements:
- Broker-required policy blocks with no broker.
- Broker failure blocks.
- Scope mismatch blocks.
- TTL mismatch blocks.
- Credential ref mismatch blocks.
- Target/run/job binding mismatch blocks.
- Valid `stub`, `env`, and `command` broker receipts allow when policy matches.

Matrix wiring:
- Fast lane: broker and gate package tests.
- Core CI lane: contract/golden tests for broker JSON.
- Risk lane: `make test-hardening` for command timeout, malformed output, and
  missing receipt.
- Cross-platform lane: command broker tests use portable test helper binaries.

Acceptance criteria:
- Broker-required policy blocks with no broker.
- Broker failure blocks.
- Scope/TTL mismatch blocks.
- Credential evidence is included in proof packs.

Changelog impact: required
Changelog section: Security
Draft changelog entry: Strengthened brokered JIT access validation with typed broker receipts, scope and TTL checks, and proof-ready credential evidence.
Semver marker override: [semver:minor]
Contract/API impact: additive broker schemas, CLI JSON fields, reason codes, and evidence records.
Versioning/migration impact: additive; old minimal broker refs remain accepted only where policy does not require strict receipt validation.
Architecture constraints: broker providers issue refs/receipts; gate validates policy; pack verifies evidence integrity.
ADR required: yes
TDD first failing test(s): no broker, broker command failure, malformed receipt, TTL too long, scope mismatch, binding mismatch, valid stub/env/command receipt.
Cost/perf impact: medium for command broker latency; keep default timeout bounded.
Chaos/failure hypothesis: a hung or noisy command broker must fail closed within deterministic timeout and without leaking stderr secrets.

## Wave 2: P0 Decision Explain, Proof, Outcome, Sandbox, And Stop Controls

### Story A2.1: Structured policy explain JSON

Priority: P0

Recommendation:
Return stable machine-readable explanations for gate decisions.

Why:
Automation, CI, incident review, and proof bundles cannot rely on prose-only
explanations.

Strategic direction:
Make explain JSON a stable contract with deterministic ordering, while
preserving existing human `--explain` ergonomics.

Expected benefit:
Users and downstream tools can understand why Gait allowed, blocked, required
approval, or dry-ran an action without parsing human text.

Tasks:
- Add a schema for gate explain JSON with verdict, matched rules, rule
  priorities, missing fields, approval requirement, credential/JIT requirement,
  context evidence status, sandbox evidence status, delegation evidence status,
  kill-switch status, freeze-window status, rate/budget status, proof refs, and
  reason codes.
- Add deterministic explain assembly in `core/gate`.
- Expose explain JSON through an additive CLI contract, either embedded in
  `gait gate eval --json` or via an additive flag that does not break current
  human `--explain`.
- Golden-test ordering and omissions.
- Document the machine explain contract and distinguish it from human help text.

Repo paths:
- `core/gate/explain.go`
- `core/gate/policy.go`
- `cmd/gait/gate.go`
- `cmd/gait/explain.go`
- `schemas/v1/gate/explain.schema.json`
- `core/schema/testdata/`
- `cmd/gait/testdata/`
- `docs/contracts/gate_explain_json.md`
- `CHANGELOG.md`

Run commands:
- `go test ./core/gate ./cmd/gait ./core/schema/...`
- `make test-contracts`
- `make prepush-full`

Test requirements:
- Explain JSON validates against schema for allow/block/require_approval/dry_run.
- Rule ordering and missing-field ordering are deterministic.
- No prose-only explain is required for machine consumers.
- Existing `--explain` help tests remain green.

Matrix wiring:
- Fast lane: targeted Go tests.
- Core CI lane: schema/golden via `make test-contracts`.
- Risk lane: fail-closed explanations must not omit missing-state status.
- Cross-platform lane: golden output stable across OSes.

Acceptance criteria:
- Stable schema.
- Deterministic ordering.
- Golden tests.
- No prose-only explain for machine consumers.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added schema-validated structured gate explain JSON for deterministic machine-readable policy explanations.
Semver marker override: [semver:minor]
Contract/API impact: additive schema and CLI JSON fields.
Versioning/migration impact: additive; human `--explain` remains backward-compatible.
Architecture constraints: explain is derived from Go gate evaluation data, not recomputed by wrappers.
ADR required: no
TDD first failing test(s): golden allow/block/approval/dry-run explain JSON fixtures with stable order assertions.
Cost/perf impact: low
Chaos/failure hypothesis: partial evaluator state must produce explicit missing/evidence status rather than absent fields that look successful.

### Story A2.2: Authorization proof bundle artifact

Priority: P0

Recommendation:
Make authorization proof a named product artifact that links all decision and
evidence receipts.

Why:
The durable product contract is artifacts and schemas. Security and compliance
need one offline-verifiable bundle, not scattered logs.

Strategic direction:
Build on PackSpec v1 and proof records rather than introducing a separate
artifact system.

Expected benefit:
One artifact proves decision, approvals, broker receipt, context, sandbox,
delegation, outcome/non-execution, and pack integrity.

Tasks:
- Add an authorization proof payload/schema with refs for decision trace,
  approval audit, credential broker receipt, context evidence, sandbox evidence,
  delegation audit, outcome receipt, and pack digest.
- Extend pack build support for authorization proof bundles or add an explicit
  pack subtype that still verifies through `gait pack verify`.
- Ensure linked evidence is included or reference-verifiable with deterministic
  manifest entries.
- Add tamper tests for linked evidence, missing files, hash mismatch, and
  signature failures.
- Add CLI docs and examples for building and verifying an authorization proof
  bundle.

Repo paths:
- `core/pack/authorization.go`
- `core/pack/pack.go`
- `core/pack/proof.go`
- `core/schema/v1/pack/types.go`
- `schemas/v1/pack/authorization_proof.schema.json`
- `schemas/v1/pack/manifest.schema.json`
- `cmd/gait/pack.go`
- `cmd/gait/pack_cli_test.go`
- `docs/contracts/authorization_proof_bundle.md`
- `examples/authorization-proof/`
- `CHANGELOG.md`

Run commands:
- `go test ./core/pack ./cmd/gait ./core/schema/...`
- `make test-contracts`
- `make test-hardening`
- `make prepush-full`

Test requirements:
- One command verifies linked proof bundle offline.
- Tampered linked evidence fails verification.
- Missing linked evidence fails verification.
- Blocked/approval/dry-run bundles prove non-execution.
- Allowed bundles prove decision plus executor result.

Matrix wiring:
- Fast lane: pack and CLI tests.
- Core CI lane: manifest/schema/golden tests and `make test-contracts`.
- Risk lane: tamper and missing-evidence tests under `make test-hardening`.
- Cross-platform lane: deterministic zip bytes and stable paths.

Acceptance criteria:
- One command can verify the linked proof bundle offline.
- Tampered linked evidence fails verification.
- Blocked/approval/dry-run outcomes prove non-execution.
- Allowed outcomes prove decision plus executor result.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added authorization proof bundles that link decision traces, approvals, broker receipts, context, sandbox, delegation, and outcome evidence for offline verification.
Semver marker override: [semver:minor]
Contract/API impact: new pack payload/schema and additive CLI pack behavior.
Versioning/migration impact: additive PackSpec v1 subtype; existing pack verify remains compatible.
Architecture constraints: `core/pack` owns deterministic packaging and verification; linked evidence uses schema-validated canonical payloads.
ADR required: yes
TDD first failing test(s): build/verify authorization proof, tampered approval audit, tampered broker receipt, missing outcome receipt, blocked non-execution proof.
Cost/perf impact: medium for larger bundles; maintain deterministic zip and bounded file sizes.
Chaos/failure hypothesis: partial bundle construction must not produce a verifying artifact with missing required evidence.

### Story A2.3: Action outcome receipt linkage

Priority: P0

Recommendation:
Prove what happened after an allowed decision, and prove non-execution for
blocked, approval-required, and dry-run outcomes.

Why:
An allow verdict without outcome evidence does not answer whether the executor
ran, failed, or touched the expected target.

Strategic direction:
Separate authorization decision proof from execution outcome receipt, then link
them in the proof bundle.

Expected benefit:
Incident responders can verify both "Gait allowed this" and "the executor
reported this result" or "Gait did not execute this."

Tasks:
- Add outcome receipt schema with trace ref, executor ref, result digest,
  status/exit code, side-effect target ref/digest, credential evidence ref, and
  approval audit ref.
- Add non-execution outcome receipts for block, require_approval, and dry_run
  verdicts.
- Add CLI/helper surface for writing outcome receipts from wrapper/adapters
  without moving decision logic out of Go.
- Link outcome receipts into authorization proof bundles.
- Add examples for allowed executor result and blocked non-execution receipt.

Repo paths:
- `core/gate/outcome.go`
- `core/schema/v1/gate/types.go`
- `schemas/v1/gate/outcome_receipt.schema.json`
- `cmd/gait/gate.go`
- `core/pack/authorization.go`
- `sdk/python/`
- `examples/integrations/`
- `docs/contracts/outcome_receipt.md`
- `CHANGELOG.md`

Run commands:
- `go test ./core/gate ./core/pack ./cmd/gait`
- `pytest` or existing Python SDK target if wrapper surface changes
- `make test-contracts`
- `make prepush-full`

Test requirements:
- Allowed action can be packed with decision plus outcome.
- Blocked action emits/verifies non-execution receipt.
- Require-approval without token emits/verifies non-execution receipt.
- Dry-run emits/verifies non-execution receipt.
- Result digest and side-effect target digest tampering fails verification.

Matrix wiring:
- Fast lane: Go tests and targeted Python wrapper tests if touched.
- Core CI lane: schema/golden tests.
- Risk lane: tamper and fail-closed non-execution paths.
- Cross-platform lane: receipt paths in temp dirs.

Acceptance criteria:
- Allowed action can be packed with decision plus outcome.
- Blocked/approval/dry-run proves non-execution.
- Pack verify covers linked receipts.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added action outcome receipts that link allowed execution results and blocked non-execution proof into authorization proof bundles.
Semver marker override: [semver:minor]
Contract/API impact: new schema, CLI/helper output, pack linkage, and wrapper handoff contract.
Versioning/migration impact: additive; existing traces remain valid but incomplete for full authorization proof bundles.
Architecture constraints: Go defines receipt schema and verification; wrappers may submit executor result metadata only.
ADR required: yes
TDD first failing test(s): allowed outcome pack verify, blocked non-execution receipt, dry-run receipt, tampered result digest.
Cost/perf impact: low to medium depending on result digest size; do not store raw sensitive results by default.
Chaos/failure hypothesis: executor failure after allow must produce a failed outcome receipt, not an ambiguous missing-result success.

### Story A2.4: Sandbox evidence requirement for generated code and `proc.exec`

Priority: P0

Recommendation:
Make `proc.exec` and generated-code execution policy-checkable with sandbox
metadata.

Why:
Generated code execution with broad env access, broad network, or unconstrained
write paths is a high-risk action even when the tool name looks harmless.

Strategic direction:
Treat sandbox posture as authorization context and proof evidence.

Expected benefit:
High-risk execution is blocked unless it carries acceptable isolation,
network, timeout, and secret-access metadata.

Tasks:
- Add sandbox metadata schema with network mode, writable paths, read-only
  paths, env access, timeout, filesystem isolation, process isolation, and
  secret access mode.
- Add policy controls: `require_sandbox`, `allowed_network_modes`,
  `max_timeout_seconds`, `deny_env_access`, and `allowed_writable_paths`.
- Bind sandbox metadata digest into trace/proof, not raw env or secrets.
- Enforce sandbox requirements for high-risk `proc.exec` and generated-code
  actions.
- Add docs and examples for sandbox-safe execution metadata.

Repo paths:
- `core/gate/sandbox.go`
- `core/gate/policy.go`
- `core/schema/v1/gate/types.go`
- `schemas/v1/gate/sandbox_evidence.schema.json`
- `schemas/v1/gate/intent_request.schema.json`
- `schemas/v1/gate/trace_record.schema.json`
- `cmd/gait/gate.go`
- `docs/contracts/sandbox_evidence.md`
- `examples/policy/sandbox/`
- `CHANGELOG.md`

Run commands:
- `go test ./core/gate ./cmd/gait ./core/schema/...`
- `make test-contracts`
- `make test-hardening`
- `make test-chaos`
- `make prepush-full`

Test requirements:
- High-risk `proc.exec` without sandbox metadata blocks.
- Broad network mode blocks when policy disallows it.
- Broad env access blocks when `deny_env_access` is true.
- Timeout above max blocks.
- Writable path outside allowlist blocks.
- Trace records sandbox metadata digest and omits env/secrets.

Matrix wiring:
- Fast lane: gate tests.
- Core CI lane: schema/golden tests.
- Risk lane: hardening and chaos for missing/malformed sandbox evidence.
- Cross-platform lane: path allowlist tests handle Windows paths deterministically.

Acceptance criteria:
- High-risk `proc.exec` without sandbox metadata blocks.
- Broad env/network generated-code execution blocks.
- Trace records sandbox metadata digest, not raw env/secrets.

Changelog impact: required
Changelog section: Security
Draft changelog entry: Added sandbox metadata policy enforcement for high-risk `proc.exec` and generated-code actions.
Semver marker override: [semver:minor]
Contract/API impact: additive sandbox schema, policy fields, trace fields, and reason codes.
Versioning/migration impact: additive; strict policies must document sandbox evidence requirements.
Architecture constraints: sandbox evidence is evaluated in Go core; Gait verifies metadata and proof, not sandbox runtime implementation.
ADR required: yes
TDD first failing test(s): missing sandbox block, network disallowed block, env access block, timeout block, writable path block, valid sandbox allow.
Cost/perf impact: low
Chaos/failure hypothesis: malformed sandbox metadata in `oss-prod` must block and must not serialize raw environment data.

### Story A2.5: Kill switch and emergency stop scopes

Priority: P0

Recommendation:
Preserve current `job_id` stop behavior and add forward-compatible broader
stop scopes.

Why:
Operators need immediate coverage for risky actions already in flight or newly
dispatched by agent, tool, target, repo, or environment.

Strategic direction:
Keep job stop state authoritative while introducing a broader stop schema that
can be enforced consistently by `gate eval` and MCP surfaces.

Expected benefit:
Emergency stop becomes a deterministic authorization input rather than an
out-of-band convention.

Tasks:
- Audit current job stop path and reason codes.
- Add schema/state shape for stop scopes: `agent_id`, `tool_name`, `target`,
  `repo`, and `environment`.
- Add deterministic matching and reason codes for job and broader stops.
- Enforce matching stopped job in `gate eval`; extend MCP server/proxy behavior
  where context carries the relevant identifiers.
- Journal blocked dispatch decisions.
- Document forward-compatible behavior and unsupported surfaces explicitly.

Repo paths:
- `core/jobruntime/`
- `core/gate/kill_switch.go`
- `cmd/gait/job.go`
- `cmd/gait/gate.go`
- `cmd/gait/mcp_server.go`
- `schemas/v1/job/`
- `schemas/v1/gate/kill_switch_state.schema.json`
- `docs/contracts/kill_switch.md`
- `CHANGELOG.md`

Run commands:
- `go test ./core/jobruntime ./core/gate ./cmd/gait ./core/mcp`
- `make test-contracts`
- `make test-hardening`
- `make test-chaos`
- `make prepush-full`

Test requirements:
- Matching stopped job blocks with deterministic reason.
- Broader stop schema validates and round-trips.
- Matching broader stop blocks where enforcement is implemented.
- Blocked dispatch is journaled.
- MCP and `gate eval` produce consistent verdicts for supported stop inputs.

Matrix wiring:
- Fast lane: jobruntime, gate, and CLI tests.
- Core CI lane: schema/golden tests.
- Risk lane: lock-safe stop state and fail-closed unavailable state tests.
- Cross-platform lane: state files use temp dirs and atomic writes.

Acceptance criteria:
- Matching stopped job blocks with deterministic reason.
- Broader stop schema is forward-compatible.
- Blocked dispatch is journaled.
- MCP and `gate eval` behavior stay consistent where supported.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added forward-compatible kill-switch scopes while preserving job-level emergency stop enforcement.
Semver marker override: [semver:minor]
Contract/API impact: additive schema, CLI/job state behavior, reason codes, and MCP/gate enforcement fields.
Versioning/migration impact: additive; existing job stop state remains valid.
Architecture constraints: durable stop state must be atomic and deterministic; unsupported surfaces must report capability status.
ADR required: yes
TDD first failing test(s): stopped job blocks, stopped agent blocks, stopped target blocks, unavailable stop state fail-closed in `oss-prod`.
Cost/perf impact: low to medium depending on stop-state lookup; cache only with deterministic invalidation.
Chaos/failure hypothesis: unreadable stop state in `oss-prod` must block covered high-risk actions rather than allowing by omission.

## Wave 3: P1 Rich Policy Controls And Coverage Inputs

### Story A3.1: ABAC/PBAC typed policy attributes

Priority: P1

Recommendation:
Support richer contextual policy matching with typed attributes.

Why:
Production authorization depends on business and operational attributes such as
owner, team, criticality, data class, regulated data, egress, release action,
credential-bearing targets, and freeze windows.

Strategic direction:
Add typed policy attributes without turning Gait into a general-purpose hosted
policy engine.

Expected benefit:
Teams can express production deploy, customer data, financial, regulated, and
freeze-window controls directly in structured policy.

Tasks:
- Add typed attribute schema and Go types for owner/team, repo criticality,
  production target, environment, data class, regulated data,
  customer-impacting target, financial target, credential-bearing target,
  external egress, release/deploy action, credential issuer/source/access type,
  business process, and freeze window.
- Add strict policy validation for known attributes and fail validation on
  unknown strict-policy attributes.
- Add match support for require approval, require JIT, block, and allow
  outcomes.
- Add freeze-window status to explain JSON.
- Add example policies for prod deploy requiring approval plus JIT and
  customer-data write requiring approval or block.

Repo paths:
- `core/gate/attributes.go`
- `core/gate/policy.go`
- `core/schema/v1/gate/types.go`
- `schemas/v1/gate/policy.schema.json`
- `schemas/v1/gate/intent_request.schema.json`
- `examples/policy/attributes/`
- `docs/contracts/policy_attributes.md`
- `CHANGELOG.md`

Run commands:
- `go test ./core/gate ./cmd/gait ./core/schema/...`
- `make test-contracts`
- `make test-hardening`
- `make prepush-full`

Test requirements:
- Prod deploy can require approval plus JIT.
- Customer-data write can block or require approval.
- Freeze-window policy blocks or requires approval deterministically.
- Unknown strict-policy attributes fail validation.

Matrix wiring:
- Fast lane: gate tests.
- Core CI lane: schema/golden tests.
- Risk lane: hardening tests for unknown/missing strict attributes.
- Cross-platform lane: time-zone tests use fixed UTC timestamps.

Acceptance criteria:
- Prod deploy can require approval plus JIT.
- Customer-data write can block or require approval.
- Unknown strict-policy attributes fail validation.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added typed ABAC/PBAC policy attributes for production, data, egress, credential, release, business-process, and freeze-window controls.
Semver marker override: [semver:minor]
Contract/API impact: additive policy and intent attributes, explain fields, reason codes.
Versioning/migration impact: additive; strict validation requires docs for accepted attribute names.
Architecture constraints: attributes remain structured fields; no prompt or free-text policy parsing.
ADR required: no
TDD first failing test(s): prod deploy approval plus JIT, customer-data block, freeze window block, unknown strict attribute error.
Cost/perf impact: low
Chaos/failure hypothesis: ambiguous or unparseable freeze-window data must fail closed under strict policy.

### Story A3.2: Approval segregation of duties and delegation escalation controls

Priority: P1

Recommendation:
Prevent approval misuse, self-approval, role conflict, and delegation
escalation.

Why:
Approval only improves safety when approvers are authorized, independent, and
scope-bound.

Strategic direction:
Make approval and delegation evidence policy-verifiable and proof-linked.

Expected benefit:
Security reviewers can rely on approval audits for separation-of-duties and
delegation-chain review.

Tasks:
- Add policy controls for required approver roles, forbidden self-approval,
  distinct approvers, incompatible role pairs, max delegation chain depth,
  allowed delegatees, external delegate reduced-trust mode, and approval bound
  to JIT/credential requirement.
- Extend approval audit schema with SOD details, role evidence, rejected
  approvers, self-approval status, incompatible-role status, and JIT binding.
- Extend delegation audit with signature-chain status, reduced-trust mode, depth,
  and scope mismatch reason codes.
- Ensure approval scope mismatch blocks.
- Add docs and examples for SOD approval policies.

Repo paths:
- `core/gate/approval.go`
- `core/gate/approval_audit.go`
- `core/gate/delegation.go`
- `core/gate/delegation_audit.go`
- `core/gate/policy.go`
- `core/schema/v1/gate/types.go`
- `schemas/v1/gate/approval_audit_record.schema.json`
- `schemas/v1/gate/delegation_audit_record.schema.json`
- `cmd/gait/approve.go`
- `cmd/gait/gate.go`
- `docs/contracts/approval_sod.md`
- `CHANGELOG.md`

Run commands:
- `go test ./core/gate ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make prepush-full`

Test requirements:
- Requester cannot self-approve when forbidden.
- Same approver cannot satisfy distinct approver count.
- Incompatible role pairs block.
- Unsigned delegation chain blocks.
- Delegation depth over max blocks.
- Approval scope mismatch blocks.
- Approval bound to JIT credential mismatch blocks.

Matrix wiring:
- Fast lane: approval/delegation tests.
- Core CI lane: schema/golden audit tests.
- Risk lane: hardening for malformed/unsigned tokens.
- Cross-platform lane: time/expiry tests use fixed UTC clocks.

Acceptance criteria:
- Requester cannot self-approve when forbidden.
- Unsigned delegation chain blocks.
- Approval scope mismatch blocks.
- Approval audit includes SOD details.

Changelog impact: required
Changelog section: Security
Draft changelog entry: Added approval segregation-of-duties and delegation escalation controls with proof-ready audit details.
Semver marker override: [semver:minor]
Contract/API impact: additive policy fields, audit fields, reason codes, approval/delegation validation behavior.
Versioning/migration impact: additive; existing approvals remain valid only where SOD policy does not require additional evidence.
Architecture constraints: Go core validates approvals/delegation; wrappers cannot certify approval sufficiency.
ADR required: yes
TDD first failing test(s): self-approval block, distinct approver failure, incompatible roles, unsigned delegation, approval scope mismatch, valid SOD allow.
Cost/perf impact: low
Chaos/failure hypothesis: missing role evidence or JIT-binding evidence under strict SOD policy must block rather than silently downgrade.

### Story A3.3: Circuit breakers and deterministic budgets

Priority: P1

Recommendation:
Stop runaway or repeated risky behavior with lock-safe deterministic counters.

Why:
Repeated blocked attempts, broad tool use, destructive calls, egress bursts, and
approval/JIT escalation attempts are risk signals that should stop execution.

Strategic direction:
Extend existing rate/destructive budget primitives into authorization-grade
state with fail-closed production semantics.

Expected benefit:
Gait can prevent repeated or runaway risky behavior before it becomes an
incident.

Tasks:
- Add budget controls for repeated blocked attempts, unique tools per run,
  destructive calls per identity/job/session, egress calls per window, execution
  window duration, optional token/API spend metadata, and approval/JIT
  escalation attempts.
- Use deterministic atomic state and lock-safe updates.
- Add reason codes that distinguish each budget type.
- Add `oss-prod` fail-closed behavior when required budget state is unavailable.
- Add explain JSON rate/budget status and proof refs.

Repo paths:
- `core/gate/rate_limit.go`
- `core/gate/budget.go`
- `core/fsx/`
- `core/schema/v1/gate/types.go`
- `schemas/v1/gate/budget_state.schema.json`
- `cmd/gait/gate.go`
- `docs/contracts/circuit_breakers.md`
- `CHANGELOG.md`

Run commands:
- `go test ./core/gate ./core/fsx ./cmd/gait`
- `make test-hardening`
- `make test-chaos`
- `make test-runtime-slo`
- `make prepush-full`

Test requirements:
- Repeated blocked attempts trip budget.
- Unique tools per run budget trips.
- Destructive calls per identity/job/session trip.
- Egress calls per window trip.
- Unavailable budget state fails closed in `oss-prod`.
- Concurrent gate eval invocations do not corrupt budget state.

Matrix wiring:
- Fast lane: gate and fsx tests.
- Core CI lane: contract tests for JSON state and reason codes.
- Risk lane: hardening, chaos, runtime SLO.
- Cross-platform lane: lock behavior tested on OS matrix.

Acceptance criteria:
- Counters are deterministic and lock-safe.
- Unavailable budget state fails closed in `oss-prod`.
- Reason codes distinguish budget type.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added deterministic circuit breakers for blocked attempts, tool breadth, destructive actions, egress, execution windows, spend metadata, and escalation attempts.
Semver marker override: [semver:minor]
Contract/API impact: additive policy/state schema, reason codes, and explain fields.
Versioning/migration impact: additive; state schema versioned for future migration.
Architecture constraints: budget state writes must be atomic and concurrency-safe; state unavailability follows fail-closed policy.
ADR required: yes
TDD first failing test(s): each budget type trips, concurrent updates remain deterministic, unavailable state fail-closed.
Cost/perf impact: medium; must meet runtime SLO for gate overhead.
Chaos/failure hypothesis: lock contention or partial budget write must not allow covered high-risk execution in `oss-prod`.

### Story A3.4: Context and memory boundary rules

Priority: P1

Recommendation:
Govern actions derived from untrusted context and memory.

Why:
Prompt or retrieval injection can turn external/tool-derived content into write
or egress actions unless provenance and context evidence are enforceable.

Strategic direction:
Treat context source, freshness, digest, and envelope status as authorization
inputs for writes and egress.

Expected benefit:
Gait can block or require approval/JIT for risky actions derived from untrusted,
external, tool-output, or stale memory context.

Tasks:
- Extend allowed provenance/source classifications to include `memory` while
  preserving `user`, `system`, `tool_output`, and `external`.
- Add policy controls requiring approval and/or JIT for writes or egress derived
  from untrusted context.
- Require context envelope for high-risk derived actions where policy demands it.
- Block stale or missing context evidence under strict policy.
- Add trace fields for context digest, source, source ref count, freshness, and
  trusted/untrusted decision.
- Add docs and examples for external context to egress block/approval.

Repo paths:
- `core/contextproof/`
- `core/gate/context_evidence.go`
- `core/gate/policy.go`
- `core/gate/intent.go`
- `core/schema/v1/context/`
- `core/schema/v1/gate/types.go`
- `schemas/v1/context/`
- `schemas/v1/gate/intent_request.schema.json`
- `docs/contracts/context_boundary.md`
- `CHANGELOG.md`

Run commands:
- `go test ./core/contextproof ./core/gate ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make test-chaos`
- `make prepush-full`

Test requirements:
- Untrusted external context to egress blocks or requires approval.
- Tool-output-derived write blocks or requires approval under policy.
- Missing required context evidence fails closed.
- Stale context envelope blocks.
- Trace records context digest/source/ref count.

Matrix wiring:
- Fast lane: contextproof and gate tests.
- Core CI lane: schema/golden tests.
- Risk lane: hardening and chaos for missing/stale/malformed envelopes.
- Cross-platform lane: fixed timestamps and temp files.

Acceptance criteria:
- Untrusted external context to egress blocks or requires approval.
- Missing required context evidence fails closed.
- Trace records context digest/source/ref count.

Changelog impact: required
Changelog section: Security
Draft changelog entry: Added context and memory boundary policy controls for untrusted-context writes and egress.
Semver marker override: [semver:minor]
Contract/API impact: additive source enum, policy controls, trace/explain fields, reason codes.
Versioning/migration impact: additive; existing context envelopes remain valid but may be insufficient for strict untrusted-source policies.
Architecture constraints: context proof remains privacy-aware and reference-first; no raw sensitive content by default.
ADR required: no
TDD first failing test(s): external-to-egress block, memory-to-write approval, stale envelope block, missing envelope fail-closed, valid envelope allow.
Cost/perf impact: low
Chaos/failure hypothesis: context envelope parsing failure under high-risk strict policy must block, not fall back to unverified provenance.

### Story A3.5: WRKR and Agent Action BOM consumption

Priority: P1

Recommendation:
Consume WRKR discovery and future Agent Action BOM metadata without making Gait
the scanner.

Why:
Gait should enforce policy based on discovery evidence, coverage status, and
credential/source metadata, while WRKR or another tool owns scanning.

Strategic direction:
Accept signed/digested inventory inputs as policy and proof context, not as a
new scanning subsystem.

Expected benefit:
Teams can prove that covered actions map to discovered tool inventories and
policy coverage without Gait owning repo history or scanning logic.

Tasks:
- Extend `--wrkr-inventory` support with digest/ref emission in trace and proof.
- Add future `--agent-action-bom` input with schema for BOM digest/ref, policy
  coverage status, proof coverage status, and credential/source metadata.
- Add policy controls to require BOM or WRKR evidence for high-risk actions.
- Add digest binding into signed proof surfaces.
- Add docs making clear that Gait consumes discovery output and does not own
  scanner logic.

Repo paths:
- `core/gate/context_wrkr.go`
- `core/gate/policy.go`
- `cmd/gait/gate.go`
- `schemas/v1/scout/`
- `schemas/v1/gate/action_bom.schema.json`
- `core/schema/v1/gate/types.go`
- `docs/contracts/action_bom.md`
- `examples/wrkr/`
- `CHANGELOG.md`

Run commands:
- `go test ./core/gate ./cmd/gait ./core/schema/...`
- `make test-contracts`
- `make test-hardening`
- `make prepush-full`

Test requirements:
- Required BOM missing blocks high-risk action.
- WRKR/BOM digest appears in signed trace/proof.
- Mismatched BOM digest blocks.
- Inventory enriches policy context deterministically.
- Gait does not perform scanning in tests or docs.

Matrix wiring:
- Fast lane: gate and CLI tests.
- Core CI lane: schema/golden tests.
- Risk lane: missing/mismatched evidence fail-closed.
- Cross-platform lane: inventory paths in temp dirs.

Acceptance criteria:
- Required BOM missing blocks high-risk action.
- BOM digest appears in signed proof.
- Gait does not own PR-history or repo-scanning logic.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added WRKR and Agent Action BOM evidence consumption for high-risk policy coverage and proof binding.
Semver marker override: [semver:minor]
Contract/API impact: additive CLI input, schema, trace/proof fields, policy controls.
Versioning/migration impact: additive; `--wrkr-inventory` remains supported while `--agent-action-bom` is introduced.
Architecture constraints: Gait consumes inventory/BOM evidence only; scanning remains external.
ADR required: no
TDD first failing test(s): missing BOM block, digest mismatch block, valid BOM allow, proof includes BOM digest.
Cost/perf impact: low
Chaos/failure hypothesis: unreadable or malformed BOM evidence required by policy must block in `oss-prod`.

## Wave 4: P0 End-To-End Product Proof

### Story A4.1: V1 execution authorization demo acceptance path

Priority: P0

Recommendation:
Ship one end-to-end demo proving standing privilege block, JIT requirement,
kill switch, and offline proof verification.

Why:
The North Star needs a concrete operator path that proves Gait can stop,
authorize, and prove privileged agent actions end to end.

Strategic direction:
Make the demo offline, deterministic, scriptable, and CI-backed.

Expected benefit:
Users, reviewers, and contributors can run one command to see the full
execution-time authorization value proposition.

Tasks:
- Add demo fixtures for one risky agent action with broad/static credential
  provenance.
- Add policy fixture that blocks standing privilege.
- Add approval fixture showing approval alone still blocks when JIT is required.
- Add broker fixture using valid approval plus scoped brokered credential to
  allow.
- Add kill-switch fixture that blocks the subsequent matching action.
- Build an authorization proof bundle and verify it offline.
- Add script and CI/acceptance wiring.
- Add docs walkthrough with expected `--json` outputs and no secrets.

Repo paths:
- `examples/authorization-demo/`
- `examples/policy/authorization-demo/`
- `scripts/test_authorization_demo.sh`
- `cmd/gait/*_test.go`
- `core/gate/testdata/`
- `core/pack/testdata/`
- `docs/authorization_demo.md`
- `README.md`
- `docs-site/public/llms.txt`
- `docs-site/public/llm/`
- `CHANGELOG.md`

Run commands:
- `bash scripts/test_authorization_demo.sh`
- `gait pack verify <artifact.zip> --json`
- `make test-scenarios`
- `make test-hardening`
- `make prepush-full`

Test requirements:
- Static credential path blocks.
- Approval without JIT blocks.
- Approval plus valid brokered scoped credential allows.
- Kill switch blocks subsequent matching action.
- Evidence pack verifies offline.
- Demo is hermetic and does not require network, cloud accounts, or secrets.

Matrix wiring:
- Fast lane: targeted CLI tests for demo fixtures.
- Core CI lane: scenario and contract tests.
- Acceptance lane: `bash scripts/test_authorization_demo.sh`.
- Risk lane: hardening for proof tamper and missing evidence.
- Cross-platform lane: shell script needs portable path handling or equivalent
  Go test wrapper for Windows.
- Release/UAT lane: include in local UAT once stable.

Acceptance criteria:
- Risky agent action arrives with broad/static credential provenance.
- Gait blocks standing privilege.
- Same action with approval but no JIT still blocks when JIT is required.
- Same action with valid approval plus brokered scoped credential allows.
- Kill switch blocks subsequent matching action.
- Evidence pack verifies offline.

Changelog impact: required
Changelog section: Added
Draft changelog entry: Added an offline execution-authorization demo proving standing-privilege blocking, JIT enforcement, kill-switch behavior, and proof bundle verification.
Semver marker override: [semver:minor]
Contract/API impact: example/demo contract, scenario script, docs, and acceptance fixture expectations.
Versioning/migration impact: additive; demo fixtures become compatibility examples for future releases.
Architecture constraints: demo must exercise Go CLI/core paths only; no mock policy decision in scripts.
ADR required: no
TDD first failing test(s): scenario script initially fails until the four decision transitions and pack verification pass.
Cost/perf impact: low to medium; demo should complete in under 60 seconds locally.
Chaos/failure hypothesis: deleting or tampering with any demo evidence file must make final offline verification fail.

## Explicit Non-Goals

- Do not add a hosted dashboard or require a network service for this work.
- Do not make Gait a credential issuer. Gait validates broker receipts and
  credential references only.
- Do not move policy decisions, proof verification, signing, hashing, or schema
  validation into Python wrappers, docs, or examples.
- Do not store raw secrets, raw credentials, private keys, or real customer data
  in traces, receipts, packs, fixtures, or docs.
- Do not implement WRKR scanning or PR-history analysis inside Gait.
- Do not break existing v1 pack/runpack/gate trace verification contracts.
- Do not change stable exit codes casually; any new code needs explicit tests
  and docs.

## Definition of Done

- Every touched public schema has valid/invalid fixtures and contract tests.
- Every new CLI JSON shape has golden tests with deterministic ordering.
- Every fail-closed behavior has tests for allow path, block path, malformed
  input, missing evidence, and unavailable state where relevant.
- Every proof bundle and linked receipt uses JCS canonicalization before hashing
  or signing.
- Every artifact-producing test uses `t.TempDir()` or equivalent.
- Docs and examples reflect implemented Go behavior, not roadmap intent.
- `CHANGELOG.md` has operator-facing entries for user-visible behavior.
- Required validation for implementation PRs includes targeted tests,
  `make test-contracts`, risk lanes named by each story, and `make prepush-full`.
- Final implementation can hand off the V1 demo with:
  - `bash scripts/test_authorization_demo.sh`
  - `gait pack verify <artifact.zip> --json`
  - deterministic JSON evidence proving the four decision transitions and kill
    switch block.
