# PLAN ADOPTION: Gait (Zero-Ambiguity Adoption Execution Plan)

Date: 2026-02-06
Source of truth: `product/PRD.md`, `product/ROADMAP.md`, `product/PLAN_v1.md`, `README.md`
Scope: adoption work for v1 through v1.5 outcomes (developer activation, team rollout, CI embedding, enterprise-ready expansion)

This plan is written for direct execution with minimal interpretation. Every story defines owner-facing tasks, repo paths, run commands, and acceptance criteria.

---

## Global Adoption Decisions (Locked)

- Primary go-to-market motion: **product-led** (self-serve first, sales-assisted second).
- Primary activation wedge: **incident -> deterministic regression** in under 15 minutes.
- First-user path: **one path only**:
  - install
  - `gait demo`
  - `gait verify`
  - `gait regress init --from <run_id>`
  - `gait regress run`
- Runtime boundary rule: adoption guidance always enforces at **tool-call boundary**, never prompt boundary.
- Default rollout mode for new teams: **observe-first**, then **enforce high-risk tools**.
- Artifacts are the trust surface:
  - runpack
  - regress result (+ optional JUnit)
  - gate traces
  - evidence pack
- Product metrics are mandatory for adoption decisions; no roadmap changes without metric evidence.

---

## PLG Operating Principles (Applied to Gait)

These operational principles guide prioritization and sequencing:

1. Fast time-to-value over broad feature count.
2. One clear entry point over many optional paths.
3. Self-serve defaults before enterprise process.
4. Product usage signals drive expansion and sales-assist.
5. Friction removal is a product requirement (not "docs polish").
6. Clear free-to-paid boundary based on governance value.
7. In-product success mechanisms (`doctor`, templates, examples).
8. Measure activation and conversion weekly.
9. Protect trust via deterministic outputs and stable contracts.
10. Keep expansion tied to the execution path, not dashboards.

---

## North-Star Metrics and Definitions (Locked)

### Activation

- `A1`: First successful `gait demo`.
- `A2`: First successful `gait verify` on produced runpack.
- `A3`: First successful `gait regress init`.
- `A4`: First successful `gait regress run`.
- Activation complete: `A1+A2+A3+A4` within 24 hours of install.

### Expansion

- `E1`: Repository has at least 1 gated high-risk tool.
- `E2`: Repository has at least 1 regression fixture in CI.
- `E3`: Team has at least 1 signed gate trace reviewed in incident or change workflow.

### Retention

- `R1`: Weekly active repositories running at least one Gait command.
- `R2`: Weekly regress runs per active repository.
- `R3`: Percent of production tool calls routed through wrapped/gated path.

### Quality and Trust

- `Q1`: Median activation time (`install -> first regress run`) <= 30 minutes.
- `Q2`: Doc-run mismatch rate < 5% (examples that do not work as written).
- `Q3`: Support tickets caused by ambiguous CLI output <= defined threshold.

---

## Adoption Ladder (Target User Journey)

1. First 5 minutes: prove deterministic artifact loop.
2. First day: convert one incident/run into deterministic regression.
3. First week: gate one high-risk tool with approval flow.
4. First month: policy rollout to team repos with CI enforcement.
5. Quarter: enterprise evidence and governance packs in routine use.

---

## Epic A0: Adoption Data and Instrumentation Foundation

Objective: make adoption measurable without violating offline-first defaults.

### Story A0.1: Define telemetry/event contract (offline-safe)

Tasks:
- Define local event schema for CLI usage and milestone events:
  - `schemas/v1/scout/adoption_event.schema.json` (or adoption schema under `schemas/v1/registry/` if preferred)
- Add Go type mirror:
  - `core/schema/v1/scout/` (or matching package for chosen schema location)
- Include event fields:
  - command name
  - success/failure
  - exit code
  - elapsed_ms
  - milestone tags (`A1`-`A4`, `E1`-`E3`)
  - anonymized environment metadata (OS, version)

Acceptance criteria:
- Schema validates in CI.
- Event records can be written locally without network access.

### Story A0.2: Add opt-in local usage logging

Tasks:
- Add opt-in flag/env for local metrics file:
  - e.g. `GAIT_ADOPTION_LOG=./gait-out/adoption.jsonl`
- Implement write-on-success/failure events in CLI command router:
  - `cmd/gait/main.go`
- Keep default behavior no-op unless opt-in enabled.

Acceptance criteria:
- When enabled, every major command appends a valid event record.
- No behavior change when unset.

### Story A0.3: Add adoption report command

Tasks:
- Add CLI helper:
  - `gait doctor adoption --from <events.jsonl> --json`
- Summarize activation funnel and failure points from local event logs.

Acceptance criteria:
- Command outputs deterministic JSON with activation status and blockers.

---

## Epic A1: Positioning and README Conversion Funnel

Objective: make README both implementation guide and adoption asset.

### Story A1.1: Rewrite README to a single conversion path

Tasks:
- Ensure top section sequence is fixed:
  - promise
  - install
  - 5-minute demo
  - verify
  - incident -> regression
  - gate high-risk tools
- Keep all commands copy/paste-ready and OS-safe.
- Include expected outputs for each command.
- Add "why this matters" section focused on:
  - controllability
  - incident debuggability
  - policy enforcement

Repo paths:
- `README.md`

Acceptance criteria:
- New user can execute path end-to-end without reading other docs.

### Story A1.2: Add architecture and integration diagrams

Tasks:
- Add concise architecture section:
  - tool boundary wrapper -> `gait gate eval` -> executor -> trace/runpack
- Add "no bypass" pattern (only wrapped tools exposed to agents).

Repo paths:
- `README.md`
- `docs/` if created for diagrams, otherwise keep in README.

Acceptance criteria:
- Integration model is unambiguous for Python users.

### Story A1.3: Version-aligned release notes for v1.1-v1.5

Tasks:
- Add "What's new by milestone" section tied to roadmap:
  - v1.1 coverage and pack foundations
  - v1.2 enforcement depth
  - v1.3 MCP proxy
  - v1.4 evidence packs
  - v1.5 skills

Acceptance criteria:
- Users understand adoption path without reading roadmap first.

---

## Epic A2: Zero-Friction Onboarding Assets

Objective: reduce setup toil to near zero.

### Story A2.1: One-command quickstart script (offline-safe)

Tasks:
- Add script:
  - `scripts/quickstart.sh`
- Script validates:
  - `gait` binary present
  - writable output dir
  - runs `gait demo`
  - runs `gait verify`
  - prints next command for `regress init`

Acceptance criteria:
- Script completes on macOS/Linux in under 2 minutes.

### Story A2.2: Framework quickstart guides (top 3)

Tasks:
- Add guides with minimal code:
  - `examples/integrations/openai_agents/`
  - `examples/integrations/langchain/`
  - `examples/integrations/autogen/`
- Each guide must show:
  - wrapped tool call path
  - gate eval
  - block behavior
  - trace artifact path

Acceptance criteria:
- Each guide includes exact runnable commands and expected outputs.

### Story A2.3: Troubleshooting-first onboarding

Tasks:
- Add onboarding troubleshooting section:
  - map frequent failures -> single command fixes
- Extend `gait doctor` guidance for onboarding-specific failures.

Repo paths:
- `README.md`
- `core/doctor/`
- `cmd/gait/doctor.go`

Acceptance criteria:
- Top 10 onboarding failures have deterministic fix guidance.

---

## Epic A3: SDK and Wrapper Adoption Path

Objective: make correct integration the easiest integration.

### Story A3.1: Publish canonical Python wrapper pattern

Tasks:
- Add canonical wrapper example using existing SDK primitives:
  - `sdk/python/gait/adapter.py`
  - `sdk/python/gait/client.py`
- Add "must-do" requirements in docs:
  - only wrapped tools exposed
  - fail-closed on gate evaluation failure
  - no raw credential exposure in agent context

Acceptance criteria:
- Copy-paste wrapper example works and enforces block/approval behavior.

### Story A3.2: Add wrapper conformance tests

Tasks:
- Add tests that verify:
  - blocked verdict prevents execution
  - require_approval prevents execution without token
  - dry_run skips execution
  - allow executes exactly once

Repo paths:
- `sdk/python/tests/`

Acceptance criteria:
- Conformance tests pass and prevent regression of enforcement semantics.

### Story A3.3: Add integration checklist for app teams

Tasks:
- Add checklist doc:
  - `docs/integration_checklist.md` (or `README` section if docs dir avoided)
- Checklist covers:
  - tool registration
  - wrapper enforcement
  - trace persistence
  - runpack recording
  - CI regression

Acceptance criteria:
- Teams can self-audit integration readiness in < 30 minutes.

---

## Epic A4: Policy Templates and Safe Rollout Defaults

Objective: prevent policy authoring from becoming the adoption bottleneck.

### Story A4.1: Starter policy packs by risk tier

Tasks:
- Add templates:
  - `examples/policy/base_low_risk.yaml`
  - `examples/policy/base_medium_risk.yaml`
  - `examples/policy/base_high_risk.yaml`
- Include explicit allow/block/require_approval examples.

Acceptance criteria:
- `gait policy test` passes against fixture intents for each template.

### Story A4.2: Observe -> enforce rollout guide

Tasks:
- Document staged rollout:
  - simulate mode
  - dry_run mode
  - require_approval for high-risk tools
  - enforce mode
- Include exit-code handling matrix for CI/runtime.

Acceptance criteria:
- Teams can move from observe to enforce without service interruption.

### Story A4.3: Approval runbooks

Tasks:
- Add operational runbook:
  - token minting
  - key handling
  - TTL/scoping policy
  - incident audit steps

Repo paths:
- `docs/approval_runbook.md` (or `README` appendix)
- `SECURITY.md` for key-management references

Acceptance criteria:
- Security reviewers can execute approval workflow from docs only.

---

## Epic A5: CI Adoption Kits and Guardrails

Objective: make "incident to regression in CI" turnkey.

### Story A5.1: CI templates for common providers

Tasks:
- Add reusable snippets:
  - GitHub Actions example under `.github/workflows/`
  - generic shell CI snippet in docs
- Template flow:
  - restore fixture
  - `gait regress run --json --junit=...`
  - upload artifacts
  - fail on stable exit codes

Acceptance criteria:
- A new repo can enable deterministic regress checks in one PR.

### Story A5.2: Add nightly E2E/integration profile recommendation

Tasks:
- Document test cadence:
  - PR: fast deterministic unit + core integration
  - nightly: full integration/e2e/perf profile
- Add sample nightly workflow file:
  - `.github/workflows/adoption-nightly.yml`

Acceptance criteria:
- Guidance clearly separates speed-vs-depth without weakening correctness.

### Story A5.3: Policy compliance checks in CI

Tasks:
- Add example CI job for:
  - `gait policy test` with canonical fixture set
  - failure summary with reason codes

Acceptance criteria:
- Policy regressions are caught before merge.

---

## Epic A6: Distribution and Install Experience

Objective: remove install friction while preserving release integrity.

### Story A6.1: Stabilize direct binary install path

Tasks:
- Ensure release assets include:
  - platform binaries
  - checksums
  - signature/provenance pointers
- Add install/verify commands to README for each platform.

Acceptance criteria:
- Users can install and verify binary integrity in < 5 minutes.

### Story A6.2: Prepare Homebrew tap (gated until stable release)

Tasks:
- Create/update release checklist section:
  - criteria to open Homebrew tap
  - formula update workflow
  - rollback process
- Defer public tap publication until stable CLI contracts are frozen.

Repo paths:
- `README.md`
- `CONTRIBUTING.md`
- `product/ROADMAP.md` milestone note

Acceptance criteria:
- Homebrew publication is ready-to-run when stability gate is met.

### Story A6.3: Add install diagnostics to doctor

Tasks:
- Extend `gait doctor` with:
  - binary path/version checks
  - permissions
  - key/config sanity

Acceptance criteria:
- Install failures produce deterministic and actionable output.

---

## Epic A7: Community-Led Growth Loops

Objective: increase self-serve adoption and repeat usage.

### Story A7.1: Public example packs and reproducible demos

Tasks:
- Add curated demo scenarios:
  - incident reproduction
  - policy block on injected intent
  - approval flow success/failure
- Keep all examples offline-safe.

Acceptance criteria:
- Every scenario has expected outputs and deterministic result checks.

### Story A7.2: "Paste-into-ticket" workflow enablement

Tasks:
- Add explicit templates/snippets for:
  - incident tickets
  - PR descriptions
  - postmortem sections
- Include run_id + verify command conventions.

Acceptance criteria:
- Teams can standardize incident evidence format in one sprint.

### Story A7.3: Contribution paths for adapters and policies

Tasks:
- Add contribution guide sections for:
  - adapter examples
  - policy packs
  - deterministic fixture contributions

Repo paths:
- `CONTRIBUTING.md`
- `examples/`

Acceptance criteria:
- External contributors can add high-quality adoption assets with low reviewer overhead.

---

## Definition of Done (Adoption Work)

- Every new adoption artifact has:
  - exact commands
  - expected outputs
  - deterministic behavior
- All docs examples are validated in CI or scripted smoke checks.
- No onboarding path requires hidden setup knowledge.
- Integration patterns enforce tool-boundary control and fail-closed behavior.
- Changes improve at least one locked adoption metric (`A*`, `E*`, `R*`, `Q*`).

