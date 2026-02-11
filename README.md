# Gait — Agent Control Plane

Enforce policy, capture signed proof, and regression-test every AI agent tool call. One CLI, no network required.

Public docs: [https://davidahmann.github.io/gait/](https://davidahmann.github.io/gait/)
Wiki: [https://github.com/davidahmann/gait/wiki](https://github.com/davidahmann/gait/wiki)
Changelog: [CHANGELOG.md](CHANGELOG.md)

## The Problem

AI agents execute tool calls — database writes, API calls, file mutations — with real authority and real consequences. When something goes wrong, there is no artifact trail, no regression test, and no policy gate that can block the next one. Guardrails scan prompts. Gait decides whether the action runs, and signs the proof either way.

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

## Why Gait Exists

**What it does:** Gait sits at the tool-call boundary — the point where an agent exercises real authority. It decides `allow`, `block`, `require_approval`, or `dry_run` before side effects execute, and emits a signed trace for every decision. Most runtime governance tools observe and alert after the fact. Gait decides before the action runs and proves what happened.

**What it is:**

- an execution-boundary guard for production agent tool calls
- a verifiable artifact standard (runpack + trace) for incidents, CI, and audits
- a vendor-neutral CLI that works across frameworks and model providers

**What it is not:**

- not a hosted dashboard
- not prompt-only filtering
- not a replacement for your identity provider, SIEM, or ticketing system

**Works with your existing stack:**

- identity and vault systems (CyberArk, HashiCorp Vault, cloud IAM)
- AI gateway/guardrail scanners for prompt and output inspection
- SIEM and observability (Splunk, Datadog, Elastic)

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
gait policy validate examples/policy/base_low_risk.yaml --json
gait policy fmt examples/policy/base_low_risk.yaml --write --json
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delete.json --json
gait policy simulate --baseline examples/policy/base_low_risk.yaml --policy examples/policy/base_high_risk.yaml --fixtures examples/policy/intents --json
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

Result: `verdict: block`, `reason_codes: ["blocked_prompt_injection"]`, and `matched_rule` for fast rule-debug context.

Gate evaluates structured tool-call intent, not prompt text. If verdict is not `allow`, execution does not run.

## Where Enforcement Lives In Your Code

Gait does not automatically intercept tools. Your runtime must call Gait at the tool-dispatch chokepoint and enforce the verdict before any side effects execute.

Minimal insertion pattern:

```python
def dispatch_tool(tool_call):
    decision = gait_evaluate(tool_call)  # gate eval / mcp proxy / POST /v1/evaluate
    if decision["verdict"] != "allow":
        return {
            "executed": False,
            "verdict": decision["verdict"],
            "reason_codes": decision.get("reason_codes", []),
        }
    result = execute_real_tool(tool_call)
    return {"executed": True, "result": result}
```

If you run `gait mcp serve`, the service still returns a decision only. The caller must enforce non-`allow` outcomes as non-executable.

## Integrate With Your Framework

**OpenClaw** — install the official boundary package in one command:

```bash
bash scripts/install_openclaw_skill.sh
```

**Gas Town** — wire the worker hook to gate eval. Adapter and secure deployment guide:

- `examples/integrations/gastown`
- `docs/launch/secure_deployment_gastown.md`

**Other frameworks** — adapters ship for OpenAI Agents, LangChain, AutoGen, and AutoGPT:

- `examples/integrations/openai_agents`
- `examples/integrations/langchain`
- `examples/integrations/autogen`
- `examples/integrations/autogpt`
- `sdk/python/examples/openai_style_tool_decorator.py`
- `sdk/python/examples/langchain_style_tool_decorator.py`

Full integration walkthrough: `docs/integration_checklist.md`
Reduce repeated flags with a project config: `docs/project_defaults.md`

For long-running MCP interception instead of one-shot calls:

```bash
gait mcp serve --policy examples/policy-test/allow.yaml --listen 127.0.0.1:8787 --trace-dir ./gait-out/mcp-serve/traces
```

Transport endpoints from one service config:

- `POST /v1/evaluate` (JSON response)
- `POST /v1/evaluate/sse` (SSE response)
- `POST /v1/evaluate/stream` (NDJSON response)

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

Generate and validate local signing keys:

```bash
gait keys init --out-dir ./gait-out/keys --prefix prod --json
gait keys verify --private-key ./gait-out/keys/prod_private.key --public-key ./gait-out/keys/prod_public.key --json
```

For staged rollout, simulate mode reports what *would have* happened without enforcing:

```bash
gait gate eval \
  --policy examples/policy/base_medium_risk.yaml \
  --intent examples/policy/intents/intent_write.json \
  --simulate --json
```

References: `docs/policy_authoring.md`, `docs/approval_runbook.md`, `docs/policy_rollout.md`

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

## Why Now

Agent frameworks are shipping tool-use to production faster than security tooling can keep up. OpenClaw's recent RCE advisory and Gas Town's worker-escape disclosure both trace back to unguarded tool-call boundaries — exactly the problem Gait is built to solve. If your agents call tools with real authority, the window between "works in staging" and "incident in production" is closing.

## Commands

```text
gait demo                                          # offline first win
gait verify <run_id|path>                          # offline integrity proof
gait run replay <run_id|path>                      # deterministic stub replay
gait run diff <left> <right>                       # artifact diff
gait run receipt --from <run_id|path>              # regenerate ticket footer
gait run inspect --from <run_id|path>              # readable run timeline (terminal/json)
gait regress bootstrap --from <run_id|path>        # incident to CI test (one command)
gait regress init --from <run_id|path>             # create fixture from runpack
gait regress run [--junit junit.xml]               # run regressions
gait policy validate <policy.yaml>                 # strict syntax+semantic validation
gait policy fmt <policy.yaml> [--write]            # deterministic policy formatting
gait policy test <policy.yaml> <fixture.json>      # test policy offline
gait policy simulate --baseline <p> --policy <p> --fixtures <csv>  # compare candidate policy vs baseline
gait gate eval --policy <p> --intent <i>           # evaluate tool intent
gait keys init|rotate|verify                        # local signing key lifecycle
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

## Ecosystem

- Discovery: `docs/ecosystem/awesome.md`
- Contribute: `docs/ecosystem/contribute.md`
- Machine-readable index: `docs/ecosystem/community_index.json`
- Adapter proposal: `.github/ISSUE_TEMPLATE/adapter.yml`
- Skill proposal: `.github/ISSUE_TEMPLATE/skill.yml`

## Docs

- Start here: `docs/README.md`
- Mental model: `docs/concepts/mental_model.md`
- Architecture: `docs/architecture.md`
- Integration checklist: `docs/integration_checklist.md`
- All docs, contracts, and references: `docs/README.md`

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

## Feedback

Something broken, confusing, or wrong? Open an issue or start a discussion. Gait is pre-1.1 and the fastest way to improve it is honest feedback from people trying to use it.

- Issues: [https://github.com/davidahmann/gait/issues](https://github.com/davidahmann/gait/issues)
- `SECURITY.md` for vulnerability reports
- `CONTRIBUTING.md`
- `CODE_OF_CONDUCT.md`

## OSS and Enterprise

- OSS v1 is the free execution substrate: runpack, regress, gate, doctor, scout, and adapter kits.
- Enterprise is a separate control-plane layer for fleet governance, consuming OSS artifacts.
- Artifact contracts remain stable regardless of enterprise adoption.

Details: `docs/packaging.md`
