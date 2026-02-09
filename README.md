# Gait

Gait is the offline-first CLI that makes production AI agents controllable and debuggable by default.

It gives teams a deterministic workflow from incident to proof:

- capture a verifiable run artifact (`runpack_<run_id>.zip`)
- replay and diff runs deterministically (stub replay by default)
- convert incidents into CI regressions
- enforce policy at the tool-call boundary with signed traces

The durable contract is artifacts, schemas, and exit codes. Not a hosted UI.

## Why Teams Adopt Gait

Production agent incidents usually fail on the same question: "What happened, exactly?"

Gait answers that with portable artifacts instead of screenshots and guesswork:

- platform teams get repeatable incident reproduction and CI guardrails
- security teams get enforceable policy decisions and signed trace records
- compliance teams get verifiable evidence packs and stable outputs
- agent developers get a fast loop from bug report to deterministic regression

## Start Here (Single Install Path)

Use one path for first use: build from source.

Prerequisites:

- Go `1.25.x`
- `git`

```bash
git clone https://github.com/davidahmann/gait.git
cd gait
go build -o ./gait ./cmd/gait
export PATH="$PWD:$PATH"
```

Run the offline first-win loop:

```bash
gait demo
gait verify run_demo
bash scripts/quickstart.sh
```

Sample `gait demo` output:

```text
run_id=run_demo
bundle=./gait-out/runpack_run_demo.zip
ticket_footer=GAIT run_id=run_demo manifest=sha256:<digest> verify="gait verify run_demo"
verify=ok
```

## Paste Into Ticket: Receipt Semantics

The `ticket_footer` line is the shareable contract across incident tickets, PRs, and CI:

- `run_id`: stable handle for this run
- `manifest=sha256:<digest>`: immutable runpack manifest digest
- `verify="gait verify <run_id>"`: one-command integrity check

This is how teams move from "we think the agent did X" to "here is the exact verifiable artifact."

To regenerate a one-line footer from an existing artifact:

```bash
gait run receipt --from <run_id|path>
```

## Incident To Regression (Deterministic CI Path)

Convert a captured run into CI checks:

```bash
gait regress init --from run_demo --json
gait regress run --json --junit ./gait-out/junit.xml
```

What you get:

- `gait.yaml`
- `fixtures/run_demo/runpack.zip`
- `regress_result.json`
- optional `junit.xml` for CI test reporting

Canonical CI path:

- `.github/workflows/adoption-regress-template.yml`
- `docs/ci_regress_kit.md`

Stable exit behavior is CI-friendly:

- `0` success
- `5` regression failed

## Gate High-Risk Tools

Start with deterministic policy fixture tests:

```bash
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_medium_risk.yaml examples/policy/intents/intent_write.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delete.json --json
```

Typical outcomes:

- low risk read -> `allow`
- medium risk write -> `require_approval`
- high risk destructive call -> `block`

Then evaluate real tool intents through Gate:

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_delete.json \
  --profile oss-prod \
  --credential-broker stub \
  --trace-out ./gait-out/trace_delete.json \
  --json
```

For staged rollout, use simulate mode first:

```bash
gait gate eval --policy examples/policy/base_medium_risk.yaml --intent examples/policy/intents/intent_write.json --simulate --json
```

## Why Gate Exists (Enterprise Security Boundary)

In production agent systems, instruction and data collide:

- external or retrieved content can carry tool-like instructions
- prompt-layer filtering alone does not reliably constrain tool execution
- governance must happen at the execution boundary, not in prompt text

Gate evaluates structured tool-call intent, not prompt text. If verdict is not `allow`, execution must not run.

Concrete blocked prompt-injection-style example:

```bash
gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json
```

Expected result contains:

- `verdict: block`
- `reason_codes: ["blocked_prompt_injection"]`
- `violations: ["prompt_injection_egress_attempt"]`

## Integration Model (No Bypass Rule)

The minimum safe integration pattern:

```text
agent runtime
  -> wrapper or sidecar boundary
    -> gait gate eval --policy ... --intent ... --json
      -> allow: execute tool once
      -> block / require_approval / dry_run / error: do not execute
```

Rules:

- expose wrapped tools only, never raw side-effecting executors
- keep policy logic in Go core, not Python wrappers
- fail closed when evaluation or validation fails

Canonical integration paths:

- Python wrapper: `sdk/python/gait/adapter.py`
- Non-Python sidecar: `examples/sidecar/gate_sidecar.py`

Use `docs/integration_checklist.md` for the implementation checklist and conformance checks.

## Core Commands

```text
gait demo
gait verify <run_id|path> [--profile standard|strict]
gait run replay <run_id|path> [--real-tools --unsafe-real-tools --allow-tools ...]  # stub replay only
gait run diff <left> <right>
gait run receipt --from <run_id|path>
gait regress init --from <run_id|path>
gait regress run [--junit junit.xml]
gait policy test <policy.yaml> <intent_fixture.json>
gait gate eval --policy <policy.yaml> --intent <intent.json>
gait approve --intent-digest <sha256> --policy-digest <sha256> --ttl <duration> --scope <csv> --approver <identity> --reason-code <code>
gait trace verify <trace.json>
gait doctor --json
```

All major commands support `--json`. Most support `--explain`.

Security posture tips:

- Use `gait verify --profile strict ...` (or `gait guard verify --profile strict ...`) to require signatures plus explicit verify keys.
- In `oss-prod`, high-risk allow/approval policy rules must require broker credentials and runtime eval must pass `--credential-broker`.

## Contracts You Can Build On

- Canonical primitive contract: `docs/contracts/primitive_contract.md`
- Determinism: verify, diff, and stub replay are deterministic for identical artifacts
- Offline-first: core workflows do not require network access
- Default-safe privacy: reference receipts by default, raw capture is explicit opt-in
- Fail-closed safety: high-risk paths block on policy/approval ambiguity
- Schema stability: versioned artifacts and stable machine-readable outputs
- Stable exit codes: exit code surface is treated as API contract

## v1.1 to v1.5 Progress

- `v1.1`: scout coverage and guard pack foundations
- `v1.2`: deeper enforcement and approval semantics
- `v1.3`: MCP proxy path and adapter boundary support
- `v1.4`: incident/evidence workflows and verification chain
- `v1.5`: installable Gait skills for capture, regression, and rollout workflows

## Docs Ladder

Read in this order:

1. `/Users/davidahmann/Projects/gait/README.md`
2. `docs/contracts/primitive_contract.md`
3. `docs/integration_checklist.md`
4. `docs/policy_rollout.md`
5. `docs/approval_runbook.md`
6. `docs/ci_regress_kit.md`
7. `docs/evidence_templates.md`

## Development

```bash
make fmt
make lint
make test
make test-e2e
make test-adoption
make test-release-smoke
make test-contracts
make test-hardening
make test-hardening-acceptance
make test-live-connectors
```

`make test-live-connectors` is non-gating by default and skips unless `GAIT_ENABLE_LIVE_CONNECTOR_TESTS=1`.

Enable hooks locally:

```bash
pre-commit install --hook-type pre-commit --hook-type pre-push
```

## Project Links

- `SECURITY.md`
- `CONTRIBUTING.md`
- `CODE_OF_CONDUCT.md`
- `docs/hardening/contracts.md`
- `docs/hardening/release_checklist.md`
- `product/PRD.md`
- `product/ROADMAP.md`
