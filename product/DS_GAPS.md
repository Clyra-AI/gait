# PLAN DS_GAPS: Documentation + Storyline Gap Closure (Zero-Ambiguity Execution Plan)

Date: 2026-02-14
Source of truth: `/Users/davidahmann/Projects/gait/README.md`, `/Users/davidahmann/Projects/gait/docs/architecture.md`, `/Users/davidahmann/Projects/gait/docs/flows.md`, `/Users/davidahmann/Projects/gait/docs/integration_checklist.md`, `/Users/davidahmann/Projects/gait/docs/sdk/python.md`, `/Users/davidahmann/Projects/gait/docs/ui_localhost.md`, `/Users/davidahmann/Projects/gait/examples/integrations/openai_agents/README.md`, user feedback batch (16 integration-clarity questions)
Scope: DS_GAPS only (documentation, onboarding narrative, diagrams, and validation gates for integration clarity; no runtime behavior changes in this phase)

This plan is written to be executed top-to-bottom with minimal interpretation. Each story includes concrete repository paths, tasks, acceptance criteria, and validation commands.

---

## Global Decisions (Locked for DS_GAPS)

- DS_GAPS is a clarity and adoption release track, not a runtime feature release.
- No policy engine semantics, artifact schema semantics, or exit-code contracts change in this plan.
- `docs/contracts/*` remains normative. Storyline docs must not contradict contract docs.
- Every user-facing claim must map to:
  - a runnable command
  - a concrete file/artifact output
  - a fail-closed statement where applicable
- We explicitly separate three planes in docs:
  - integration runtime plane (agent + adapter + tool boundary)
  - operator CLI plane (demo/verify/doctor/report)
  - CI/release plane (regress and quality gates)
- The docs must answer all 16 reported concerns directly and unambiguously.

---

## Concern Closure Matrix (All Reported Concerns)

| Concern ID | User Concern (Condensed) | Current State | DS_GAPS Closure | Priority |
|---|---|---|---|---|
| C-01 | "Explain what Gait does in simple language" | Partial | Add plain-language problem -> solution block at top of README and docs start page | P0 |
| C-02 | "How does this work with Gemini/Copilot/preloaded agents?" | Partial | Add managed-agent integration boundary guide with supported/unsupported interception tiers | P0 |
| C-03 | "Architecture diagram is too low-level/confusing" | Open | Add integration-first architecture diagram and move component internals to secondary section | P0 |
| C-04 | "In flow 1, is Go Core the agent?" | Open | Add explicit flow legend clarifying actor roles and what is not the agent runtime | P0 |
| C-05 | "In flow 1b, which part is the agent?" | Open | Add actor mapping annotations for 1b (operator/job/pack vs external runtime) | P0 |
| C-06 | "Flow 2 relation to runtime checks/policies" | Addressed but under-emphasized | Keep flow 2 and add direct bridge to concrete wrapper quickstart path | P1 |
| C-07 | "Flow 3 relation to agent/workspace?" | Partial | Clarify that flow 3 is incident->CI pipeline flow, not live agent runtime | P1 |
| C-08 | "High-risk approval trigger: CI or human-in-loop runtime?" | Partial | Add trigger matrix and runtime-vs-CI semantics with explicit exit-code mapping | P0 |
| C-09 | "MCP serve: what does it do; what is adapter; does it do all flows?" | Partial | Add MCP capability matrix (`proxy`/`bridge`/`serve`) and explicit non-goals | P0 |
| C-10 | "Flow 6: checkpoint support vs long-running only?" | Addressed but terse | Add session checkpoint intent statement and practical usage boundaries | P1 |
| C-11 | "Flow 7 delegation scope-down + IdP/token exchange?" | Partial | Add delegation boundary doc: what Gait validates vs what external IdP handles | P0 |
| C-12 | "Why CLI from Python? Should SDK duplicate CLI?" | Addressed | Add prominent "Python is thin subprocess layer" callout in integration docs | P1 |
| C-13 | "`gait demo` output unclear" | Partial (improved output) | Replace README hero with a <=60s narrated simple end-to-end scenario, move existing 20s demo lower, and add field legend docs | P0 |
| C-14 | "UI demo clearer than CLI but still unclear with agent flow" | Partial | Add UI-to-runtime relationship section and path from UI -> wrapper quickstart | P1 |
| C-15 | "Show demo in agent context" | Partial (exists in examples) | Promote agent-context quickstart into top-level onboarding path | P0 |
| C-16 | "Need one simple scenario of Gait between agent and tool" | Partial | Add one-page canonical scenario with allow/block/approval outcomes and artifacts | P0 |

---

## Deliverable Map (Planned Repository Touches)

```text
product/
  DS_GAPS.md

README.md
  - add plain-language block
  - add "managed agents boundary" summary
  - elevate agent-context quickstart path
  - surgically replace top hero demo asset with <=60s narrated simple end-to-end scenario
  - keep existing `gait_demo_20s` assets in a lower "fast proof" subsection
  - add demo output legend link

docs/
  architecture.md
    - integration-first architecture section (new primary)
    - component internals moved to secondary section
  flows.md
    - actor-role legend
    - explicit runtime/operator/CI boundaries per flow
    - approval trigger matrix references
  integration_checklist.md
    - explicit "managed/hosted agent" branch
    - promoted simple scenario path
  ui_localhost.md
    - explicit relationship to wrapped runtime path
  approval_runbook.md
    - runtime HITL vs CI policy gate clarifications
  policy_rollout.md
    - delegation + approval trigger references
  concepts/mental_model.md
    - single-paragraph plain-language explanation aligned with README
  sdk/python.md
    - prominent thin-wrapper reminder and CLI authority note

docs/new (planned)
  docs/agent_integration_boundary.md
    - managed agents, hosted copilots, and interceptability tiers
  docs/mcp_capability_matrix.md
    - proxy vs bridge vs serve and adapter definition
  docs/scenarios/simple_agent_tool_boundary.md
    - one simple scenario end-to-end
  docs/demo_output_legend.md
    - explain all key output fields from demo/tour

docs/assets + docs-site/public/assets (planned)
  gait_demo_simple_e2e_60s.cast
    - asciinema capture for narrated simple scenario
  gait_demo_simple_e2e_60s.gif
    - README top hero asset
  gait_demo_simple_e2e_60s.mp4
    - alternate video asset

examples/
  integrations/openai_agents/README.md
    - onboarding framing and links from top-level docs

scripts/ (planned)
  record_runpack_hero_demo.sh
    - add `DEMO_PROFILE=simple_e2e_60s` recording/rendering path
  test_docs_storyline_acceptance.sh
    - deterministic checks that key concern answers are present in docs
  test_demo_recording_validation.sh
    - validate duration <=60s, narration markers, and required command sequence in captured demo
```

---

## Success Metrics and Gate Criteria (Locked)

### Product Clarity Metrics

- `CL1`: A new reader can answer all 16 concern questions using docs alone without source-code inspection.
- `CL2`: Time-to-understand "where Gait sits in agent runtime" <= 5 minutes from `README.md` start.
- `CL3`: Time-to-first agent-context run (wrapper quickstart allow/block/require_approval) <= 15 minutes from clean checkout.
- `CL4`: Documentation explicitly distinguishes runtime vs operator vs CI planes in architecture and flows.
- `CL5`: README top demo asset is <=60s and explicitly narrates `intent -> gate -> allow/block -> trace -> runpack -> regress`, while the previous 20s asset remains available lower on the page.

### Engineering and Quality Gates

- `Q1`: Existing repo quality gates remain green (`make lint-fast`, `make test-fast`).
- `Q2`: Existing docs-site validation remains green (`make docs-site-check`).
- `Q3`: Existing integration smoke remains green (`make test-adoption`, `make test-adapter-parity`).
- `Q4`: New DS docs acceptance gate passes (`scripts/test_docs_storyline_acceptance.sh`) once implemented.
- `Q5`: Demo recording validation gate passes (`scripts/test_demo_recording_validation.sh`) once implemented.

### Release/Exit Gate

DS_GAPS track is complete only when all `CL1..CL5` and `Q1..Q5` are green and all `C-01..C-16` are marked closed in this plan.

---

## Epic 0: Canonical Language and Positioning

Objective: remove ambiguity about what Gait is, what it is not, and where it sits.

### Story 0.1: Add plain-language problem -> solution block

Tasks:
- Add a short block in `/Users/davidahmann/Projects/gait/README.md`:
  - Problem: autonomous agent operations need record/control/debug/proof.
  - Solution mapping:
    - `runpack`/`pack` -> record + evidence
    - `gate` -> policy decision before side effects
    - `regress` -> incident-to-CI guardrail
    - `guard`/`incident` -> compliance and incident packaging
- Add aligned summary in `/Users/davidahmann/Projects/gait/docs/concepts/mental_model.md`.

Acceptance criteria:
- Concern `C-01` closes with explicit 1:1 mapping.
- No wording conflicts with `/Users/davidahmann/Projects/gait/docs/contracts/primitive_contract.md`.

### Story 0.2: Add explicit "is/is not" boundary

Tasks:
- Add "Gait is not an orchestrator/model provider/hosted control plane" in README and docs start pages.
- Add "Gait requires interception at tool boundary for enforcement" callout near all integration entrypoints.

Acceptance criteria:
- Concern `C-02` partially addressed by boundary is now explicit.
- Concern `C-12` language is directly answered in docs.

---

## Epic 1: Integration-First Architecture and Flow Rewrite

Objective: make architecture and flows answer "how do I integrate this with my agent?" first.

### Story 1.1: Architecture page top section rewritten around integration

Tasks:
- In `/Users/davidahmann/Projects/gait/docs/architecture.md`:
  - Add a first diagram: `Agent Runtime -> Adapter/Wrapper -> gait gate eval (or mcp serve) -> Tool Executor`.
  - Show artifacts emitted (`trace`, `runpack/pack`, `regress fixtures`) as outputs from the boundary.
  - Keep existing Go-core component diagram as secondary "implementation internals."

Acceptance criteria:
- Concerns `C-03`, `C-04`, and `C-05` are directly answered on page.
- First diagram contains explicit actor labels: `Agent Runtime`, `Adapter`, `Gait CLI/Core`, `Tool`.

### Story 1.2: Flows page actor legend + plane labels

Tasks:
- Add top legend in `/Users/davidahmann/Projects/gait/docs/flows.md`:
  - Runtime plane
  - Operator plane
  - CI plane
- Add one-line "What this flow is/what this flow is not" under each flow (1, 1b, 2, 3, 4, 5, 6, 7).

Acceptance criteria:
- Concerns `C-04`, `C-05`, `C-07`, `C-08`, `C-09`, `C-10`, `C-11` have explicit answer lines in flow doc.

---

## Epic 2: Managed/Hosted Agent Integration Boundaries

Objective: answer "what if my agent runtime is controlled by another company?" with actionable guidance.

### Story 2.1: Add managed-agent boundary guide

Tasks:
- Add `/Users/davidahmann/Projects/gait/docs/agent_integration_boundary.md` with tiers:
  - Tier A: full wrapper/sidecar control (best fit)
  - Tier B: API middleware/proxy interception
  - Tier C: hosted/preloaded agents with limited interception (observe-only or partial guardrails)
- For each tier, define:
  - what Gait can enforce
  - what Gait can only observe
  - what remains external/non-addressable

Acceptance criteria:
- Concern `C-02` fully closes with explicit "supported vs constrained" matrix.

### Story 2.2: Add decision tree for choosing integration path

Tasks:
- Add "Can I intercept tool calls?" decision flow in README and integration checklist.
- Link directly to:
  - wrapper quickstart
  - mcp serve path
  - observe-only CI/report path for constrained environments

Acceptance criteria:
- New readers can select an integration path in <= 2 minutes.

---

## Epic 3: MCP and Adapter Semantics Clarification

Objective: remove ambiguity about MCP serve and adapter terminology.

### Story 3.1: Add MCP capability matrix

Tasks:
- Add `/Users/davidahmann/Projects/gait/docs/mcp_capability_matrix.md`:
  - `mcp proxy`: one-shot evaluation from payload
  - `mcp bridge`: alias behavior and intended usage notes
  - `mcp serve`: long-running HTTP evaluation service
- For each mode, include:
  - accepts
  - returns
  - what it does not do automatically
  - runtime enforcement responsibility

Acceptance criteria:
- Concern `C-09` closes with concrete matrix and non-goals.

### Story 3.2: Define "adapter" in one normative paragraph

Tasks:
- Add term definition in README + flows + MCP docs:
  - "adapter = payload translation layer from framework schema (`openai`, `anthropic`, `langchain`, `mcp`) into normalized `IntentRequest`."
- Link to command flags in usage docs.

Acceptance criteria:
- Adapter meaning is unambiguous and consistent across docs.

---

## Epic 4: Approval and Delegation Runtime Semantics

Objective: make runtime trigger and token boundary behavior explicit.

### Story 4.1: Approval trigger matrix (runtime vs CI)

Tasks:
- Extend `/Users/davidahmann/Projects/gait/docs/approval_runbook.md` and `/Users/davidahmann/Projects/gait/docs/policy_rollout.md` with:
  - when `require_approval` appears
  - expected exit codes in runtime and CI contexts
  - human-in-loop standard flow vs CI blocking semantics

Acceptance criteria:
- Concern `C-08` closes with explicit trigger/response table.

### Story 4.2: Delegation vs external IdP boundary

Tasks:
- Add explicit section in flow/delegation docs:
  - Gait validates signed delegation tokens and scope bindings.
  - Gait does not implement enterprise IdP/OIDC token exchange lifecycle.
  - Show integration point where IdP-issued identity can be mapped into delegation token issuance workflows.

Acceptance criteria:
- Concern `C-11` closes with explicit "in scope/out of scope" boundary statement.

---

## Epic 5: Demo and UI Narrative Improvements

Objective: ensure demo and UI outputs communicate what happened and why it matters for agent integration.

### Story 5.1: Add `gait demo` output legend

Tasks:
- Add `/Users/davidahmann/Projects/gait/docs/demo_output_legend.md` explaining:
  - `mode`
  - `run_id`
  - `bundle`
  - `ticket_footer`
  - `verify`
  - `next`
  - `metrics_opt_in`
- Link from README first-win section.

Acceptance criteria:
- Concern `C-13` closes with field-level explanation.

### Story 5.2: Promote agent-context quickstart into top onboarding path

Tasks:
- In README "Try It" and "First Win", add explicit next step:
  - run wrapper quickstart scenarios from `/Users/davidahmann/Projects/gait/examples/integrations/openai_agents/quickstart.py`.
- Keep existing demo/tour path, but provide clear branch for "agent context".

Acceptance criteria:
- Concern `C-15` closes with visible top-level path.

### Story 5.3: UI docs tie back to runtime boundary

Tasks:
- Update `/Users/davidahmann/Projects/gait/docs/ui_localhost.md` with a direct "UI is shell over CLI; runtime integration still requires wrapper/sidecar/MCP boundary."
- Link from UI docs to simple scenario doc.

Acceptance criteria:
- Concern `C-14` closes with explicit linkage.

### Story 5.4: Refresh README Hero Demo Asset With <=60s Narrated Scenario

Tasks:
- Update `/Users/davidahmann/Projects/gait/scripts/record_runpack_hero_demo.sh` to support a dedicated profile:
  - `DEMO_PROFILE=simple_e2e_60s`
- Use `asciinema` capture as the source recording for the new profile and produce:
  - `/Users/davidahmann/Projects/gait/docs/assets/gait_demo_simple_e2e_60s.cast`
  - `/Users/davidahmann/Projects/gait/docs/assets/gait_demo_simple_e2e_60s.gif`
  - `/Users/davidahmann/Projects/gait/docs/assets/gait_demo_simple_e2e_60s.mp4`
  - mirrored assets under `/Users/davidahmann/Projects/gait/docs-site/public/assets/`
- Ensure the recorded sequence includes typed narration lines between commands while remaining <=60 seconds and covering:
  - intent creation/input
  - gate eval allow
  - gate eval block or require_approval
  - trace emission
  - runpack/pack verification path
  - regress bootstrap/run
- Update README surgically:
  - replace current top hero asset with `gait_demo_simple_e2e_60s.gif` (and mp4 link)
  - keep existing `gait_demo_20s.gif`/`gait_demo_20s.mp4` in a lower subsection ("Fast 20-Second Proof")
- Add validation script `/Users/davidahmann/Projects/gait/scripts/test_demo_recording_validation.sh` to check duration ceiling, required markers, and asset presence.

Acceptance criteria:
- New narrated simple scenario asset appears as the top README hero.
- Existing 20-second asset remains available lower on README.
- New cast/gif/mp4 assets exist in both `docs/assets` and `docs-site/public/assets`.
- Duration is <=60 seconds and required command/narration markers are present.
- Concern `C-13` is fully closed by visual + textual demo clarity.

---

## Epic 6: Single Simple Scenario and Comprehension Validation

Objective: provide one canonical, minimal scenario that demonstrates where Gait sits and what it emits.

### Story 6.1: Add canonical simple scenario page

Tasks:
- Add `/Users/davidahmann/Projects/gait/docs/scenarios/simple_agent_tool_boundary.md`:
  - one tool call enters adapter
  - `gate eval` returns `allow|block|require_approval`
  - only allow executes side effect
  - trace emitted
  - runpack/pack built
  - regress fixture generated

Acceptance criteria:
- Concern `C-16` closes with a single path that can be copied and run.

### Story 6.2: Add concern-answer traceability test

Tasks:
- Add `/Users/davidahmann/Projects/gait/scripts/test_docs_storyline_acceptance.sh` that checks:
  - each concern `C-01..C-16` has at least one anchored answer section
  - key docs cross-link correctly
  - no dead links in newly introduced docs

Acceptance criteria:
- CI can fail if concern coverage regresses.

---

## Test and Validation Plan (Locked)

### Test Matrix

| Test Class | Goal | Command(s) | Expected |
|---|---|---|---|
| Docs integrity | links/render/build consistency | `make docs-site-check` | pass |
| CLI stability | ensure referenced outputs remain true | `go test ./cmd/gait -count=1` | pass |
| Integration smoke | ensure agent-context path remains runnable | `make test-adoption` | pass |
| Adapter parity | ensure cross-adapter references remain valid | `make test-adapter-parity` | pass |
| UAT local | ensure no regressions in broader acceptance | `make test-uat-local` | pass |
| New DS concern gate (planned) | enforce 16-concern answer coverage | `bash scripts/test_docs_storyline_acceptance.sh` | pass |
| Demo recording validation (planned) | enforce <=60s narrated hero integrity and asset placement | `DEMO_PROFILE=simple_e2e_60s bash scripts/record_runpack_hero_demo.sh` and `bash scripts/test_demo_recording_validation.sh` | pass |

### Validation Commands (Execution Bundle)

```bash
make lint-fast
go test ./cmd/gait -count=1
make docs-site-check
make test-adoption
make test-adapter-parity
make test-uat-local
DEMO_PROFILE=simple_e2e_60s bash scripts/record_runpack_hero_demo.sh
```

Planned additional gate (after script lands):

```bash
bash scripts/test_docs_storyline_acceptance.sh
bash scripts/test_demo_recording_validation.sh
```

### Manual Validation Checklist (Operator Readability)

- Reader can answer:
  - "Where does Gait run relative to my agent?"
  - "What must I intercept to enforce policy?"
  - "What does `mcp serve` do vs not do?"
  - "When does approval trigger and who acts?"
  - "How do I run one simple scenario end-to-end?"
- Reader can complete agent-context quickstart scenarios (allow, block, require_approval) without reading source code.
- README top demo visibly narrates one end-to-end boundary scenario in <=60 seconds.
- Existing 20-second demo remains accessible lower on the same page.

---

## Risk Register and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Docs drift from runtime behavior | high | require command-backed examples; add DS docs acceptance script |
| Over-rotation into verbosity | medium | keep a "fast path" section at top of each key page |
| Conflicting terminology across pages | high | add a shared glossary block for `agent runtime`, `adapter`, `wrapper`, `sidecar`, `operator` |
| Breaking existing onboarding | medium | additive docs changes; retain current command paths |
| Demo asset becomes stale or exceeds clarity budget | medium | enforce recording validation script with duration and command-marker checks |

---

## Exit Criteria (Definition of Done)

- All concern IDs `C-01..C-16` are explicitly closed with linked evidence.
- Architecture and flows are integration-first and actor-explicit.
- Managed-agent boundary and MCP capability matrix are published.
- Demo/UI docs include output/relationship clarifications.
- README hero demo is replaced with <=60s narrated simple scenario and previous 20s asset is preserved lower.
- Simple scenario page is live and linked from README/integration checklist.
- Validation bundle passes and DS concern gate is enforced in CI.

---

## Out of Scope (This Plan)

- Any Go runtime logic changes for policy, pack, runpack, regress, or MCP behavior.
- Any schema changes.
- Any new external service dependency.
- Any hosted dashboard/on-platform control plane work.
