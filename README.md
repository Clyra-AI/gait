# Gait — Signed Proof and Fail-Closed Control for Production AI Agent Tool Calls

## Overview

Use Gait when an AI agent can cause real side effects and you need deterministic control plus portable proof.

Gait is not an agent framework, not a model host, and not a dashboard. It is an offline-first Go CLI that sits at the tool boundary.

Capture every prod agent tool call as a signed, offline-verifiable pack. Enforce fail-closed policy before high-risk actions execute. Turn incidents into CI regressions in one command.

Docs: [clyra-ai.github.io/gait](https://clyra-ai.github.io/gait/) | Install: [`docs/install.md`](docs/install.md) | Homebrew: [`docs/homebrew.md`](docs/homebrew.md)

Managed/preloaded agent note: managed agents can use Gait at the tool boundary, but Gait does not host the model or replace your agent runtime.

## When To Use Gait

- Tool-calling AI agents need enforceable allow/block/approval decisions.
- You need signed, portable evidence artifacts for PRs, incidents, or audits.
- You want offline, deterministic regressions that fail CI with stable exit behavior.
- You run multi-step jobs and need checkpoints, pause/resume/cancel, and inspectable state.

## When Not To Use Gait

- No local Gait CLI or Gait artifacts are available in the execution path.
- Your workflow only needs prompt orchestration without tool-side effects or evidence contracts.
- You only need hosted observability dashboards and do not need offline verification or deterministic replay.

## Try It (Offline, <60s)

### Fast 20-Second Proof

```bash
# Install (checksums at docs/install.md)
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/install.sh | bash

# Create a signed pack from a synthetic agent run
gait demo

# Prove it's intact
gait verify run_demo

# Turn it into a CI regression gate — one command
gait regress bootstrap --from run_demo --junit ./gait-out/junit.xml
```

No account. No API key. No internet. You now have a verified artifact and a permanent regression test.

## Dev vs Prod

Development quickstart:

- `gait demo`
- `gait verify run_demo`

Production hardening baseline:

```bash
mkdir -p .gait
gait policy init baseline-highrisk --out .gait/policy.yaml
cat > .gait/config.yaml <<'YAML'
gate:
  policy: .gait/policy.yaml
  profile: oss-prod
  key_mode: prod
  private_key_env: GAIT_PRIVATE_KEY
  credential_broker: env
  credential_env_prefix: GAIT_BROKER_TOKEN_
  rate_limit_state: ./gait-out/gate_rate_limits.json

mcp_serve:
  enabled: true
  listen: 127.0.0.1:8787
  auth_mode: token
  auth_token_env: GAIT_MCP_TOKEN
  max_request_bytes: 1048576
  http_verdict_status: strict
  allow_client_artifact_paths: false

retention:
  trace_ttl: 168h
  session_ttl: 336h
  export_ttl: 168h
YAML
gait doctor --production-readiness --json
```

Use production mode when gating real side effects in shared or customer-facing environments.

Local UI playground: [`docs/ui_localhost.md`](docs/ui_localhost.md) | Launch with `gait ui`

## See It

### Simple End-To-End Scenario

![Gait simple end-to-end tool-boundary scenario](docs/assets/gait_demo_simple_e2e_60s.gif)

Video: [`gait_demo_simple_e2e_60s.mp4`](docs/assets/gait_demo_simple_e2e_60s.mp4) | Scenario walkthrough: [`docs/scenarios/simple_agent_tool_boundary.md`](docs/scenarios/simple_agent_tool_boundary.md) | Output legend: [`docs/demo_output_legend.md`](docs/demo_output_legend.md)

See: [2,880 tool calls gate-checked in 24 hours](docs/blog/openclaw_24h_boundary_enforcement.md)

## What You Get

**Signed packs** — every run and job emits a tamper-evident artifact (Ed25519 + SHA-256 manifest). Verify offline. Attach to PRs, incidents, audits. One artifact is the entire proof. Export OTEL-style JSONL and deterministic PostgreSQL index SQL with `gait pack export`.

**Fail-closed policy enforcement** — `gait gate eval` evaluates a structured tool-call intent against YAML policy before the side effect runs. Non-allow means non-execute. Signed trace proves the decision.

**Incident → CI gate in one command** — `gait regress bootstrap` converts a bad run into a permanent regression fixture with JUnit output. Exit 0 = pass, exit 5 = drift. Never debug the same failure twice.

**Durable jobs** — dispatch long-running agent work that survives failures. Checkpoints, pause/resume/cancel, approval gates, deterministic stop reasons. No more lost state at step 47.

**Deterministic replay and diff** — replay an agent run using recorded results as stubs (no real API calls). Diff two packs to see what changed, including context drift classification.

**Voice agent gating** — gate high-stakes spoken commitments (refunds, quotes, eligibility) before they're uttered. Signed `SayToken` capability + callpack artifacts for voice boundaries.

**Risk ranking** — rank highest-risk actions across runs and traces by tool class and blast radius. Offline, no dashboard.

## Integrations

```python
def dispatch_tool(tool_call):
    decision = gait_evaluate(tool_call)
    if decision["verdict"] != "allow":
        return {"executed": False, "verdict": decision["verdict"]}
    return {"executed": True, "result": execute_real_tool(tool_call)}
```

Gait enforces at the tool boundary, not the prompt boundary. Your dispatcher calls Gait; non-`allow` means non-execute.

Blessed lane: [`examples/integrations/openai_agents/`](examples/integrations/openai_agents/)

Quickstart script: `examples/integrations/openai_agents/quickstart.py`

Additional adapters: [LangChain](examples/integrations/langchain/) · [AutoGen](examples/integrations/autogen/) · [AutoGPT](examples/integrations/autogpt/) · [OpenClaw](examples/integrations/openclaw/) · [Gastown](examples/integrations/gastown/) · [Voice](examples/integrations/voice_reference/)

MCP-native: `gait mcp proxy` (one-shot) | `gait mcp serve` (long-running). Details: [`docs/mcp_capability_matrix.md`](docs/mcp_capability_matrix.md)

Integration boundary guide: [`docs/agent_integration_boundary.md`](docs/agent_integration_boundary.md) | Checklist: [`docs/integration_checklist.md`](docs/integration_checklist.md) | Python SDK: [`docs/sdk/python.md`](docs/sdk/python.md)

## CI Adoption (One PR)

```bash
gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml
```

- exit `0` = pass, exit `5` = regression failed
- Template: [`.github/workflows/adoption-regress-template.yml`](.github/workflows/adoption-regress-template.yml)
- Drop-in action: [`.github/actions/gait-regress/README.md`](.github/actions/gait-regress/README.md)
- GitLab/Jenkins/Circle: [`docs/ci_regress_kit.md`](docs/ci_regress_kit.md)
- Canonical copy-paste guide: [`docs/adopt_in_one_pr.md`](docs/adopt_in_one_pr.md)
- Threat model: [`docs/threat_model.md`](docs/threat_model.md)
- Failure taxonomy and exits: [`docs/failure_taxonomy_exit_codes.md`](docs/failure_taxonomy_exit_codes.md)

## Contract Commitments

- **determinism**: verify, diff, and stub replay produce identical results on identical artifacts
- **offline-first**: core workflows do not require network
- **fail-closed**: high-risk paths block on policy or approval ambiguity
- **schema stability**: versioned artifacts with backward-compatible readers
- **stable exit codes**: `0` success · `1` internal/runtime failure · `2` verification failure · `3` policy block · `4` approval required · `5` regress failed · `6` invalid input · `7` dependency missing · `8` unsafe operation blocked

Normative spec: [`docs/contracts/primitive_contract.md`](docs/contracts/primitive_contract.md) | PackSpec v1: [`docs/contracts/packspec_v1.md`](docs/contracts/packspec_v1.md) | Intent+receipt: [`docs/contracts/intent_receipt_conformance.md`](docs/contracts/intent_receipt_conformance.md)

Hardening: [`docs/hardening/v2_2_contract.md`](docs/hardening/v2_2_contract.md) | Runbook: [`docs/hardening/prime_time_runbook.md`](docs/hardening/prime_time_runbook.md)

## Documentation

1. [`docs/README.md`](docs/README.md) — ownership map
2. [`docs/concepts/mental_model.md`](docs/concepts/mental_model.md) — how Gait works
3. [`docs/architecture.md`](docs/architecture.md) — component boundaries
4. [`docs/flows.md`](docs/flows.md) — end-to-end sequences
5. [`docs/durable_jobs.md`](docs/durable_jobs.md) — durable job lifecycle and differentiation
6. [`docs/contracts/primitive_contract.md`](docs/contracts/primitive_contract.md) — normative spec

Public docs: [clyra-ai.github.io/gait](https://clyra-ai.github.io/gait/) | Wiki: [github.com/Clyra-AI/gait/wiki](https://github.com/Clyra-AI/gait/wiki) | Changelog: [CHANGELOG.md](CHANGELOG.md)

## Developer Workflow

![CI](https://github.com/Clyra-AI/gait/actions/workflows/ci.yml/badge.svg)
![CodeQL](https://github.com/Clyra-AI/gait/actions/workflows/codeql.yml/badge.svg)
![Intent+Receipt Conformance](https://github.com/Clyra-AI/gait/actions/workflows/intent-receipt-conformance.yml/badge.svg)

```bash
make fmt && make lint && make test
make test-e2e
make test-hardening-acceptance
make test-uat-local
```

Push hooks: `make hooks` | Full gate: `GAIT_PREPUSH_MODE=full git push` | Branch protection: `make github-guardrails`

Contributor guide: [`CONTRIBUTING.md`](CONTRIBUTING.md)

## Command Surface

```text
gait demo                                         Create a signed pack offline
gait tour                                         Interactive walkthrough
gait verify <run_id|path>                          Verify integrity offline
gait verify chain|session-chain                    Multi-artifact chain verification
gait job submit|status|checkpoint|pause|resume     Durable job lifecycle
gait job approve|cancel|inspect                    Job approval and inspection
gait pack build|verify|inspect|diff|export         Unified pack operations + OTEL/Postgres sinks
gait regress init|bootstrap|run                    Incident → CI gate
gait gate eval                                     Policy enforcement + signed trace
gait approve                                       Mint signed approval tokens
gait delegate mint|verify                          Delegation token lifecycle
gait report top                                    Rank highest-risk actions
gait voice token mint|verify                       Voice commitment gating
gait voice pack build|verify|inspect|diff          Voice callpack operations
gait run record|inspect|replay|diff|receipt        Run recording and replay
gait run session start|append|status|checkpoint|compact  Session journaling
gait run reduce                                    Reduce runpack by predicate
gait mcp proxy|bridge|serve                        MCP transport adapters
gait policy init|validate|fmt|simulate|test        Policy authoring
gait doctor [--production-readiness] [adoption]    Diagnostics + readiness
gait keys init|rotate|verify                       Signing key lifecycle
gait scout snapshot|diff|signal                    Drift and adoption signals
gait guard pack|verify|retain|encrypt|decrypt      Evidence and encryption
gait trace verify                                  Verify signed trace integrity
gait incident pack                                 Build incident evidence pack
gait registry install|list|verify                  Signed skill-pack registry
gait migrate                                       Migrate legacy artifacts to v1
gait ui                                            Local playground
gait version                                       Print version
```

All commands support `--json`. Most support `--explain`.

## Feedback

Issues: [github.com/Clyra-AI/gait/issues](https://github.com/Clyra-AI/gait/issues) | Security: [`SECURITY.md`](SECURITY.md) | Contributing: [`CONTRIBUTING.md`](CONTRIBUTING.md) | Code of conduct: [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)
