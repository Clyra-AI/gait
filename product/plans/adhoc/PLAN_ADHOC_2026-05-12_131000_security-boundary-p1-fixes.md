# PLAN ADHOC: Security Boundary P1 Fixes

Date: 2026-05-12
Source: user-provided combined `code-review` and `app-audit` report
Profile: `gait`
Scope: `oss-prod` fail-closed defaults and reference-mode runpack privacy
Plan path: `product/plans/adhoc/PLAN_ADHOC_2026-05-12_131000_security-boundary-p1-fixes.md`

This plan converts the supplied P1 findings into an execution-ready backlog.
It does not overwrite `product/PLAN_NEXT.md` or any rolling plan file.

## Global Decisions (Locked)

- Go remains authoritative for policy parsing, verdict semantics, raw/reference
  capture behavior, artifact serialization, schemas, CLI output, exit codes,
  and MCP boundary enforcement.
- `oss-prod` is a fail-closed profile. It must not accept a policy whose
  unmatched action path can return `default_allow`.
- The `oss-prod` default-verdict guard must be shared by `gait gate eval`,
  `gait mcp proxy`, and `gait mcp serve`; wrappers and SDKs must inherit the
  boundary behavior instead of reimplementing it.
- Standard profile behavior remains configurable. This plan fixes the strict
  production profile contract without changing low-friction local development
  defaults unless an explicit policy requests strictness.
- Reference-mode runpacks must serialize references and digests by default, not
  raw tool args or raw result payloads.
- Raw payload retention is explicit raw capture behavior. Raw capture must be
  obvious in CLI JSON warnings, docs, fixtures, and changelog language.
- Digest normalization must still happen before stripping reference-mode
  payloads, so `args_digest`, `result_digest`, receipts, verify, diff, replay,
  and regression conversion remain deterministic.
- Tests that create policies, intents, runpacks, keys, journals, or packs use
  `t.TempDir()` or equivalent and never write artifacts to the source tree.
- Docs follow implementation. Any docs claim that `oss-prod` defaults to block
  or reference mode avoids raw data must be backed by command and artifact
  tests in the same change.

## Current Baseline (Observed)

Observed command evidence:

- `gait doctor --json`
- Result on 2026-05-12: `ok=true`, `status=warn`, failed checks `0`, warnings
  `1`.
- Warning: local registry cache is not initialized. Workdir, output directory,
  job runtime durability, temp directory, rate-limit lock, binary readiness, key
  source configuration, repo schema files, hooks path, and onboarding assets
  passed.

Observed audit evidence from the supplied report:

- `make lint-fast` passed, including repo skill validation, community index
  validation, repo hygiene, hook config, `go vet`, `golangci-lint`, `ruff`, and
  `mypy`.
- `make test-fast` passed, including Go package tests and Python SDK tests,
  `52 passed`.
- Docs consistency, docs storyline acceptance, and onboarding contract smoke all
  passed.
- Full `prepush-full`, CodeQL, hardening, chaos, and runtime SLO lanes were not
  run locally for the report.

Observed implementation baseline:

- `cmd/gait/gate.go` enforces some `oss-prod` checks, including no
  `--simulate`, `--key-mode prod`, and an explicit signing key source.
- `core/gate/policy.go` returns `policy.DefaultVerdict` with reason code
  `default_<verdict>` when no rule matches. With `default_verdict: allow`, an
  unmatched high-risk intent can still return `verdict:"allow"` and
  `reason_codes:["default_allow"]`.
- `cmd/gait/mcp.go` and `cmd/gait/mcp_server.go` route MCP proxy and serve
  requests through Gate evaluation, but they do not currently reject
  `default_verdict: allow` for `oss-prod` before evaluation.
- `cmd/gait/run_record.go` defaults `capture_mode` to `reference` and blocks
  raw context evidence unless `--unsafe-context-raw` is set.
- `core/runpack/record.go` computes digests from `IntentRecord.Args` and
  `ResultRecord.Result`, then serializes the records. Reference mode therefore
  can still write `args` and `result` objects into `intents.jsonl` and
  `results.jsonl`.
- `schemas/v1/runpack/intent.schema.json` and
  `schemas/v1/runpack/result.schema.json` make raw `args` and `result` optional
  fields but do not state that they are raw-mode only.
- `docs/policy_authoring.md` says `oss-prod` defaults to block, which currently
  drifts from the permissive unmatched-policy behavior.

## Exit Criteria

- `gait gate eval --profile oss-prod` rejects `default_verdict: allow` before
  emitting an allow verdict, with stable JSON error text and exit code.
- `gait mcp proxy --profile oss-prod` and `gait mcp serve --profile oss-prod`
  enforce the same policy guard as `gate eval`.
- Standard profile policies with `default_verdict: allow` continue to work and
  keep their existing verdict/reason-code behavior.
- Reference-mode runpacks omit `intents[].args` and `results[].result` after
  digesting and receipt normalization.
- Raw-mode runpacks may retain raw payload fields, but CLI output and docs make
  the privacy risk explicit.
- Verify, diff, replay, reduce, and regress flows remain deterministic with
  reference-mode runpacks whose raw payload fields were stripped.
- Public docs, examples, schema descriptions, docs-site LLM exports, and
  `CHANGELOG.md` match the implemented `oss-prod` and capture-mode contracts.
- Promotion to hardened `oss-prod` or default-private evidence claims is blocked
  until targeted tests plus `make prepush-full`, `make test-contracts`,
  `make test-hardening`, `make test-chaos`, `make test-runtime-slo`, and
  `make codeql` pass or have an explicit blocker recorded.

## Public API and Contract Map

- CLI contracts:
  - `gait gate eval --policy <policy.yaml> --intent <intent.json> --profile oss-prod --json`
  - `gait mcp proxy --policy <policy.yaml> --call <tool_call.json|-> --profile oss-prod --json`
  - `gait mcp serve --policy <policy.yaml> --profile oss-prod --json`
  - `gait run record --input <record.json> --capture-mode reference --json`
  - `gait run record --input <record.json> --capture-mode raw --json`
- Schema contracts:
  - `schemas/v1/gate/policy.schema.json`
  - `schemas/v1/gate/gate_result.schema.json`
  - `schemas/v1/runpack/manifest.schema.json`
  - `schemas/v1/runpack/intent.schema.json`
  - `schemas/v1/runpack/result.schema.json`
  - `schemas/v1/runpack/refs.schema.json`
- Go contracts:
  - `core/gate/` owns policy evaluation, default-verdict semantics,
    reason-code stability, and fail-closed behavior.
  - `core/mcp/` and `cmd/gait/mcp*.go` own boundary routing through Gate and
    must not weaken `oss-prod` policy checks.
  - `core/runpack/` owns reference/raw capture serialization, digest
    normalization, deterministic zips, verify, diff, replay, reduce, and
    session-derived runpacks.
  - `cmd/gait/` owns user-facing CLI flags, JSON output, warnings, help text,
    and exit codes.
- Documentation contracts:
  - `README.md`, `docs/`, `docs-site/public/llms.txt`,
    `docs-site/public/llm/`, `examples/integrations/`, and `CHANGELOG.md`
    update when public behavior or operator claims change.
  - Docs must not claim privacy-safe reference evidence or fail-closed
    `oss-prod` behavior before targeted tests prove those contracts.

## Docs and OSS Readiness Baseline

- First-screen docs should keep the execution-boundary story crisp: define
  policy, evaluate structured intent, enforce at the boundary, and verify
  artifacts offline.
- Policy docs must state that `standard` is configurable and `oss-prod` rejects
  permissive defaults that could allow unmatched actions.
- Runpack docs must distinguish digest/reference evidence from raw payload
  capture and clearly identify raw mode as an explicit privacy-sensitive choice.
- Examples must remain copy-pasteable offline and must not include real secrets,
  customer data, private keys, production URLs, or raw credentials.
- Docs-site LLM exports must be regenerated or surgically updated for changed
  public behavior so downstream agents receive the corrected contract.
- OSS trust baseline remains present, but launch language must stay "go with
  conditions" until the two P1s and promotion lanes are complete.

## Recommendation Traceability

| User recommendation or finding | Planned coverage |
|---|---|
| P1: `oss-prod` can allow unmatched high-risk actions under `default_verdict: allow` | Story SBP1.1 |
| Apply the same guard to MCP proxy/serve | Story SBP1.1 |
| P1: reference-mode runpacks can contain raw intent/result payloads | Story SBP1.2 |
| Add raw/reference regression and golden tests | Stories SBP1.1, SBP1.2 |
| Align policy, privacy, README, docs-site exports, schemas, examples, and changelog | Story SBP2.1 |
| Run promotion validation before hardened claims | Story SBP3.1 |

## Test Matrix Wiring

- Fast lane:
  - `make lint-fast`
  - `make test-fast`
  - targeted `go test` package commands listed per story
- Core CI lane:
  - command tests under `cmd/gait`
  - Gate tests under `core/gate`
  - MCP proxy/server tests under `cmd/gait` and `core/mcp`
  - runpack tests under `core/runpack`
  - schema and golden fixture tests for changed public artifacts
- Acceptance lane:
  - `scripts/test_docs_consistency.sh`
  - `scripts/test_docs_storyline_acceptance.sh`
  - onboarding contract smoke with a temp binary when README or docs first-win
    commands change
- Cross-platform lane:
  - GitHub Actions Linux, macOS, and Windows matrix for CLI, path handling,
    zip determinism, temp directories, and artifact read/write behavior
- Risk lane:
  - `make test-hardening`
  - `make test-chaos`
  - `make test-runtime-slo` when Gate/MCP hot-path checks or runpack
    serialization costs change
- Release/UAT lane:
  - `make prepush-full`
  - `make test-contracts`
  - `make codeql`
- Gating rule:
  - No hardened `oss-prod` or default-private evidence claim ships until the
    two P1 fixes have failing tests first, stable JSON/exit-code assertions,
    docs updates, changelog entries, and promotion-lane evidence.

## Minimum-Now Sequence

1. Add failing tests proving `oss-prod` rejects or blocks `default_verdict:
   allow` for `gate eval`, MCP proxy, and MCP serve.
2. Implement a shared `oss-prod` policy guard and wire it through Gate and MCP
   command paths.
3. Add failing tests proving reference-mode runpacks strip `args` and `result`
   while retaining deterministic digests and receipts.
4. Implement reference-mode payload stripping after digest normalization and add
   raw-mode CLI warnings.
5. Update docs, schema descriptions, examples, docs-site LLM exports, README
   claims, and changelog.
6. Run promotion validation lanes and record blockers before any hardened
   release or security-boundary promotion.

## Explicit Non-Goals

- Do not build hosted UI, dashboards, services, or cloud dependencies.
- Do not move policy evaluation, privacy decisions, signing, hashing,
  canonicalization, or artifact serialization into Python, wrappers, docs-site,
  or examples.
- Do not change standard profile semantics beyond preserving existing tests.
- Do not add raw customer data, real credentials, private keys, or production
  endpoints to fixtures.
- Do not rewrite runpack schema versioning or break existing v1 artifacts
  without an explicit migration plan and compatibility tests.
- Do not broaden this plan into unrelated Gate features such as freeze windows,
  credential broker recipes, or sandbox policy unless required to close these
  two P1 findings.

## Definition of Done

- Every story has failing tests committed before implementation where practical.
- `oss-prod` permissive-default behavior is impossible through `gate eval`, MCP
  proxy, and MCP serve.
- Reference-mode runpacks contain no raw intent args or raw result payload
  objects in `intents.jsonl` or `results.jsonl`.
- Raw mode remains available only as explicit capture behavior and emits
  operator-visible warnings.
- `verify`, `diff`, `replay`, `reduce`, `regress`, and schema validation remain
  deterministic on updated fixtures.
- Docs and changelog match code and tests, including docs-site LLM exports.
- Required fast, contract, hardening, chaos, runtime SLO, CodeQL, and
  prepush-full lanes either pass or have an explicit promotion blocker recorded.

## Wave 1: P1 Runtime And Privacy Blockers

### Story SBP1.1: Enforce `oss-prod` fail-closed defaults

Recommendation:
Reject permissive default policies in `oss-prod` so unmatched actions cannot
return `default_allow`.

Why:
The strict production profile currently enforces key mode and some broker
checks, but it can still allow unknown high-risk actions when a policy sets
`default_verdict: allow`.

Strategic direction:
Add one shared policy guard for `oss-prod` and call it from all command paths
that evaluate policy at the tool boundary.

Expected benefit:
Operators can trust that `oss-prod` is fail closed for unmatched actions, and
docs no longer overstate production safety.

Tasks:

- Add failing command tests in `cmd/gait/main_test.go` for `gait gate eval
  --profile oss-prod` with `default_verdict: allow`, a high-risk unmatched
  write intent, prod key mode, and a temp private key source.
- Add failing MCP proxy tests in `cmd/gait/mcp_test.go` for `--profile oss-prod`
  with the same permissive default policy.
- Add failing MCP serve handler tests in `cmd/gait/mcp_server_test.go` proving
  the server path returns a non-allow response and does not execute under the
  permissive default policy.
- Implement a shared guard in `cmd/gait/gate.go` or a small helper owned by the
  Gate command boundary that rejects `default_verdict: allow` when
  `profile == oss-prod`.
- Reuse the helper from `evaluateMCPProxyPayload`, which also covers
  `evaluateMCPServeRequest`.
- Preserve standard profile `default_verdict: allow` behavior and existing
  `default_allow` reason-code tests.
- Assert stable JSON error text, exit code, and no trace/runpack/pack output for
  rejected `oss-prod` policies.
- Update `docs/policy_authoring.md`, policy examples, README snippets,
  docs-site LLM exports, and `CHANGELOG.md`.

Repo paths:

- `cmd/gait/gate.go`
- `cmd/gait/mcp.go`
- `cmd/gait/mcp_server.go`
- `cmd/gait/main_test.go`
- `cmd/gait/mcp_test.go`
- `cmd/gait/mcp_server_test.go`
- `core/gate/policy.go`
- `core/mcp/proxy.go`
- `docs/policy_authoring.md`
- `README.md`
- `docs-site/public/llms.txt`
- `docs-site/public/llm/`
- `CHANGELOG.md`

Run commands:

- `go test ./cmd/gait ./core/gate ./core/mcp`
- `make test-contracts`
- `make test-hardening`
- `make test-chaos`
- `make prepush-full`

Test requirements:

- `oss-prod` plus `default_verdict: allow` exits with a stable invalid-policy
  or invalid-input code before an allow verdict can be emitted.
- `oss-prod` plus `default_verdict: block` preserves existing strict behavior.
- `standard` plus `default_verdict: allow` still allows unmatched low-risk
  fixtures with `reason_codes:["default_allow"]`.
- MCP proxy and serve inherit the same strict policy rejection.
- No trace, runpack, pack, approval audit, or export artifact is written for a
  rejected `oss-prod` permissive default.

Matrix wiring:
Fast lane for targeted command tests; core CI lane for Gate/MCP contract tests;
risk lane for fail-closed and no-artifact side effects; cross-platform lane for
temp key and artifact path behavior.

Acceptance criteria:

- A permissive default policy cannot produce `verdict:"allow"` in `oss-prod`.
- Gate, MCP proxy, and MCP serve produce consistent JSON errors and exit/status
  behavior.
- Existing standard profile fixtures remain unchanged.
- Docs and changelog describe the strict `oss-prod` default-verdict contract.

Changelog impact: required
Changelog section: Security
Draft changelog entry: Fixed `oss-prod` policy evaluation so permissive default policies cannot allow unmatched tool calls at the Gate or MCP boundary.
Semver marker override: [semver:patch]
Contract/API impact: tightens `oss-prod` CLI/MCP behavior and JSON error contract; standard profile remains unchanged.
Versioning/migration impact: no schema break; permissive `oss-prod` policies must change `default_verdict` to `block` or `require_approval`.
Architecture constraints: Gate/MCP boundary behavior stays in Go; wrappers inherit the rejection and must not add local allow fallbacks.
ADR required: no
TDD first failing test(s): `oss-prod` gate eval rejects default allow; MCP proxy rejects default allow; MCP serve rejects default allow; standard default allow still passes.
Cost/perf impact: low
Chaos/failure hypothesis: a malformed or rejected strict policy must fail before trace or artifact writes and must never downgrade to allow.

### Story SBP1.2: Strip raw payloads from reference-mode runpacks

Recommendation:
In reference mode, compute digests from raw args/results when provided, then
omit raw `args` and `result` objects from serialized runpack records.

Why:
The manifest can currently claim `capture_mode:"reference"` while
`intents.jsonl` and `results.jsonl` still contain sensitive raw payloads.

Strategic direction:
Treat reference mode as metadata plus digests. Preserve raw values only when the
caller explicitly selects raw capture.

Expected benefit:
Runpacks attached to CI, tickets, and incident reports match the default privacy
promise and avoid accidental credential/customer-data leakage.

Tasks:

- Add failing unit tests in `core/runpack/record_test.go` that build a
  reference-mode runpack from raw `Args` and `Result`, unzip it, and assert
  `args` and `result` are absent while `args_digest` and `result_digest` remain
  stable.
- Add failing CLI tests in `cmd/gait/main_test.go` or a focused run-record test
  file proving `gait run record --capture-mode reference --json` strips raw
  fields and `--capture-mode raw --json` preserves them with warnings.
- Implement post-normalization stripping in `core/runpack/record.go` for
  `captureMode == "reference"` after digest and receipt normalization.
- Ensure `normalizeDigestBearingFields` can digest raw values from
  `normalization.intent_args`, `normalization.result_payloads`, or existing
  `Args`/`Result` fields before stripping.
- Add CLI warnings in `cmd/gait/run_record.go` when raw mode is selected and,
  if useful, when reference mode stripped raw payload fields.
- Update runpack diff/replay/reduce tests where they depend on raw metadata, so
  metadata mode remains deterministic and full/raw mode remains explicit.
- Update schema descriptions for `schemas/v1/runpack/intent.schema.json` and
  `schemas/v1/runpack/result.schema.json` to state that raw fields are raw-mode
  only.
- Add golden fixtures for reference-mode stripped records and raw-mode retained
  records.
- Update runpack/privacy docs, README capture snippets, docs-site LLM exports,
  and `CHANGELOG.md`.

Repo paths:

- `cmd/gait/run_record.go`
- `cmd/gait/main_test.go`
- `core/runpack/record.go`
- `core/runpack/record_test.go`
- `core/runpack/diff.go`
- `core/runpack/diff_test.go`
- `core/runpack/read_replay_test.go`
- `core/runpack/reduce_test.go`
- `schemas/v1/runpack/intent.schema.json`
- `schemas/v1/runpack/result.schema.json`
- `schemas/v1/runpack/manifest.schema.json`
- `docs/`
- `README.md`
- `docs-site/public/llms.txt`
- `docs-site/public/llm/`
- `CHANGELOG.md`

Run commands:

- `go test ./core/runpack ./cmd/gait`
- `make test-contracts`
- `make test-hardening`
- `make test-chaos`
- `make prepush-full`

Test requirements:

- Reference mode strips `args` and `result` from serialized JSONL records after
  computing digests.
- Reference mode preserves `args_digest`, `result_digest`, refs, manifest file
  hashes, manifest digest, and signature verification.
- Raw mode preserves raw payload objects and emits a warning in JSON output.
- `runpack.VerifyZip`, `DiffRunpacks` metadata mode, `ReplayStub`,
  `ReduceToMinimal`, and regression bootstrap remain deterministic with
  stripped reference-mode payloads.
- Credential-shaped synthetic fixture values do not appear in generated
  reference-mode zip bytes.

Matrix wiring:
Fast lane for runpack unit and CLI tests; core CI lane for schema/golden
contracts; risk lane for privacy leakage and malformed payload cases;
cross-platform lane for deterministic zip bytes and path handling.

Acceptance criteria:

- A temp reference-mode runpack built from synthetic credential-shaped payloads
  contains no raw secret-shaped fields in `intents.jsonl` or `results.jsonl`.
- Raw mode remains explicit and visibly warned.
- Existing verifier and deterministic diff/replay flows continue to pass.
- Docs and changelog explain the default privacy fix and raw-mode opt-in.

Changelog impact: required
Changelog section: Security
Draft changelog entry: Fixed reference-mode runpack recording so raw intent arguments and raw result payloads are stripped after digesting unless raw capture is explicitly selected.
Semver marker override: [semver:patch]
Contract/API impact: tightens reference-mode artifact contents and CLI warnings; raw-mode artifact behavior remains explicit.
Versioning/migration impact: no schema version bump expected; existing raw-bearing reference artifacts still verify, but newly recorded reference artifacts omit raw payload fields.
Architecture constraints: runpack capture privacy is implemented in Go core after digest normalization, not in wrappers or docs.
ADR required: no
TDD first failing test(s): reference strips args/result while preserving digests; raw preserves args/result with warning; reference zip bytes lack credential-shaped payload strings.
Cost/perf impact: low
Chaos/failure hypothesis: digest or receipt normalization errors must fail closed rather than write a partially stripped or unverifiable runpack.

## Wave 2: Contract And Documentation Alignment

### Story SBP2.1: Align public contracts with fixed behavior

Recommendation:
Update docs, schemas, examples, and changelog so public claims match the two P1
runtime fixes.

Why:
The audit found docs/code drift on `oss-prod` defaults and reference-mode
privacy. The implementation is not done until user-facing contracts are true.

Strategic direction:
Patch only the public surfaces touched by the fixes, and keep examples offline,
deterministic, and safe by default.

Expected benefit:
Security reviewers, OSS evaluators, and downstream agents get the same behavior
from docs, examples, CLI JSON, and artifacts.

Tasks:

- Update `docs/policy_authoring.md` to state that `oss-prod` rejects
  `default_verdict: allow` and that standard mode remains policy-configurable.
- Update runpack/privacy docs to say reference mode stores digests and
  references, while raw mode is explicit and privacy-sensitive.
- Update schema descriptions for optional raw fields in
  `schemas/v1/runpack/intent.schema.json` and
  `schemas/v1/runpack/result.schema.json`.
- Update README first-value or security-boundary snippets only where they claim
  `oss-prod` fail-closed behavior or reference-only evidence.
- Update `docs-site/public/llms.txt` and relevant `docs-site/public/llm/*.md`
  exports for the corrected claims.
- Add or update examples under `examples/policy/` and runpack examples so they
  demonstrate strict defaults and reference-mode privacy without secrets.
- Add `CHANGELOG.md` entries under `## [Unreleased]`.

Repo paths:

- `README.md`
- `docs/policy_authoring.md`
- `docs/`
- `docs-site/public/llms.txt`
- `docs-site/public/llm/`
- `examples/policy/`
- `examples/integrations/`
- `schemas/v1/runpack/intent.schema.json`
- `schemas/v1/runpack/result.schema.json`
- `CHANGELOG.md`

Run commands:

- `scripts/test_docs_consistency.sh`
- `scripts/test_docs_storyline_acceptance.sh`
- `make lint-fast`
- `make test-contracts`
- `make prepush-full`

Test requirements:

- Docs consistency checks pass.
- Storyline acceptance passes with the corrected first-value and
  security-boundary claims.
- Any docs command snippets that changed are covered by command tests or an
  onboarding smoke test.
- Docs-site LLM exports contain no stale claim that reference mode can include
  raw payloads by default or that `oss-prod` permits default allow.

Matrix wiring:
Fast lane for docs checks and lint; core CI lane for contract tests tied to
changed snippets; acceptance lane for storyline/onboarding; cross-platform lane
for docs command snippets where they invoke the CLI.

Acceptance criteria:

- Docs, examples, and LLM exports agree with code and command tests.
- Changelog entries are operator-facing and placed under `## [Unreleased]`.
- No new docs example includes raw secrets, private keys, customer data, or real
  destructive targets.

Changelog impact: required
Changelog section: Fixed
Draft changelog entry: Aligned policy and runpack documentation with strict `oss-prod` defaults and reference-mode privacy behavior.
Semver marker override: [semver:patch]
Contract/API impact: public docs and schema descriptions align with tightened runtime and artifact contracts.
Versioning/migration impact: no migration beyond adopting strict `oss-prod` policies and understanding raw-mode opt-in.
Architecture constraints: docs must cite Go CLI and schema behavior as source of truth.
ADR required: no
TDD first failing test(s): docs consistency or docs storyline fixture showing stale `oss-prod` or reference-mode claims.
Cost/perf impact: low
Chaos/failure hypothesis: stale docs can cause operators to deploy a fail-open policy or leak raw data, so docs checks must gate promotion.

## Wave 3: Promotion Validation

### Story SBP3.1: Run hardened promotion lanes before release claims

Recommendation:
After the P1 fixes and docs alignment land, run the full promotion matrix before
claiming hardened `oss-prod` or default-private evidence readiness.

Why:
Fast gates passed in the report, but hardening, chaos, runtime SLO, CodeQL, and
full prepush lanes were not run locally.

Strategic direction:
Treat this as validation and evidence collection, not feature development.
Record blockers instead of softening contracts.

Expected benefit:
Release and security-boundary promotion decisions are based on deterministic
command evidence rather than audit prose alone.

Tasks:

- Run the full validation commands listed below on the branch that contains
  Stories SBP1.1, SBP1.2, and SBP2.1.
- Capture JSON anchors where available, including `gait doctor --json` and at
  least one strict `gait gate eval --profile oss-prod --json` negative fixture.
- Confirm no generated runpack or temporary evidence artifact is written to the
  source tree.
- If a lane fails due to repo-fixable issues in the touched surface, open a
  follow-up fix story or patch before release promotion.
- If a lane is blocked by external tooling, permissions, or unavailable hosted
  services, record the blocker and keep the release claim gated.

Repo paths:

- `cmd/gait/`
- `core/gate/`
- `core/mcp/`
- `core/runpack/`
- `schemas/v1/`
- `docs/`
- `docs-site/public/`
- `CHANGELOG.md`

Run commands:

- `gait doctor --json`
- `gait gate eval --policy <policy.yaml> --intent <intent.json> --profile oss-prod --json`
- `make lint-fast`
- `make test-fast`
- `make test-contracts`
- `make test-hardening`
- `make test-chaos`
- `make test-runtime-slo`
- `make codeql`
- `make prepush-full`

Test requirements:

- Full promotion lanes pass on a clean worktree or have explicit blockers.
- Negative `oss-prod` fixture proves permissive default rejection.
- Reference-mode runpack fixture proves no raw payload leakage.
- CodeQL and hardening lanes do not identify unresolved security regressions in
  the touched paths.

Matrix wiring:
Fast lane for lint/test-fast; core CI lane for command/schema contracts;
acceptance lane for docs/onboarding; risk lane for hardening, chaos, and runtime
SLO; release/UAT lane for CodeQL and prepush-full.

Acceptance criteria:

- Promotion evidence is sufficient to change the audit verdict from "no-go for
  hardened claims" to "go for hardened `oss-prod` and default-private evidence
  claims", or blockers are explicitly documented.
- No lane is skipped silently.
- Any blocker has owner, command, failure summary, and next action.

Changelog impact: not required
Changelog section: none
Draft changelog entry: none
Semver marker override: none
Contract/API impact: no new contract beyond validating the completed fixes.
Versioning/migration impact: none.
Architecture constraints: validation must not weaken fail-closed or privacy contracts to pass.
ADR required: no
TDD first failing test(s): none; this story executes the completed test matrix.
Cost/perf impact: medium
Chaos/failure hypothesis: hardening or chaos lanes may expose missed fail-open or leakage paths; those block promotion until fixed.
