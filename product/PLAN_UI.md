# PLAN UI: Gait Localhost UX and Aha Surface (Zero-Ambiguity Build Plan)

Date: 2026-02-12
Source of truth: `product/PLAN_v1.md`, `product/PLAN_v2.3.md`, `product/PLAN_SITE.md`, `README.md`, `docs/integration_checklist.md`, `docs/ci_regress_kit.md`
Scope: optional localhost UI only (adoption, wow/aha, UX acceleration). No hosted control plane. No policy/runtime authority moves out of Go.

This plan is written to be executed top-to-bottom with minimal interpretation. Each story includes concrete repo paths, commands, and acceptance criteria.

---

## Global Decisions (Locked for UI plan)

- UI is an adoption accelerator, not a new product core.
- Go CLI remains authoritative for:
  - policy evaluation
  - signing/verification
  - artifact generation
  - schema validation
- UI must call existing `gait` commands with `--json`; no duplicate policy or verification logic in TypeScript.
- Default runtime mode is local-only:
  - `127.0.0.1` bind
  - offline-capable
  - no outbound dependency for core flow
- UI is optional and non-blocking; CLI-first path remains primary and fully supported.
- Visual direction is intentionally bold and operator-focused, inspired by `fabra_oss/playground`:
  - dark gradient command-center atmosphere
  - strong status color language
  - live execution feedback
  - clear copy/paste command affordances
- Single command entrypoint:
  - `gait ui`
- End-user install should not require Node for runtime:
  - frontend built at release time
  - static assets served by Go binary
- Dependency freshness is a release gate (see Epic 0.2).

---

## Product Outcomes (Locked)

- Time-to-first-wow: user sees verifiable artifact proof in <= 60 seconds.
- Time-to-adoption: user reaches deterministic CI regression path in <= 15 minutes.
- UX clarity: user can understand block/approval vs allow semantics without reading long docs.
- Conversion utility: UI doubles as a demoable marketing asset while remaining technically honest.

---

## Repository Layout (Created in Epic 0)

```
.
├─ cmd/gait/
│  └─ ui.go                         # gait ui command entrypoint
├─ core/ui/                         # Go-side localhost UI server + adapters
│  ├─ server.go
│  ├─ handlers.go
│  └─ contracts.go
├─ ui/local/                        # frontend source (build-time dependency)
│  ├─ package.json
│  ├─ src/
│  └─ dist/                         # static build output (gitignored)
├─ internal/uiassets/
│  └─ embed.go                      # go:embed of built static assets
├─ docs/
│  ├─ ui_localhost.md               # end-user guide
│  └─ contracts/ui_contract.md      # API surface between frontend and Go
├─ scripts/
│  ├─ ui_build.sh
│  ├─ ui_sync_assets.sh
│  └─ check_ui_deps_freshness.sh
├─ .github/workflows/
│  ├─ ci.yml                        # UI build/test path filters
│  └─ ui-nightly.yml                # dependency freshness + e2e smoke
└─ product/
   └─ PLAN_UI.md
```

---

## Dependency Baseline (Validated 2026-02-12)

Verified via `npm view`:

- `next`: `16.1.6`
- `react`: `19.2.4`
- `react-dom`: `19.2.4`
- `typescript`: `5.9.3`
- `tailwindcss`: `4.1.18`
- `eslint-config-next`: `16.1.6`

Supporting UI libs (optional, if used):

- `@tanstack/react-query`: `5.90.21`
- `zustand`: `5.0.11`
- `zod`: `4.3.6`
- `lucide-react`: `0.563.0`
- `sonner`: `2.0.7`
- `@radix-ui/react-dialog`: `1.1.15`
- `@radix-ui/react-tooltip`: `1.2.8`
- `framer-motion`: `12.34.0`
- `date-fns`: `4.1.0`

Rules:

- Do not introduce stale major versions without explicit rationale.
- Use exact or bounded-compatible versions with lockfile committed.
- Add automated freshness checks in CI/nightly.

---

## Epic 0: Foundations, Contracts, and Dependency Hygiene

Objective: establish a maintainable UI foundation that cannot diverge from CLI truth.

### Story 0.1: Scaffold localhost UI architecture

Tasks:
- Add `gait ui` command.
- Implement Go HTTP server for local UI APIs.
- Add frontend scaffold under `ui/local/`.
- Define explicit UI API contract doc.

Repo paths:
- `cmd/gait/ui.go`
- `core/ui/server.go`
- `core/ui/handlers.go`
- `docs/contracts/ui_contract.md`
- `ui/local/`

Commands:
- `go build -o ./gait ./cmd/gait`
- `./gait ui --help`

Acceptance criteria:
- `gait ui` starts local server and serves frontend successfully.
- No UI route performs policy/verify logic without invoking Go-side command handlers.

### Story 0.2: Enforce up-to-date dependency policy

Tasks:
- Initialize `ui/local/package.json` with validated baseline versions.
- Add dependency freshness script:
  - compare installed versions vs latest npm registry for allowlisted packages.
- Add monthly freshness check in CI/nightly workflow.

Repo paths:
- `ui/local/package.json`
- `ui/local/package-lock.json`
- `scripts/check_ui_deps_freshness.sh`
- `.github/workflows/ui-nightly.yml`

Commands:
- `cd ui/local && npm ci`
- `bash scripts/check_ui_deps_freshness.sh`

Acceptance criteria:
- Freshness check fails on stale dependencies outside policy window.
- CI artifact includes machine-readable freshness report.

### Story 0.3: Build and embed pipeline

Tasks:
- Build UI static assets with reproducible output.
- Embed assets via `go:embed`.
- Add deterministic asset sync script and docs.

Repo paths:
- `scripts/ui_build.sh`
- `scripts/ui_sync_assets.sh`
- `internal/uiassets/embed.go`

Commands:
- `bash scripts/ui_build.sh`
- `go test ./cmd/gait ./core/ui/...`

Acceptance criteria:
- Running the same build inputs yields stable asset hash manifest.
- Binary can serve embedded assets without Node runtime.

---

## Epic 1: Aha/Wow First-Run Experience

Objective: deliver a 60-second emotional and technical "this is useful" moment.

### Story 1.1: Guided first-run flow

Tasks:
- Implement stepper UX:
  - Step 1: run `gait demo`
  - Step 2: run `gait verify`
  - Step 3: show runpack path and ticket footer
- Include live progress states and terminal-equivalent JSON view.

Repo paths:
- `ui/local/src/app/page.tsx`
- `ui/local/src/components/first-run/*`
- `core/ui/handlers.go`

Commands:
- `./gait ui`
- through UI: run demo + verify

Acceptance criteria:
- New user can complete first-run flow with no manual command typing.
- UI shows exact command(s) executed and captured outputs.

### Story 1.2: Artifact success panel

Tasks:
- Add persistent panel with:
  - `run_id`
  - runpack path
  - verify status
  - manifest digest
  - ticket footer copy button
- Link each field to originating JSON keys.

Repo paths:
- `ui/local/src/components/artifacts/*`
- `cmd/gait/run_receipt.go` (read-only integration if needed)

Acceptance criteria:
- Panel data exactly matches CLI `--json` outputs.
- Copy actions are deterministic and tested.

### Story 1.3: Built-in “next best step” CTA

Tasks:
- Show deterministic next actions after success:
  - `regress init`
  - `regress run`
  - policy test path
- Provide one-click execution with fallback copy commands.

Repo paths:
- `ui/local/src/components/next-steps/*`
- `core/ui/contracts.go`

Acceptance criteria:
- Next-step actions execute reliably from same workspace.
- Failure states include actionable CLI equivalents.

---

## Epic 2: Core Functional Surfaces (Adoption UX)

Objective: expose the most valuable CLI workflows in a frictionless UI shell.

### Story 2.1: Runpack explorer

Tasks:
- Add viewer for:
  - `manifest.json`
  - `run.json`
  - `intents.jsonl`
  - `results.jsonl`
  - `refs.json`
- Add digest and schema metadata badges.

Repo paths:
- `ui/local/src/components/runpack-explorer/*`
- `core/ui/handlers.go`

Acceptance criteria:
- Explorer renders valid runpack contents and schema IDs.
- Large JSONL files are paged/virtualized to avoid UI freeze.

### Story 2.2: Trace and gate decision viewer

Tasks:
- Add trace viewer with:
  - verdict
  - reason codes
  - intent/policy digests
  - signature status
- Include strict non-allow semantics explanation inline.

Repo paths:
- `ui/local/src/components/trace-viewer/*`
- `core/ui/handlers.go`

Acceptance criteria:
- Viewer clearly distinguishes `allow`, `block`, `require_approval`, `dry_run`.
- Non-allow always shown as non-executable in UI copy.

### Story 2.3: Regress workflow panel

Tasks:
- Add `regress init` and `regress run` panel with:
  - fixture paths
  - grader summary
  - JUnit output path
  - stable exit code display
- Include downloadable `regress_result.json`.

Repo paths:
- `ui/local/src/components/regress/*`
- `core/ui/handlers.go`

Acceptance criteria:
- User can initialize and run regress from UI with deterministic outputs.
- Exit code semantics match CLI contract exactly.

---

## Epic 3: Intent + Receipt Conformance Surface

Objective: make contract trust visible and easy to validate for adopters.

### Story 3.1: Intent + Receipt conformance runner in UI

Tasks:
- Add UI action to run conformance script/command bundle:
  - intent schema checks
  - digest continuity checks
  - receipt/ticket-footer checks
- Render pass/fail matrix with file references.

Repo paths:
- `ui/local/src/components/conformance/*`
- `scripts/test_intent_receipt_conformance.sh`
- `core/ui/handlers.go`

Acceptance criteria:
- Conformance result equals CLI/script outcome for same workspace.
- Failures provide direct links to contract docs.

### Story 3.2: Contract explainer cards

Tasks:
- Add short explainers for:
  - `IntentRequest`
  - `GateResult`
  - `TraceRecord`
  - `Runpack` + receipts
- Link to normative docs.

Repo paths:
- `ui/local/src/components/contracts/*`
- `docs/contracts/primitive_contract.md`
- `docs/contracts/intent_receipt_conformance.md`

Acceptance criteria:
- Users can understand contract meaning without leaving UI.
- No explanatory text contradicts normative docs.

---

## Epic 4: Blessed Integration Lane Accelerator (Coding-Agent + CI)

Objective: convert UI wow into implementation adoption in the highest-fit lane.

### Story 4.1: Wrapper snippet generator (blessed path)

Tasks:
- Add snippet generator for canonical wrapper flow:
  - emit intent
  - evaluate gate
  - fail-closed execution
- Provide Python first, with template parity.

Repo paths:
- `ui/local/src/components/snippets/*`
- `docs/integration_checklist.md`
- `examples/integrations/openai_agents/README.md`

Acceptance criteria:
- Generated snippets compile/run with existing adapter examples.
- Snippet outputs are copy/paste stable and versioned.

### Story 4.2: GitHub Actions regress starter from UI

Tasks:
- Add “Generate CI workflow” panel using:
  - `.github/workflows/adoption-regress-template.yml`
- Support output preview + copy/export.

Repo paths:
- `ui/local/src/components/ci-starter/*`
- `docs/ci_regress_kit.md`

Acceptance criteria:
- Generated workflow stays contract-compatible with template.
- User can apply workflow in < 5 minutes after local success.

### Story 4.3: Skill workflow launcher

Tasks:
- Add launch cards for the three official skills:
  - capture runpack
  - incident to regression
  - policy test rollout
- UI remains wrapper-only (invokes existing scripts/commands).

Repo paths:
- `ui/local/src/components/skills/*`
- `.agents/skills/*/SKILL.md`
- `scripts/install_repo_skills.sh`

Acceptance criteria:
- UI can install/validate official skills via existing scripts.
- No skill logic duplicated in UI code.

---

## Epic 5: UX and Visual System (Marketing + Operator Utility)

Objective: make the UI memorable and trustworthy without becoming decorative noise.

### Story 5.1: Design system and tokens

Tasks:
- Implement theme tokens inspired by `fabra_oss/playground`:
  - dark gradient base
  - high-contrast status tokens
  - command-console typography
- Define semantic color mapping for verdicts and verification states.

Repo paths:
- `ui/local/src/app/globals.css`
- `ui/local/src/components/ui/*`

Acceptance criteria:
- UI presents consistent semantic colors across all workflow panels.
- Contrast meets accessibility requirements.

### Story 5.2: Motion and state transitions

Tasks:
- Add meaningful animations only for:
  - command execution start/complete
  - step progression
  - artifact status transitions
- Avoid excessive micro-animation.

Repo paths:
- `ui/local/src/components/*`

Acceptance criteria:
- Motion clarifies progress and outcomes.
- Reduced-motion preference is respected.

### Story 5.3: Responsive layout quality

Tasks:
- Desktop layout: command center with side panels.
- Mobile layout: stacked wizard-first flow.
- Ensure no horizontal overflow in JSON/command panels.

Repo paths:
- `ui/local/src/app/page.tsx`
- `ui/local/src/components/layout/*`

Acceptance criteria:
- Full first-run flow works on desktop and mobile breakpoints.
- UI remains readable on narrow screens.

---

## Epic 6: Security, Privacy, and Safety Guardrails

Objective: keep localhost UI safe and aligned with Gait defaults.

### Story 6.1: Localhost-only safety defaults

Tasks:
- Enforce local bind defaults and explicit override flags.
- Add CSRF/basic origin checks for local API endpoints.
- Disable remote access by default.

Repo paths:
- `cmd/gait/ui.go`
- `core/ui/server.go`

Acceptance criteria:
- Default startup binds only to loopback.
- Non-loopback requires explicit unsafe flag and warning.

### Story 6.2: Sensitive data handling policy

Tasks:
- Mask sensitive payload fields in UI by default where feasible.
- Ensure no secrets are persisted in browser storage by default.
- Provide explicit "copy raw JSON" warnings where needed.

Repo paths:
- `ui/local/src/lib/redaction.ts`
- `docs/ui_localhost.md`

Acceptance criteria:
- UI does not silently persist sensitive data to disk/localStorage.
- Redaction behavior is documented and test-covered.

### Story 6.3: Fail-closed UX semantics

Tasks:
- Enforce non-allow outcomes as non-executable in all UI flows.
- Display reason codes and remediation commands.
- Prevent UI wording that implies non-allow is success.

Repo paths:
- `ui/local/src/components/verdict/*`
- `docs/ui_localhost.md`

Acceptance criteria:
- No UI path allows execution when verdict is not `allow`.
- Error and block states map directly to CLI contracts.

---

## Epic 7: Packaging, Distribution, and Runtime Ergonomics

Objective: make `gait ui` easy to use with current install paths.

### Story 7.1: Single-command runtime

Tasks:
- Implement:
  - `gait ui`
  - `gait ui --port`
  - `gait ui --open-browser=false`
- Show startup banner with URL and workspace context.

Repo paths:
- `cmd/gait/ui.go`
- `cmd/gait/main.go`

Acceptance criteria:
- Command starts reliably on macOS/Linux/Windows.
- Browser launch failures do not crash server.

### Story 7.2: Release artifact integration

Tasks:
- Include UI assets in release binaries.
- Ensure checksums/SBOM/provenance still pass release gates.

Repo paths:
- `.github/workflows/release.yml`
- `.goreleaser.yaml`

Acceptance criteria:
- Release binaries include working UI assets.
- Existing release integrity pipeline remains green.

### Story 7.3: Homebrew and installer compatibility

Tasks:
- Validate `scripts/install.sh` and Homebrew install paths with UI command.
- Add install smoke for `gait ui --help` and basic startup.

Repo paths:
- `scripts/test_install.sh`
- `scripts/test_release_smoke.sh`
- `docs/homebrew.md`

Acceptance criteria:
- UI command works from all supported distribution channels.

---

## Epic 8: Quality, Testing, and Performance

Objective: keep UI trustworthy, deterministic, and fast.

### Story 8.1: Unit and contract tests

Tasks:
- Add Go tests for UI handlers and command integration.
- Add frontend unit tests for critical components and state transitions.
- Add API contract tests using JSON fixtures.

Repo paths:
- `core/ui/*_test.go`
- `cmd/gait/ui_test.go`
- `ui/local/src/**/*.test.ts(x)`

Acceptance criteria:
- Contract tests fail on API drift.
- Core UI flows have deterministic test coverage.

### Story 8.2: End-to-end smoke tests

Tasks:
- Add e2e tests for:
  - first-run flow
  - regress panel
  - conformance runner
  - CI template export
- Run headless in CI.

Repo paths:
- `ui/local/e2e/*`
- `.github/workflows/ci.yml`

Acceptance criteria:
- E2E suite passes across primary CI OS lane.
- Failures include screenshots/logs artifacts.

### Story 8.3: Runtime performance budgets

Tasks:
- Define startup/render budgets:
  - time-to-interactive
  - command roundtrip latency
  - memory envelope
- Add regression checks in nightly CI.

Repo paths:
- `perf/ui_budgets.json`
- `scripts/check_ui_budgets.py`
- `.github/workflows/ui-nightly.yml`

Acceptance criteria:
- Budget regressions fail nightly lane.
- Budget report is artifacted and reviewable.

---

## Epic 9: Documentation and Go-To-Market Enablement

Objective: make UI usable by adopters and useful in demos without overclaiming.

### Story 9.1: User guide and operator runbook

Tasks:
- Add `docs/ui_localhost.md`:
  - install/run
  - first-run walkthrough
  - troubleshooting
  - security notes
- Add FAQ section for common local setup issues.

Acceptance criteria:
- New user can launch and complete first-run using this doc only.

### Story 9.2: Demo script integration

Tasks:
- Add optional UI branch to `scripts/demo_90s.sh`:
  - starts `gait ui`
  - captures core panels/states
- Keep terminal-only mode as default.

Repo paths:
- `scripts/demo_90s.sh`
- `docs/launch/README.md`

Acceptance criteria:
- Demo script can include UI workflow without breaking existing terminal path.

### Story 9.3: Messaging guardrails

Tasks:
- Ensure all UI copy reinforces:
  - CLI authority
  - deterministic artifacts
  - fail-closed semantics
- Avoid marketing claims not backed by implementation.

Repo paths:
- `ui/local/src/content/*`
- `README.md`
- `docs/launch/*`

Acceptance criteria:
- Copy review checklist passes before release.
- UI messaging remains consistent with contracts and docs.

---

## Epic 10: Release Readiness and Acceptance Checklist

Objective: ship UI as a reliable optional surface without destabilizing core workflows.

Tasks:
- Add `scripts/test_ui_acceptance.sh` with required checks:
  - `gait ui --help`
  - first-run flow via API/e2e
  - artifact panel correctness
  - conformance runner pass
  - CI template export validity
  - dependency freshness report pass
- Wire acceptance script into CI and release checklists.

Repo paths:
- `scripts/test_ui_acceptance.sh`
- `Makefile` (target `test-ui-acceptance`)
- `.github/workflows/ci.yml`

Acceptance criteria:
- UI acceptance suite is green before merge and release.
- Existing non-UI acceptance suites remain unaffected.

---

## Definition of Done (applies to every story)

- Code is formatted and linted (`make fmt`, `make lint` plus UI-specific lint/test).
- UI uses CLI/Go contracts only; no duplicate runtime-governance logic in frontend.
- Dependency freshness checks are implemented and passing.
- `gait ui` works offline with local artifacts and no hosted dependency.
- Desktop and mobile flows are both validated.
- Non-allow verdict behavior is fail-closed and explicit in UI.
- Docs are updated with copy/paste commands and troubleshooting steps.
- Release/install paths continue to pass existing integrity and smoke tests.
