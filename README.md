# Gait

Your AI agent broke prod. Gait gives you the signed artifact to prove what happened, the regression to make sure it never happens again, and the policy gate to block it at the boundary.

Public docs + marketing site: [https://davidahmann.github.io/gait/](https://davidahmann.github.io/gait/)
Operational wiki (playbooks and troubleshooting): [https://github.com/davidahmann/gait/wiki](https://github.com/davidahmann/gait/wiki)
Changelog: [CHANGELOG.md](CHANGELOG.md)

## Install And First Win (60 Seconds)

```bash
curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/install.sh | bash
```

Linux and macOS. Windows: see `docs/install.md`.
Homebrew (tap) alternative:

```bash
brew tap davidahmann/tap
brew install gait
```

Tap-first release details: `docs/homebrew.md`.

```bash
gait demo
```

```text
run_id=run_demo
bundle=./gait-out/runpack_run_demo.zip
ticket_footer=GAIT run_id=run_demo manifest=sha256:88913ed... verify="gait verify run_demo"
verify=ok
```

You now have a signed, portable execution artifact. Verify it:

```bash
gait verify run_demo
```

Paste the `ticket_footer` line into any incident ticket or PR. Anyone with the artifact can verify it offline.

## What Just Happened

- **runpack** = a signed ZIP of exactly what the agent did (intents, results, receipts, manifest with SHA-256 hashes)
- **verify** = offline integrity proof anyone can run, no network needed
- **ticket_footer** = the one line you paste into tickets so incidents are traceable

Full mental model: `docs/concepts/mental_model.md`

## Turn That Into A CI Regression (2 Minutes)

Fast path (one command):

```bash
gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml
```

This incident is now a permanent test. If agent behavior drifts, CI fails.

Equivalent explicit path:

```bash
gait regress init --from run_demo --json
gait regress run --json --junit ./gait-out/junit.xml
```

What you get:

- `gait.yaml` and `fixtures/run_demo/runpack.zip`
- `regress_result.json`
- `junit.xml` for CI test reporting

Exit codes are stable and CI-friendly:

- `0` success
- `5` regression failed

When regress fails, JSON output includes `top_failure_reason`, `next_command`, and `artifact_paths` so you can act without parsing large files.

Canonical CI template: `.github/workflows/adoption-regress-template.yml`

## Block Dangerous Tool Calls (5 Minutes)

Gate examples use fixture files from this repository:

```bash
git clone https://github.com/davidahmann/gait.git && cd gait
```

Test policies deterministically (no side effects, no keys needed):

```bash
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delete.json --json
```

- low risk read -> `allow`
- high risk destructive call -> `block`

Evaluate a real intent through Gate:

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_delete.json \
  --trace-out ./gait-out/trace_delete.json \
  --json
```

Every gate decision produces a signed `trace_<id>.json`. Every block is auditable.

Block a prompt-injection-style tool call:

```bash
gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json
```

Result: `verdict: block`, `reason_codes: ["blocked_prompt_injection"]`

Gate evaluates structured tool-call intent, not prompt text. If verdict is not `allow`, execution does not run.

## Why This Matters

**What Gait is:**

- an execution-boundary guard for production agent tool calls
- a verifiable artifact standard (runpack + trace) for incidents, CI, and audits
- a vendor-neutral CLI that works across frameworks and model providers

**What Gait is not:**

- not a hosted dashboard
- not prompt-only filtering
- not a replacement for your identity provider, SIEM, or ticketing system

**Runtime governance vs ACP:**

- Runtime governance usually observes and alerts.
- Gait ACP decides at execution time (`allow`/`block`/`require_approval`/`dry_run`) and emits signed proof.
- Guardrails scan content. Gait evaluates structured action intent before side effects execute.

Reference: `docs/zero_trust_stack.md`

**Works with your existing stack:**

- identity and vault systems (for example CyberArk, HashiCorp Vault, cloud IAM)
- AI gateway/guardrail scanners for prompt and output inspection
- SIEM and observability systems (for example Splunk, Datadog, Elastic)

Integration references: `docs/zero_trust_stack.md`, `docs/external_tool_registry_policy.md`, `docs/siem_ingestion_recipes.md`

**Why tool-call boundary, not prompt layer:**

Tool calls are where authority is exercised. Portable artifacts are the durable evidence contract. Deterministic regressions turn one incident into a permanent safety test.

**OSS and Enterprise:**

- OSS v1 is the free execution substrate: runpack, regress, gate, doctor, scout, and adapter kits.
- Enterprise is a separate control-plane layer for fleet governance, consuming OSS artifacts.
- Artifact contracts remain stable regardless of enterprise adoption.

Details: `docs/packaging.md`

## Demo To Production (30 to 120 Minutes)

1. Walk through `docs/integration_checklist.md` once.
2. Pick the framework adapter closest to your stack:
   - `examples/integrations/openai_agents`
   - `examples/integrations/langchain`
   - `examples/integrations/autogen`
   - `examples/integrations/openclaw`
   - `examples/integrations/autogpt`
   - `examples/integrations/gastown`
3. If you use OpenClaw, install the official boundary package:
   - `bash scripts/install_openclaw_skill.sh`
4. Wire the boundary: wrapper or sidecar -> `gait gate eval` -> execute only on `allow`.
5. Add a regress fixture and JUnit output in CI before enabling privileged tools.

Reduce repeated flags with a project config: `docs/project_defaults.md`

For long-running MCP interception instead of one-shot calls:

```bash
gait mcp serve --policy examples/policy-test/allow.yaml --listen 127.0.0.1:8787 --trace-dir ./gait-out/mcp-serve/traces
```

## Production Posture (oss-prod Profile)

For fail-closed production enforcement, use `--profile oss-prod` with explicit keys and a credential broker:

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_delete.json \
  --profile oss-prod \
  --key-mode prod \
  --private-key examples/scenarios/keys/approval_private.key \
  --credential-broker stub \
  --trace-out ./gait-out/trace_delete_prod.json \
  --json
```

The demo key in `examples/scenarios/keys/` is for walkthroughs only. In production, provision your own key and replace `stub` with `env` or `command` broker.

For staged rollout, simulate mode reports what *would have* happened without enforcing:

```bash
gait gate eval \
  --policy examples/policy/base_medium_risk.yaml \
  --intent examples/policy/intents/intent_write.json \
  --simulate --json
```

References: `docs/approval_runbook.md`, `docs/policy_rollout.md`, `docs/project_defaults.md`

## Local Signal Engine

Cluster incident families and rank top issues offline:

```bash
gait scout signal --runs ./gait-out/runpack_run_demo.zip --json
```

If you also have regress results or traces, pass them for richer analysis:

```bash
gait scout signal --runs ./gait-out/runpack_run_demo.zip --regress ./gait-out/regress_result.json --traces ./gait-out/trace_delete.json --json
```

Output includes deterministic fingerprints, family grouping, ranked top issues with driver attribution (`policy_change`, `tool_result_shape_change`, `reference_set_change`, `configuration_change`), and bounded fix suggestions.

## Ecosystem

- Discovery: `docs/ecosystem/awesome.md`
- Contribute: `docs/ecosystem/contribute.md`
- Machine-readable index: `docs/ecosystem/community_index.json`
- Adapter proposal: `.github/ISSUE_TEMPLATE/adapter.yml`
- Skill proposal: `.github/ISSUE_TEMPLATE/skill.yml`

## Commands

```text
gait demo                                          # offline first win
gait verify <run_id|path>                          # offline integrity proof
gait run replay <run_id|path>                      # deterministic stub replay
gait run diff <left> <right>                       # artifact diff
gait run receipt --from <run_id|path>              # regenerate ticket footer
gait regress bootstrap --from <run_id|path>        # incident to CI test (one command)
gait regress init --from <run_id|path>             # create fixture from runpack
gait regress run [--junit junit.xml]               # run regressions
gait policy test <policy.yaml> <fixture.json>      # test policy offline
gait gate eval --policy <p> --intent <i>           # evaluate tool intent
gait mcp proxy --policy <p> --call <payload.json>  # one-shot MCP/tool-call boundary
gait mcp serve --policy <p> --listen <addr>        # long-running local interception service
gait approve --intent-digest ... --ttl ...         # mint approval token
gait scout signal --runs <csv>                     # cluster incidents
gait guard pack --run <id> --out <path>            # evidence bundle
gait incident pack --from <id> --window <dur>      # incident bundle
gait trace verify <trace.json>                     # verify signed trace
gait doctor --json                                 # environment diagnostics
```

All commands support `--json`. Most support `--explain`.

## Contracts

- Primitive contract: `docs/contracts/primitive_contract.md`
- Determinism: verify, diff, and stub replay are deterministic for identical artifacts
- Offline-first: core workflows require no network
- Fail-closed: high-risk paths block on policy or approval ambiguity
- Schema stability: versioned artifacts, backward-compatible readers
- Exit codes: `0` success, `2` verification failed, `3` policy block, `4` approval required, `5` regress failed, `6` invalid input
- Runtime SLOs: `docs/slo/runtime_slo.md`

## Docs

Start here:

- `docs/README.md`
- `docs/concepts/mental_model.md`
- `docs/architecture.md`
- `docs/flows.md`

Integration and operations:

- `docs/integration_checklist.md`
- `docs/project_defaults.md`
- `docs/policy_rollout.md`
- `docs/approval_runbook.md`
- `docs/ci_regress_kit.md`

Contracts and compliance:

- `docs/contracts/primitive_contract.md`
- `docs/evidence_templates.md`
- `docs/slo/runtime_slo.md`

Positioning and distribution:

- `docs/positioning.md`
- `docs/packaging.md`
- `docs/install.md`
- `docs/homebrew.md`

Ecosystem and launch:

- `docs/ecosystem/awesome.md`
- `docs/launch/README.md`
- `docs/launch/rfc_openclaw.md`
- `docs/launch/rfc_gastown.md`
- `docs/launch/secure_deployment_openclaw.md`
- `docs/launch/secure_deployment_gastown.md`

External integration:

- `docs/zero_trust_stack.md`
- `docs/external_tool_registry_policy.md`
- `docs/siem_ingestion_recipes.md`

Wiki: [https://github.com/davidahmann/gait/wiki](https://github.com/davidahmann/gait/wiki) (playbook layer synced from `docs/wiki/`)

## Development

```bash
make fmt && make lint && make test
make test-e2e
make test-adoption
make test-contracts
make test-hardening-acceptance
make test-release-smoke
make test-uat-local
make docs-site-install
make docs-site-build
```

90-second terminal demo: `bash scripts/demo_90s.sh`

Enable required pre-push hook: `make hooks`

## Links

- `SECURITY.md`
- `CONTRIBUTING.md`
- `CODE_OF_CONDUCT.md`
- `docs/hardening/contracts.md`
- `docs/hardening/release_checklist.md`
- `docs/uat_functional_plan.md`
- `product/PRD.md`
- `product/ROADMAP.md`
