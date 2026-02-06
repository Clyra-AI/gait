# Gait

Gait is an offline-first CLI that makes production AI agent runs **controllable and debuggable by default**.

When an agent touches real tools, credentials, production data, or money, "logs" are not enough. Gait turns each run into a **verifiable artifact** you can paste into an incident ticket, diff deterministically, and convert into a CI regression.

- **Runpack**: produce and verify an integrity-checked `runpack_<run_id>.zip` (demo today; adapters capture real runs)
- **Regress**: turn runpacks into deterministic CI tests (with stable exit codes and optional JUnit)
- **Gate**: enforce policy on *structured tool-call intent* (not prompts), support rollout simulation and approval chains, and emit signed trace records
- **Doctor**: first-5-minutes diagnostics with stable `--json`

The durable product contract is **artifacts, schemas, and exit codes**, not a hosted UI.

## Start Here (5 minutes, offline)

Install by downloading a release binary:

- https://github.com/davidahmann/gait/releases

Run the offline demo and verify the artifact:

```bash
gait demo
gait verify run_demo
```

Expected `gait demo` output:

```text
run_id=run_demo
bundle=./gait-out/runpack_run_demo.zip
ticket_footer=GAIT run_id=run_demo manifest=sha256:<digest> verify="gait verify run_demo"
verify=ok
```

Replay deterministically (stubs by default):

```bash
gait run replay run_demo --json
```

Real replay guardrails require explicit unsafe flags, allowlisted tools, and an environment interlock:

```bash
GAIT_ALLOW_REAL_REPLAY=1 gait run replay run_demo --real-tools --unsafe-real-tools --allow-tools tool.write --json
```

CLI note: flags can be placed before or after positional arguments (for example `gait verify run_demo --json` and `gait verify --json run_demo` are both supported).

## Onboarding Troubleshooting (Top 10)

Run this first when onboarding fails:

```bash
gait doctor --json
```

Use these deterministic fixes for common failures:

1. `gait: command not found`
   `go build -o ./gait ./cmd/gait && export PATH="$PWD:$PATH"`
2. `scripts/quickstart.sh: Permission denied`
   `chmod +x scripts/quickstart.sh`
3. `doctor` reports missing schema files
   `git restore --source=HEAD -- schemas`
4. `verify error: open ./gait-out/runpack_run_demo.zip: no such file`
   `gait demo`
5. `gate eval` missing policy or intent input
   `gait policy test examples/policy-test/allow.yaml examples/policy-test/intent.json --json`
6. `regress init` missing source runpack
   `gait regress init --from run_demo --json`
7. `regress run` fails because fixtures/config are missing
   `gait regress init --from run_demo --json`
8. `output directory not writable`
   `mkdir -p ./gait-out && chmod u+rwx ./gait-out`
9. pre-push hook fails on checks
   `make lint && make test`
10. integration examples missing from local checkout
    `git restore --source=HEAD -- scripts/quickstart.sh examples/integrations`

For adoption milestone visibility from local logs:

```bash
gait doctor adoption --from ./gait-out/adoption.jsonl --json
```

## The Receipt You Paste Into Tickets

The `ticket_footer` line is a copy/paste receipt designed for incident threads, PRs, and CI logs:

- `run_id`: the stable handle for this artifact set
- `manifest=sha256:<digest>`: immutable digest of the runpack manifest
- `verify="gait verify <run_id>"`: one-command integrity check

This is the core workflow: convert "the agent did something" into **a reproducible artifact**.

## Why Gait Exists

Teams shipping agents to production eventually hit the same hair-on-fire questions:

- What did the agent reference or retrieve?
- What tool calls did it actually make?
- What changed between two runs?
- Was that action allowed under policy?
- Can we prove it to security, audit, or incident review?

As agents scale, humans stop reading every generated plan, prompt, or code path. Trust has to scale through **deterministic artifacts** and **enforced boundaries**.

## How It Works (v1)

Gait is built around a small number of strict contracts:

1. **Offline-first**: core workflows do not require network access.
2. **Determinism**: `verify`, `diff`, and stub replay are deterministic for the same artifacts.
3. **Default privacy**: record reference receipts by default, not raw sensitive payloads.
4. **Fail-closed safety**: in high-risk modes, inability to evaluate policy blocks execution.
5. **Schema stability**: artifacts and `--json` outputs are versioned and backward-compatible within a major.

### What You Get (Artifacts)

Gait emits and consumes a few canonical artifacts:

- **Runpack**: `runpack_<run_id>.zip`
  - `manifest.json` (hashes, schema ids/versions, capture mode, optional signatures)
  - `run.json` (run metadata)
  - `intents.jsonl` (normalized tool-call intents)
  - `results.jsonl` (normalized tool-call results, with redaction rules applied)
  - `refs.json` (reference receipts by default)
- **Regress**: `regress_result.json` (optional `junit.xml`)
- **Gate trace**: `trace_<trace_id>.json` (signed trace record per decision)
- **Scout snapshot**: `inventory_snapshot_<snapshot_id>.json` and optional `inventory_coverage_<snapshot_id>.json`
- **Guard evidence pack**: `evidence_pack_<pack_id>.zip` with `pack_manifest.json`
- **Run reducer report**: `<reduced_runpack>.reduce_report.json`
- **Registry metadata cache**: `registry_pack.json` plus local pin files under `~/.gait/registry/pins/`

Schemas live under `schemas/v1/`. Go types are authoritative under `core/schema/v1/`.

Coverage and pack foundations:

- `gait scout snapshot` discovers inventory from workspace files and computes policy coverage.
- `gait scout diff` reports deterministic inventory drift.
- `gait guard pack` builds evidence packs; `gait guard verify` verifies pack integrity offline.
- `gait registry install` installs signed manifests with allowlisted remote hosts and pinning.
- `gait registry list` enumerates locally cached/pinned policy packs.
- `gait registry verify` verifies cached registry metadata signatures and pin digests offline.
- `gait run reduce` emits minimized runpacks that still trigger selected failure predicates.

## Core Workflows

### 0) Record A Runpack From Normalized Input

Capture a run artifact from structured run data:

```bash
gait run record <run_record.json> --json
```

Migrate runpack artifacts to the current schema generation:

```bash
gait migrate <run_id_or_runpack_path> --json
```

### 1) Incident To CI Regression

Convert a run artifact into a deterministic regression test:

```bash
gait demo
gait regress init --from run_demo --json
gait regress run --json
```

What happens:

- `gait regress init` writes `gait.yaml` and a fixture under `fixtures/`
- `gait regress run` runs deterministic graders and emits `regress_result.json`
- Use `--junit=./junit.xml` to produce CI-friendly test reports

### 2) Deterministic Diff (With Privacy Modes)

Diff two runpacks deterministically:

```bash
gait run diff <left_run_id_or_path> <right_run_id_or_path> --privacy=metadata --json
```

`--privacy=metadata` avoids payload diffs and focuses on stable structural drift.

### 2.5) Minimize A Failing Runpack

Reduce to the smallest artifact that still triggers a chosen failure predicate:

```bash
gait run reduce --from <run_id_or_path> --predicate missing_result --json
```

### 3) Gate High-Risk Tools (Policy At The Tool Boundary)

Gate evaluates **structured tool-call intent** (tool name, args, declared targets/destinations, data classes, provenance, context). Prompts and retrieved content are not policy inputs.

Policy test flow (offline):

```bash
gait policy test examples/policy-test/allow.yaml examples/policy-test/intent.json --json
gait policy test examples/policy-test/block.yaml examples/policy-test/intent.json --json
gait policy test examples/policy-test/require_approval.yaml examples/policy-test/intent.json --json
```

Exit codes:

- `0`: allow
- `3`: block
- `4`: require approval

Non-enforcing rollout simulation:

```bash
gait gate eval --policy examples/policy-test/block.yaml --intent examples/policy-test/intent.json --simulate --json
```

Example v1.2 policy controls:

```yaml
default_verdict: allow
rules:
  - name: sensitive-egress
    effect: require_approval
    min_approvals: 2
    require_broker_credential: true
    broker_reference: egress
    broker_scopes: [export]
    rate_limit:
      requests: 10
      window: hour
      scope: tool_identity
    dataflow:
      enabled: true
      tainted_sources: [external, tool_output]
      destination_kinds: [host]
      destination_operations: [write]
      action: require_approval
      reason_code: dataflow_tainted_egress
      violation: tainted_egress
    match:
      tool_names: [tool.write]
```

#### Why Gate Exists (Enterprise Context)

In agent systems, **instructions** and **data** collide:

- External content can smuggle tool-like instructions into the agent context.
- If enforcement is not at the execution boundary, privileged tools can run from untrusted input.

Gate blocks this by making tool execution depend on deterministic evaluation over typed intent.

Concrete blocked example:

```bash
gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json
```

Expected result: verdict `block` with reason code `blocked_prompt_injection`.

### 4) Approvals And Signed Traces

When policy requires approval, mint a scoped approval token:

```bash
gait approve --intent-digest <sha256> --policy-digest <sha256> --ttl 1h --scope tool.write --approver you@company --reason-code change_ticket_123 --json
```

For multi-party approval flows, pass additional tokens with `--approval-token-chain` and inspect `approval_audit_<trace_id>.json`:

```bash
gait gate eval --policy <policy.yaml> --intent <intent.json> --approval-token token_a.json --approval-token-chain token_b.json --json
```

Policies can also require broker-issued credentials before allowing selected tools:

```bash
gait gate eval --policy <policy.yaml> --intent <intent.json> --credential-broker env --credential-env-prefix GAIT_BROKER_TOKEN_ --json
```

For a real integration path, use a command broker:

```bash
gait gate eval --policy <policy.yaml> --intent <intent.json> --credential-broker command --credential-command ./broker --credential-command-args issue,token --json
```

Every gate decision can produce a trace record. Verify trace integrity offline:

```bash
gait trace verify ./trace_<trace_id>.json --json --public-key ./public.key
```

When Gate emits `approval_audit_*.json` and `credential_evidence_*.json`, `gait guard pack --run <run_id_or_path>` auto-discovers and includes them in the evidence pack manifest.

### 4.5) MCP Proxy Mode (Protocol-Adjacent Enforcement)

Proxy or bridge an MCP or adapter-formatted tool call through Gate without changing gate trace verification flow:

```bash
gait mcp proxy --policy <policy.yaml> --call <tool_call.json> --adapter mcp --trace-out trace_mcp.json --json
gait mcp bridge --policy <policy.yaml> --call <tool_call.json> --adapter mcp --trace-out trace_mcp.json --json
```

Supported adapter payload formats:

- `mcp`
- `openai`
- `anthropic`
- `langchain`

Optional proxy artifacts:

- `--runpack-out <path>` writes a replayable runpack for the proxied decision
- `--export-log-out <path>` writes JSONL events
- `--export-otel-out <path>` writes OTEL-style JSONL events

Adapter lifecycle is composable by default: for each supported adapter payload, use `gait mcp proxy --runpack-out ...` (or `gait mcp bridge --runpack-out ...`) for capture and `gait regress init --from <runpack.zip>` to create deterministic regress fixtures.

### 4.6) Guard v1.4 Audit And Incident Workflows

Generate an audit-oriented pack with a template and optional summary PDF:

```bash
gait guard pack --run <run_id|path> --template soc2 --render-pdf --case-id INC-2026-42 --json
```

Build a one-command incident pack around a run and time window:

```bash
gait incident pack --from <run_id|path> --window 24h --template incident_response --json
```

Apply deterministic retention policies for trace and pack artifacts:

```bash
gait guard retain --root ./gait-out --trace-ttl 168h --pack-ttl 720h --dry-run --report-out retention_report.json --json
```

Encrypt and decrypt local artifacts with key hooks:

```bash
gait guard encrypt --in ./gait-out/evidence_pack_<id>.zip --key-env GAIT_GUARD_KEY --json
gait guard decrypt --in ./gait-out/evidence_pack_<id>.zip.gaitenc --key-env GAIT_GUARD_KEY --json
```

### 4.7) Gait Skills (v1.5)

Gait ships installable skills under `.agents/skills/` for Codex and Claude Code using shared `SKILL.md` content.

Install repo skills into both local providers:

```bash
bash scripts/install_repo_skills.sh
```

Install provider-specific:

```bash
bash scripts/install_repo_skills.sh --provider codex
bash scripts/install_repo_skills.sh --provider claude
```

Included skills:

- `gait-capture-runpack`
- `gait-incident-to-regression`
- `gait-policy-test-rollout`

Design constraints for all shipped skills:

- Skills call `gait` commands and parse `--json`.
- Skills do not embed credentials.
- Skills default to deterministic and safe modes.
- Shared skill frontmatter is Claude-compatible; Codex-specific UI metadata lives in `agents/openai.yaml`.

### 5) Explain Command Intent

Every major command supports `--explain` for short, stable intent text:

```bash
gait run record --explain
gait scout snapshot --explain
gait guard pack --explain
gait registry install --explain
gait registry list --explain
gait registry verify --explain
gait mcp proxy --explain
gait mcp bridge --explain
gait migrate --explain
gait gate eval --explain
```

## Security, Privacy, And Integrity

- **Default-safe recording**: runpacks store reference receipts by default (no raw sensitive content unless explicitly enabled).
- **Integrity checks**: `gait verify` checks file hashes deterministically.
- **Signatures**: runpacks may include signatures; gate trace records are signed. Verification requires a configured public key.
- **Release integrity**: release signing/SBOM/provenance is separate from runpack and trace signing.

## Stable Exit Codes (API Surface)

Exit codes are part of the contract:

- `0`: success
- `1`: generic failure
- `2`: verification failed
- `3`: policy block
- `4`: approval required
- `5`: regress failed
- `6`: invalid input/schema
- `7`: doctor indicates non-fixable missing dependency
- `8`: unsafe operation attempted without explicit flag

## Examples (Offline-Safe)

See `examples/` for exact commands and expected outcomes:

- `examples/stub-replay/`
- `examples/policy-test/`
- `examples/regress-run/`
- `examples/prompt-injection/`
- `examples/python/` (thin adapter demo calling local `gait`)
- `examples/integrations/openai_agents/` (wrapped tool path with allow/block and trace artifacts)
- `examples/integrations/langchain/` (wrapped tool path with allow/block and trace artifacts)
- `examples/integrations/autogen/` (wrapped tool path with allow/block and trace artifacts)

## Python SDK (Thin Adoption Layer)

Python is an adoption layer only: serialization and subprocess boundary, no policy logic.

- Package: `sdk/python/gait/`
- Tests: `sdk/python/tests/`
- Canonical example: `examples/python/reference_adapter_demo.py`
- Integration self-audit: `docs/integration_checklist.md`

Minimum integration contract:

- Expose only wrapped tools to the agent runtime.
- Call `ToolAdapter.execute(...)` for every side-effecting tool invocation.
- Fail closed: do not execute unless gate verdict is explicitly `allow`.
- Keep credentials out of model context; pass approvals and key material through process/env boundaries only.

## Development

Local commands:

```bash
make fmt
make lint
make test
make test-e2e
make test-acceptance
```

Enable hooks:

```bash
pre-commit install --hook-type pre-commit --hook-type pre-push
```

## Project Links

- Security policy: `SECURITY.md`
- Contributing: `CONTRIBUTING.md`
- Code of conduct: `CODE_OF_CONDUCT.md`
- Integration checklist: `docs/integration_checklist.md`
- Product plan: `product/PLAN_v1.md`
- Product requirements: `product/PRD.md`
- Roadmap: `product/ROADMAP.md`
