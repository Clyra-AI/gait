# PLAN v1.9: Transport-Agnostic Interception + Runpack Inspectability

Date: 2026-02-11  
Source of truth: `product/PRD.md`, `product/ROADMAP.md`, `product/PLAN_v1.md`, `product/PLAN_v1.8.md`, current `main` codebase  
Scope: OSS v1.9 execution-layer hardening only (no hosted control plane)

This plan is written to be executed top-to-bottom with minimal interpretation. Every story includes concrete repo paths, commands, and acceptance criteria.

---

## Global Decisions (Locked for v1.9)

- Preserve v1 category boundary: Gait remains an execution-boundary control and evidence CLI, not an agent framework or hosted gateway.
- Keep Go authoritative for all decision logic, canonicalization, signing, and artifact generation.
- Treat transport support as an adapter concern over one canonical policy-evaluation path.
- Keep offline-first and fail-closed semantics unchanged:
  - non-`allow` outcomes never execute side effects
  - policy-evaluation ambiguity in protected lanes degrades to block/error
- Keep deterministic artifact and exit-code contracts stable.
- Keep CI fast-path latency a product concern; move slow Windows lint from per-push CI into nightly hardening lane.

---

## v1.9 Objective

Deliver two high-leverage usability/coverage upgrades and one CI throughput optimization:

1. Make MCP interception transport-aware in a single local service configuration (JSON request/response + SSE + streamable HTTP endpoint shape).
2. Add `gait run inspect` to render a deterministic, human-readable run timeline from runpacks (terminal and `--json`).
3. Move Windows lint from main CI to nightly workflow to reduce merge-path CI duration.

---

## Current Baseline (Observed)

Already in place:

- `gait mcp serve` supports `POST /v1/evaluate` JSON request/response.
- `gait mcp proxy` supports stdin (`--call -`) one-shot evaluation.
- Runpack primitives exist: record, verify, replay, diff, reduce, receipt.
- CI lint job runs on Linux/macOS/Windows and is frequently the slowest lane.
- UAT orchestration exists in `scripts/test_uat_local.sh`.

Current gaps:

- `mcp serve` does not expose transport-aware response modes from one endpoint surface (SSE/streamable HTTP).
- No first-class `run inspect` command for readable run sequencing.
- Windows lint cost is paid on every push/PR.

---

## v1.9 Exit Criteria

v1.9 is complete only when all are true:

- `gait mcp serve` supports:
  - existing JSON API (`POST /v1/evaluate`)
  - SSE response mode (`POST /v1/evaluate/sse`)
  - streamable HTTP mode (`POST /v1/evaluate/stream` using NDJSON line output)
- All serve modes reuse identical evaluation path and yield the same verdict/reason semantics.
- `gait run inspect <run_id|path>` exists with:
  - deterministic terminal output
  - machine-readable `--json` output
  - stable exit behavior on invalid/missing artifacts.
- Documentation is updated across README/integration/flows/deployment/UAT references.
- Main CI no longer runs Windows lint per push/PR; Windows lint runs in nightly workflow.
- Full local validation passes, including UAT.
- CI and docs checks are green after push.
- Release `v1.0.3` is tagged, changelog updated, and release workflow is green.

---

## Epic V19.0: MCP Transport-Agnostic Interception Surface

Objective: keep one policy-evaluation core while supporting practical transport modes from one local service.

### Story V19.0.1: Shared request evaluation helper

Tasks:

- Refactor `cmd/gait/mcp_server.go` to centralize request decode/evaluate/response payload creation in one helper.
- Ensure all endpoint variants call the same helper and emit equivalent payload fields.

Repo paths:

- `cmd/gait/mcp_server.go`
- `cmd/gait/mcp_server_test.go`

Acceptance criteria:

- No behavior drift in existing `/v1/evaluate` tests.
- Response payload parity verified by unit tests.

### Story V19.0.2: SSE endpoint

Tasks:

- Add `POST /v1/evaluate/sse` endpoint.
- Response type: `text/event-stream`.
- Emit deterministic event sequence:
  - one `event: evaluate`
  - `data: <json mcpServeEvaluateResponse>`
  - flush and close.
- Keep identical policy evaluation behavior to JSON endpoint.

Repo paths:

- `cmd/gait/mcp_server.go`
- `cmd/gait/mcp_server_test.go`
- `docs/flows.md`
- `docs/deployment/cloud_runtime_patterns.md`

Acceptance criteria:

- SSE test validates status code, content type, event framing, and JSON payload.
- Verdict/exit_code matches JSON endpoint for same input.

### Story V19.0.3: Streamable HTTP (NDJSON) endpoint

Tasks:

- Add `POST /v1/evaluate/stream` endpoint.
- Response type: `application/x-ndjson`.
- Emit one JSON object line with `mcpServeEvaluateResponse` and close.
- Keep identical evaluation semantics to other endpoints.

Repo paths:

- `cmd/gait/mcp_server.go`
- `cmd/gait/mcp_server_test.go`
- `scripts/test_v1_8_acceptance.sh` (extend transport checks)
- `docs/flows.md`

Acceptance criteria:

- NDJSON endpoint test validates content type and parseable JSONL payload.
- Contract parity with `/v1/evaluate` verified in tests.

---

## Epic V19.1: Runpack Timeline Inspect Command

Objective: make runpacks immediately understandable in terminal without external tooling.

### Story V19.1.1: Add `gait run inspect`

Tasks:

- Add new subcommand `inspect` under `run`.
- Inputs:
  - `gait run inspect <run_id|path>`
  - optional `--from <run_id|path>`
  - `--json`
- Output:
  - terminal: concise linear timeline with intent/result correlation
  - JSON: structured run summary + per-intent entries.
- Correlate intents/results by `intent_id`; include unmatched records explicitly.
- Include verdict/reason details when present in result payload.

Repo paths:

- `cmd/gait/run.go`
- new file: `cmd/gait/run_inspect.go`
- `cmd/gait/main_test.go`
- `cmd/gait/verify.go` (top-level usage output)

Acceptance criteria:

- `gait run inspect run_demo` returns deterministic non-empty timeline.
- `--json` output is stable and parseable.
- Missing/invalid runpack path returns stable invalid-input code.

### Story V19.1.2: Inspect tests and contract fixtures

Tasks:

- Add focused tests for:
  - happy path on demo runpack
  - unmatched result records
  - invalid input path
  - `--help` and usage exposure.

Repo paths:

- `cmd/gait/main_test.go`
- new/updated tests in `cmd/gait/run_inspect_test.go` if needed

Acceptance criteria:

- Tests pass in all CI OS lanes.
- Command help appears in run usage and global usage where applicable.

---

## Epic V19.2: Documentation and Operational Guidance

Objective: ensure transport coverage + inspect flow is discoverable and operationally clear.

### Story V19.2.1: Command and flow docs

Tasks:

- Update README command list with `gait run inspect`.
- Add inspect usage examples and expected output shape.
- Update flows doc with transport endpoint map:
  - JSON `/v1/evaluate`
  - SSE `/v1/evaluate/sse`
  - stream `/v1/evaluate/stream`
- Clarify caller enforcement remains required for all transport modes.

Repo paths:

- `README.md`
- `docs/flows.md`
- `docs/concepts/mental_model.md` (if command list mention is needed)

Acceptance criteria:

- Docs reference all supported serve transport endpoints.
- Inspect command is discoverable from top-level README path.

### Story V19.2.2: Integration and deployment docs

Tasks:

- Update integration checklist to include optional transport mode selection validation.
- Update cloud runtime patterns with endpoint examples for JSON/SSE/stream modes.
- Update UAT doc command-coverage list to include `run inspect`.

Repo paths:

- `docs/integration_checklist.md`
- `docs/deployment/cloud_runtime_patterns.md`
- `docs/uat_functional_plan.md`
- `docs/README.md` (map entries if needed)

Acceptance criteria:

- Documentation map includes new/updated transport guidance.
- UAT runbook reflects v1.9 command surface.

---

## Epic V19.3: CI Throughput Optimization (Windows Lint Nightly)

Objective: reduce per-push CI latency without dropping Windows lint coverage.

### Story V19.3.1: Remove Windows from main lint matrix

Tasks:

- Update `.github/workflows/ci.yml` lint job matrix:
  - from `[ubuntu-latest, macos-latest, windows-latest]`
  - to `[ubuntu-latest, macos-latest]`.
- Keep test/e2e Windows coverage unchanged in main CI unless explicitly revised.

Repo paths:

- `.github/workflows/ci.yml`

Acceptance criteria:

- Main CI no longer runs Windows lint job.
- Other required jobs still run on pushes/PRs.

### Story V19.3.2: Add nightly Windows lint workflow

Tasks:

- Add or update nightly workflow to run Windows lint checks:
  - schedule daily (UTC)
  - support manual `workflow_dispatch`
  - run equivalent lint command surface (`make lint` or matching steps).
- Ensure logs/artifacts are retained for debugging.

Repo paths:

- new file: `.github/workflows/windows-lint-nightly.yml` (or update existing nightly workflow)
- `docs/test_cadence.md` (if cadence doc should mention it)

Acceptance criteria:

- Nightly workflow validates Windows lint path.
- Main CI runtime is reduced by removal of Windows lint lane.

---

## Epic V19.4: Validation, UAT, Release v1.0.3

Objective: ship safely with full contract validation and green release pipeline.

### Story V19.4.1: Local validation gates

Tasks:

- Run:
  - `make lint`
  - `make test`
  - `make test-e2e`
  - `make test-adoption`
  - `make test-contracts`
  - `make test-hardening-acceptance`
  - `make test-runtime-slo`
- Run full UAT:
  - `bash scripts/test_uat_local.sh --skip-brew` (or full brew path if environment supports)

Acceptance criteria:

- All local gates pass with no weakened checks.
- UAT summary reports no `FAIL`.

### Story V19.4.2: Push and CI monitoring

Tasks:

- Commit and push to `main`.
- Monitor GitHub Actions for:
  - `ci`
  - `docs` (if docs workflow triggered)
- Fix and repush until all required runs are green.

Acceptance criteria:

- All required push workflows are green for the final commit.

### Story V19.4.3: Post-green UAT rerun + release cut

Tasks:

- Rerun UAT once CI is green.
- Update `CHANGELOG.md` with `## [1.0.3] - 2026-02-11` including:
  - Added: `run inspect`
  - Changed: MCP transport-aware serve endpoints
  - Changed: Windows lint moved to nightly
- Commit changelog/release prep if needed.
- Create and push annotated tag `v1.0.3`.
- Monitor `release` workflow to green.

Acceptance criteria:

- `v1.0.3` tag exists on remote.
- Release workflow succeeds and publishes expected artifacts.

---

## Execution Order (Strict)

1. Create and commit planning artifact (`product/PLAN_v1.9.md`) within implementation commit scope.
2. Implement MCP serve transport endpoints and tests.
3. Implement `run inspect` command and tests.
4. Update docs and UAT references.
5. Apply CI workflow optimization (Windows lint nightly split).
6. Run full local validation + UAT.
7. Commit/push code changes.
8. Monitor/fix CI to green.
9. Rerun UAT.
10. Update changelog, tag `v1.0.3`, push tag, monitor release CI.

---

## Commands Reference (v1.9 Workstream)

Build and spot-check:

```bash
go build -o ./gait ./cmd/gait
./gait run inspect run_demo --json
```

MCP serve transport checks:

```bash
./gait mcp serve --policy examples/policy-test/allow.yaml --listen 127.0.0.1:8788 --trace-dir ./gait-out/mcp-serve/traces
curl -sS -H "content-type: application/json" --data-binary @request.json http://127.0.0.1:8788/v1/evaluate
curl -sS -N -H "content-type: application/json" --data-binary @request.json http://127.0.0.1:8788/v1/evaluate/sse
curl -sS -N -H "content-type: application/json" --data-binary @request.json http://127.0.0.1:8788/v1/evaluate/stream
```

Full validation:

```bash
make lint
make test
make test-e2e
make test-adoption
make test-contracts
make test-hardening-acceptance
make test-runtime-slo
bash scripts/test_uat_local.sh --skip-brew
```

Release:

```bash
git tag -a v1.0.3 -m "v1.0.3"
git push origin main
git push origin v1.0.3
```
