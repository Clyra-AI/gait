# PLAN v2.4: Pack Runtime Wedge Completion (Zero-Ambiguity Execution Plan)

Date: 2026-02-14
Source of truth: `product/PLAN_v2.3.md`, `docs/uat_functional_plan.md`, `docs/contracts/primitive_contract.md`, `docs/test_cadence.md`, memo "OSS Autonomy Harness wedge (Pack Runtime)"
Scope: v2.4 only (complete memo parity for offline-first durable jobs + unified packs while preserving existing v1.x contracts)

This plan is written to be executed top-to-bottom with minimal interpretation. Each story includes concrete repo paths, commands, and acceptance criteria.

---

## Global Decisions (Locked for v2.4)

- v2.4 is a runtime + artifact convergence release, not a hosted-service release.
- Existing contracts remain valid during migration:
  - `runpack_*` artifacts
  - `evidence_pack_*` artifacts
  - `gait run *`, `gait guard *`, `gait incident *` command families
- New unified surfaces are additive first; destructive removals are post-v2.4 and explicitly version-gated.
- `gait` remains authoritative for security primitives:
  - Ed25519 signing and verification
  - fail-closed `oss-prod` policy enforcement
  - approval/delegation token validation
- `wrkr` code is used as implementation reference for runtime operability patterns only (durable store, queue, lease, checkpoint lifecycle), not as security authority.
- All core validation remains offline-capable by default.

---

## Gap Closure Matrix (Memo Parity)

| Gap ID | Memo Gap | Current State | v2.4 Closure |
|---|---|---|---|
| G-01 | Durable long-running execution controls | `run session` foundation exists, but no unified job runtime surface | Add `gait job` command family with durable store, lease, queue, checkpoint, pause/resume/cancel, stop reasons |
| G-02 | Real replay execution not implemented | replay safety interlocks exist, real tools still stubbed | Implement real replay execution path behind explicit unsafe flags and allowlists |
| G-03 | Credential evidence TTL binding | credential evidence exists without explicit TTL fields | Extend broker response + credential evidence schema with issued/expiry fields |
| G-04 | Unified job CLI surface missing | no `job` in top-level dispatch | Add top-level `job` command and subcommands |
| G-05 | Unified pack CLI surface missing | pack behavior split across `run`, `guard`, `incident` | Add top-level `pack` command (`build/verify/inspect/diff`) |
| G-06 | Single PackSpec v1 (job + run) missing | separate runpack and guard schemas | Introduce PackSpec v1 with `pack_type=run|job` and migration rules |
| G-07 | Unified pack diff missing | `run diff` exists; guard has no diff surface | Add `pack diff` deterministic JSON with stable exit semantics |
| G-08 | Runpack signing not exposed in capture CLI | core supports signatures, `run record` has no key flags | Add signing flags to capture paths (`run record`, `pack build` run mode) |
| G-09 | Single install path not achieved | source/release/brew all treated as equal install paths | Lock one primary install path in docs/demo/UAT funnel while preserving compatibility paths |
| G-10 | PackSpec v1 TCK vectors missing | no explicit PackSpec TCK artifact lane | Create PackSpec TCK fixtures, runners, and CI gates |

---

## Borrow Map (Wrkr -> Gait)

The following pieces are approved for borrowing/adaptation.

### Borrow as design/implementation template

- Durable queue/status transition model:
  - `/Users/davidahmann/Projects/wrkr/core/queue/queue.go`
- CAS append + lock model for durable event log:
  - `/Users/davidahmann/Projects/wrkr/core/store/store.go`
- Lease acquisition/heartbeat/release execution wrapper:
  - `/Users/davidahmann/Projects/wrkr/core/dispatch/execution.go`
- Checkpoint guardrails:
  - bounded summary
  - decision-needed requires required_action
  - approval required before resume
  - `/Users/davidahmann/Projects/wrkr/core/runner/runner.go`
- Env fingerprint capture + mismatch block/override semantics:
  - `/Users/davidahmann/Projects/wrkr/core/envfp/envfp.go`
  - `/Users/davidahmann/Projects/wrkr/core/runner/runner.go`
- Jobpack operational surfaces (as UX pattern):
  - `/Users/davidahmann/Projects/wrkr/core/pack/pack.go`
  - `/Users/davidahmann/Projects/wrkr/core/pack/verify.go`
  - `/Users/davidahmann/Projects/wrkr/core/pack/diff.go`

### Do not borrow as authority (gait already stronger)

- Crypto/signing model in wrkr is digest-only:
  - `/Users/davidahmann/Projects/wrkr/core/sign/digest.go`
- Keep gait signing/tokens as source of truth:
  - `/Users/davidahmann/Projects/gait/core/sign/sign.go`
  - `/Users/davidahmann/Projects/gait/core/gate/approval.go`
  - `/Users/davidahmann/Projects/gait/core/gate/trace.go`

### Reference snippet to port (lease wrapper pattern)

```go
// wrkr pattern: acquire lease, heartbeat in goroutine, run, then release
result, runErr := run()
close(stop)
wg.Wait()
_, releaseErr := r.ReleaseLease(jobID, workerID, leaseID)
```

Source: `/Users/davidahmann/Projects/wrkr/core/dispatch/execution.go`

---

## Success Metrics and Release Gates (Locked)

### Product Metrics

- `P1`: Time-to-first-pack (new `gait pack build --type run`) <= 15 minutes from clean checkout.
- `P2`: Time-to-first-durable-job (`gait job submit` -> `gait job status`) <= 15 minutes.
- `P3`: `pack verify` pass rate >= 99% on generated artifacts in CI lanes.
- `P4`: `pack diff` deterministic output hash stable across repeated runs (same inputs) = 100%.
- `P5`: Real replay path executes allowed tools only, with zero execution when unsafe flags are absent.

### Engineering Gates

- `E1`: All existing UAT/local test orchestration remains green (`scripts/test_uat_local.sh`).
- `E2`: Existing chaos suite remains green (`make test-chaos`).
- `E3`: Existing e2e + integration lanes remain green (`make test-e2e`, `go test ./internal/integration -count=1`).
- `E4`: Existing performance budgets remain green (`make bench-check`, `make test-runtime-slo`).
- `E5`: New PackSpec TCK lane is green and deterministic.

### Release Gate

v2.4 is releasable only when all metrics `P1..P5` and engineering gates `E1..E5` are green.

---

## Repository Touch Map (Planned)

```text
product/
  PLAN_2.4.md

cmd/gait/
  main.go                               # add top-level dispatch for job + pack
  job.go                                # new: submit/status/checkpoint/pause/approve/resume/cancel/inspect
  pack.go                               # new: build/verify/inspect/diff
  run_record.go                         # add signing flags
  run.go                                # real replay execution implementation
  verify.go                             # optional convergence hooks to pack verify

core/
  jobruntime/                           # new: queue/store/lease/checkpoint runtime package
    queue.go
    store.go
    lease.go
    runner.go
  packspec/                             # new: unified pack build/verify/diff/inspect implementation
    build.go
    verify.go
    diff.go
    inspect.go
  runpack/                              # migration shims where needed
  guard/                                # migration shims where needed
  credential/
    broker.go                           # response TTL fields
    providers.go                        # issue path with TTL support
  gate/
    credential_evidence.go              # emit issued_at/expires_at/ttl_seconds

schemas/v1/
  pack/
    manifest.schema.json                # new unified PackSpec v1
    job.schema.json                     # job payload in pack
    run.schema.json                     # run payload in pack (link to existing runpack semantics)
    diff.schema.json                    # deterministic diff output schema
  gate/
    broker_credential_record.schema.json # add TTL/expiry fields (additive)

docs/
  contracts/packspec_v1.md              # new
  contracts/packspec_tck.md             # new
  install.md                            # one primary install path
  uat_functional_plan.md                # update acceptance list with v2.4 gates
  flows.md                              # add unified job + pack operational flow

fixtures/
  packspec_tck/v1/                      # new TCK fixtures (valid + invalid + migration vectors)

scripts/
  test_v2_4_acceptance.sh               # new
  test_packspec_tck.sh                  # new
  test_job_runtime_chaos.sh             # new (paired with existing chaos suite)

internal/
  e2e/v24_job_pack_cli_test.go          # new
  integration/job_runtime_test.go       # new
```

---

## Epic 0: PackSpec v1 Contract Freeze and Compatibility Rails

Objective: define one portable pack contract that supports both `job` and `run` evidence models.

### Story 0.1: Freeze PackSpec v1 schema

Tasks:
- Add `schemas/v1/pack/manifest.schema.json` with:
  - `pack_id`
  - `pack_type` (`job|run`)
  - canonical `contents[]` (`path`, `sha256`, `type`)
  - optional signatures
  - deterministic producer metadata
- Add payload schemas:
  - `schemas/v1/pack/job.schema.json`
  - `schemas/v1/pack/run.schema.json`
- Add `schemas/v1/pack/diff.schema.json` for deterministic diff outputs.

Acceptance criteria:
- Schema validation passes for all fixture vectors.
- Schema changes are additive-only and documented.

### Story 0.2: Migration and compatibility policy

Tasks:
- Document migration from:
  - `runpack_<id>.zip`
  - `evidence_pack_<id>.zip`
  to unified `pack_<id>.zip`.
- Keep compatibility readers for legacy artifacts in v2.4.
- Add explicit deprecation timeline in docs (no behavior break in v2.4).

Repo paths:
- `docs/contracts/packspec_v1.md`
- `docs/contracts/primitive_contract.md`
- `docs/flows.md`

Acceptance criteria:
- `gait verify` still works for legacy artifacts.
- `gait pack verify` works for both legacy and PackSpec v1 artifacts.

---

## Epic 1: Unified Job Runtime Surface (`gait job`)

Objective: promote durable job lifecycle controls from implicit internals to first-class CLI contract.

### Story 1.1: Add top-level `job` command dispatch

Tasks:
- Add `job` command in `/Users/davidahmann/Projects/gait/cmd/gait/main.go`.
- Add `cmd/gait/job.go` subcommands:
  - `submit`
  - `status`
  - `checkpoint list|show`
  - `pause`
  - `approve`
  - `resume`
  - `cancel`
  - `inspect`

Acceptance criteria:
- `gait job --help` includes all subcommands with deterministic usage text.
- JSON outputs include `schema_id`/`schema_version` where applicable.

### Story 1.2: Implement durable runtime engine (borrow wrkr patterns)

Tasks:
- Introduce `core/jobruntime` with:
  - status transition map
  - append-only event log + snapshot
  - CAS append with deterministic lock contention behavior
  - lease acquire/heartbeat/release
- Port/adapt approved patterns from:
  - `/Users/davidahmann/Projects/wrkr/core/queue/queue.go`
  - `/Users/davidahmann/Projects/wrkr/core/store/store.go`
  - `/Users/davidahmann/Projects/wrkr/core/dispatch/execution.go`

Acceptance criteria:
- Concurrent status/update operations are deterministic.
- State reconstruction from event log + snapshot is deterministic.

### Story 1.3: Deterministic stop reasons and decision interrupts

Tasks:
- Add checkpoint type taxonomy with explicit reasons:
  - `plan`, `progress`, `decision-needed`, `blocked`, `completed`
- Enforce:
  - summary length bounds
  - `decision-needed` requires `required_action`
  - stable reason codes in status/checkpoint outputs
- Port/adapt checkpoint validation behavior from:
  - `/Users/davidahmann/Projects/wrkr/core/runner/runner.go`

Acceptance criteria:
- Invalid checkpoint payloads fail with stable `invalid_input` category.
- Resume from decision-needed fails until approval is recorded.

### Story 1.4: Pause/resume/cancel deterministic controls

Tasks:
- Wire `pause`, `resume`, and `cancel` into job state machine.
- Add deterministic rejection for invalid transitions.
- Persist stop reasons and expose in `job status`.

Acceptance criteria:
- Transition matrix tests cover all allowed and blocked transitions.
- CLI exit codes are stable for blocked transitions.

### Story 1.5: Environment fingerprint gating for resume

Tasks:
- Capture environment fingerprint at `job submit`.
- On `job resume`, compare current fingerprint:
  - mismatch -> block by default
  - explicit override flag required
- Port/adapt pattern from:
  - `/Users/davidahmann/Projects/wrkr/core/envfp/envfp.go`
  - `/Users/davidahmann/Projects/wrkr/core/runner/runner.go`

Acceptance criteria:
- Resume fails closed on mismatch without override.
- Override path emits deterministic audit event with actor + reason.

---

## Epic 2: Unified Pack Surface (`gait pack`)

Objective: converge artifact operations into one command family while preserving legacy behavior.

### Story 2.1: Add top-level `pack` command

Tasks:
- Add `pack` command in CLI dispatch.
- Add subcommands:
  - `pack build`
  - `pack verify`
  - `pack inspect`
  - `pack diff`
- Preserve existing:
  - `run record/inspect/diff`
  - `guard pack/verify`
  via compatibility wrappers.

Acceptance criteria:
- `pack` commands run fully offline.
- Legacy commands continue to function and map to new core internals.

### Story 2.2: Implement `pack build` for both types

Tasks:
- `pack build --type run --from <run_id|path|session_chain>`
- `pack build --type job --from <job_id|path>`
- Deterministic zip properties:
  - file ordering
  - stable timestamps
  - stable modes
  - fixed compression policy

Acceptance criteria:
- Identical inputs produce byte-identical output zips.
- Manifest hashes are stable across repeated builds.

### Story 2.3: Implement `pack verify` with strict profile

Tasks:
- Verify:
  - manifest digest/hash integrity
  - declared file hash checks
  - undeclared file rejection
  - schema conformance by pack type
  - optional signature requirements (`strict` profile)

Acceptance criteria:
- Offline verify passes on valid fixtures and fails deterministically on tampered fixtures.

### Story 2.4: Implement deterministic `pack diff`

Tasks:
- Compare two packs deterministically.
- Emit stable JSON diff payload conforming to `schemas/v1/pack/diff.schema.json`.
- Return stable exit semantics:
  - `0` no differences
  - `2` differences or verify-style integrity failure

Acceptance criteria:
- Output hash is stable across repeated diff runs for same inputs.
- Diff output is CI-consumable with no unstable fields.

### Story 2.5: Implement `pack inspect`

Tasks:
- Emit normalized timeline/summary for both pack types.
- Include checkpoint/approval lineage for job-type packs.
- Include run intent/result lineage for run-type packs.

Acceptance criteria:
- Inspect output is deterministic and schema-versioned.

---

## Epic 3: Capture Signing and Replay Completion

Objective: close remaining memo-level runtime gaps around signed capture and real replay execution.

### Story 3.1: Expose signing flags in capture paths

Tasks:
- Add signing options to `gait run record`:
  - `--key-mode dev|prod`
  - `--private-key`
  - `--private-key-env`
- Add corresponding fields in JSON output:
  - signature status
  - key id
- Ensure `pack build --type run` can sign manifest in same way.

Target files:
- `/Users/davidahmann/Projects/gait/cmd/gait/run_record.go`
- `/Users/davidahmann/Projects/gait/core/runpack/record.go`

Acceptance criteria:
- Signed runpacks verify with provided public/derived keys.
- Existing unsigned path remains supported unless strict profile is requested.

### Story 3.2: Implement real replay path (unsafe, explicit)

Tasks:
- Implement real tool execution path behind existing interlocks:
  - `--real-tools`
  - `--unsafe-real-tools`
  - `--allow-tools`
  - `GAIT_ALLOW_REAL_REPLAY=1`
- Keep stub replay as default.
- Add deterministic recording of executed-vs-stubbed steps.

Target files:
- `/Users/davidahmann/Projects/gait/cmd/gait/run.go`
- `/Users/davidahmann/Projects/gait/core/runpack/replay.go`

Acceptance criteria:
- Real replay executes only explicitly allowlisted tools.
- Without unsafe interlocks, no real tool executes.

---

## Epic 4: JIT Credential Evidence TTL Binding

Objective: make credential evidence audit-complete with explicit issuance and expiry semantics.

### Story 4.1: Extend broker response model with expiry metadata

Tasks:
- Add optional fields to credential broker response:
  - `issued_at`
  - `expires_at`
  - `ttl_seconds`
- Support in:
  - stub broker
  - env broker
  - command broker JSON parsing

Target files:
- `/Users/davidahmann/Projects/gait/core/credential/broker.go`
- `/Users/davidahmann/Projects/gait/core/credential/providers.go`

Acceptance criteria:
- Existing brokers without TTL fields remain backward-compatible.
- New fields are parsed and propagated when present.

### Story 4.2: Add TTL binding to credential evidence schema and writer

Tasks:
- Extend schema additively:
  - `/Users/davidahmann/Projects/gait/schemas/v1/gate/broker_credential_record.schema.json`
- Update writer:
  - `/Users/davidahmann/Projects/gait/core/gate/credential_evidence.go`
- Thread values through `gate eval` output path:
  - `/Users/davidahmann/Projects/gait/cmd/gait/gate.go`

Acceptance criteria:
- Credential evidence artifact includes explicit issuance/expiry metadata when available.
- Schema validation passes for both legacy and TTL-augmented records.

---

## Epic 5: Install Funnel Convergence (Single Primary Path)

Objective: align with memo "single install path" without breaking existing compatibility channels.

### Story 5.1: Define one primary install path

Tasks:
- Set release installer (`scripts/install.sh`) as primary CTA path.
- Move brew/source to "alternate/advanced paths" in docs.
- Ensure README and quickstarts lead with one path only.

Target files:
- `/Users/davidahmann/Projects/gait/README.md`
- `/Users/davidahmann/Projects/gait/docs/install.md`
- `/Users/davidahmann/Projects/gait/docs/uat_functional_plan.md`

Acceptance criteria:
- First-run docs present exactly one default install path.
- Alternate paths remain documented but not primary.

### Story 5.2: UAT funnel update

Tasks:
- Keep full install-path compatibility checks in UAT script.
- Add explicit "primary path pass/fail" marker in UAT summary.

Acceptance criteria:
- UAT summary makes primary funnel status explicit.

---

## Epic 6: PackSpec v1 TCK Vectors and CI Contract Lane

Objective: create explicit, deterministic TCK artifacts for PackSpec verification and diff behavior.

### Story 6.1: Add fixture vectors

Tasks:
- Add `fixtures/packspec_tck/v1/` with:
  - valid run pack
  - valid job pack
  - tampered hash pack
  - undeclared file pack
  - schema-invalid pack
  - canonical migration vectors from legacy runpack/evidence pack

Acceptance criteria:
- Fixture set is deterministic and content-addressed.

### Story 6.2: Add TCK runner script + CI workflow hooks

Tasks:
- Add script:
  - `scripts/test_packspec_tck.sh`
- Add make target:
  - `make test-packspec-tck`
- Integrate in CI and nightly hardening/perf where appropriate.

Acceptance criteria:
- TCK lane is green in CI and is release-blocking for v2.4.

### Story 6.3: Publish TCK contract docs

Tasks:
- Add `docs/contracts/packspec_tck.md`:
  - fixture format
  - expected commands
  - expected exit codes
  - determinism requirements

Acceptance criteria:
- External contributors can run TCK from clean checkout with one command.

---

## Epic 7: Validation and Testing Matrix (Mandatory)

Objective: make v2.4 merge/release gates explicit and complete.

### 7.1 Required pre-merge local gate

```bash
make lint-fast
make test-fast
make test-e2e
go test ./internal/integration -count=1
make test-adoption
make test-adapter-parity
make test-contracts
make test-hardening-acceptance
make test-chaos
bash scripts/test_session_soak.sh
make test-runtime-slo
make bench-check
```

### 7.2 Required v2.4 functional acceptance gate

Add and require:

```bash
go build -o ./gait ./cmd/gait
bash scripts/test_v2_4_acceptance.sh ./gait
bash scripts/test_packspec_tck.sh ./gait
```

`test_v2_4_acceptance.sh` must cover at minimum:
- `gait job` lifecycle:
  - submit -> checkpoint -> pause -> approve -> resume -> cancel/status checks
- `gait pack` lifecycle:
  - build (run + job) -> verify -> inspect -> diff
- signed capture path:
  - `run record` with key flags
- real replay interlocks and execution behavior:
  - blocked without unsafe flags
  - allowed with explicit unsafe controls
- credential evidence TTL fields:
  - presence/absence compatibility

### 7.3 Existing UAT path must remain green

```bash
bash scripts/test_uat_local.sh
```

No v2.4 change may regress existing UAT suites listed in:
- `/Users/davidahmann/Projects/gait/docs/uat_functional_plan.md`

### 7.4 Existing chaos, e2e, integration, perf lanes are release-blocking

- Chaos:
  - `make test-chaos`
- E2E:
  - `make test-e2e`
- Integration:
  - `go test ./internal/integration -count=1`
- Perf:
  - `make bench-check`
  - `make test-runtime-slo`

---

## Epic 8: Rollout, Migration, and Compatibility Windows

Objective: deliver v2.4 without breaking existing adopters.

### Story 8.1: Dual-surface period (v2.4)

Tasks:
- Keep old commands active:
  - `run record|inspect|diff`
  - `guard pack|verify`
  - `incident pack`
- Add deprecation hints in command help output, not hard failures.

Acceptance criteria:
- Existing scripts continue to pass without modification.

### Story 8.2: Documentation migration map

Tasks:
- Add "old -> new command mapping" table in docs:
  - `run diff` -> `pack diff --type run`
  - `guard verify` -> `pack verify --type run|job`
  - etc.

Acceptance criteria:
- Mapping table is included in release notes and docs landing pages.

### Story 8.3: Post-v2.4 removal criteria (deferred)

Tasks:
- Define measurable removal criteria for legacy surfaces in roadmap, not in v2.4.

Acceptance criteria:
- No legacy command removal occurs in v2.4.

---

## Explicit Non-Goals (v2.4)

- No hosted control plane, dashboard, or SaaS dependency.
- No policy-language rewrite.
- No non-deterministic enforcement classifiers in execution path.
- No major-version breaking schema removals.

---

## Exit Criteria Checklist (Must All Be True)

- `gait job` command family shipped and documented.
- `gait pack` command family shipped and documented.
- PackSpec v1 (`job|run`) schemas frozen and validated.
- Deterministic `pack verify` and `pack diff` shipped with stable exit behavior.
- Capture signing exposed in CLI record/build paths.
- Real replay implemented under explicit unsafe controls.
- Credential evidence TTL binding implemented additively.
- Single primary install path documented and promoted.
- PackSpec TCK vectors + CI lane operational.
- Existing UAT, e2e, integration, chaos, and perf lanes remain green.

