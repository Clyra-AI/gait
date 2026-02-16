# Gait — Run Durable Agent Jobs, Capture Tool Calls, Prove What Happened

![CI](https://github.com/davidahmann/gait/actions/workflows/ci.yml/badge.svg)
![CodeQL](https://github.com/davidahmann/gait/actions/workflows/codeql.yml/badge.svg)
![Intent+Receipt Conformance](https://github.com/davidahmann/gait/actions/workflows/intent-receipt-conformance.yml/badge.svg)

Agents are capable enough to execute real work. The limiting factor is not intelligence — it is control, proof, and reproducibility. Long-running work fails mid-flight with no recovery. State-changing tool calls are impossible to reconstruct. Post-hoc logs are not dispute-grade evidence.

Gait is an offline-first Go CLI that fixes this. Dispatch durable jobs with checkpointed state. Capture every tool call as a signed pack. Verify, diff, and replay offline. Turn incidents into deterministic CI regressions. Gate high-risk actions before side effects execute.

## In Plain Language

Use Gait when an AI agent can cause real side effects and you need deterministic control plus portable proof.

Incident-to-surface mapping:

- Agent triggers a destructive action you cannot justify later -> `gait gate eval` + signed trace + fail-closed non-execute.
- Agent run fails and cannot be reproduced -> `gait run record`/`gait pack build` + `gait regress bootstrap`.
- Long-running work crashes midway and state is unclear -> `gait job submit|checkpoint|inspect` + job pack verification.

## When To Use Gait

- Tool-calling AI agents need enforceable allow/block/approval decisions.
- You need signed, portable evidence artifacts for PRs, incidents, or audits.
- You want offline, deterministic regressions that fail CI with stable exit behavior.
- You run multi-step jobs and need checkpoints, pause/resume/cancel, and inspectable state.

## When Not To Use Gait

- No local Gait CLI or Gait artifacts are available in the execution path.
- Your workflow only needs prompt orchestration without tool-side effects or evidence contracts.
- You only need hosted observability dashboards and do not need offline verification or deterministic replay.

**For platform and AI engineering teams** — run multi-step, multi-hour agent jobs without losing work, state, or provenance.

**For security engineers and production owners** — every high-risk tool call passes a deterministic policy decision with portable, independently verifiable proof.

## Try It (Offline, <60s)

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/install.sh | bash

# Create a signed pack from a synthetic agent run
gait demo

# Prove it's intact
gait verify run_demo

# Turn it into a CI regression gate — one command
gait regress bootstrap --from run_demo --junit ./gait-out/junit.xml
```

No account. No API key. No internet. You now have a verified artifact and a permanent regression test.

Install details: [`docs/install.md`](docs/install.md) | Homebrew: [`docs/homebrew.md`](docs/homebrew.md)
Local UI playground: [`docs/ui_localhost.md`](docs/ui_localhost.md) | Launch with `gait ui`

## See It

![Gait simple end-to-end tool-boundary scenario](docs/assets/gait_demo_simple_e2e_60s.gif)

Video: [`gait_demo_simple_e2e_60s.mp4`](docs/assets/gait_demo_simple_e2e_60s.mp4) | Scenario walkthrough: [`docs/scenarios/simple_agent_tool_boundary.md`](docs/scenarios/simple_agent_tool_boundary.md) | Output legend: [`docs/demo_output_legend.md`](docs/demo_output_legend.md)

## Simple End-To-End Scenario

Run the canonical wrapper path in `examples/integrations/openai_agents/quickstart.py` to see allow/block/require-approval outcomes at the tool boundary.

## Fast 20-Second Proof

Use the short demo assets in `docs/assets/gait_demo_20s.*` for a quick artifact-and-verify walkthrough.

## What You Get

**Durable jobs** — dispatch long-running agent work that survives failures. Checkpoints, pause/resume/cancel, approval gates, deterministic stop reasons. No more lost state at step 47.

**Signed packs** — every run and job emits a tamper-evident artifact (Ed25519 + SHA-256 manifest). Verify offline. Attach to PRs, incidents, audits. One artifact is the entire proof.

**Incident → CI gate in one command** — `gait regress bootstrap` converts a bad run into a permanent regression fixture with JUnit output. Exit 0 = pass, exit 5 = drift. Never debug the same failure twice.

**Fail-closed policy enforcement** — `gait gate eval` evaluates a structured tool-call intent against YAML policy before the side effect runs. Non-allow means non-execute. Signed trace proves the decision.

**Deterministic replay and diff** — replay an agent run using recorded results as stubs (no real API calls). Diff two packs to see what changed, including context drift classification.

**Voice agent gating** — gate high-stakes spoken commitments (refunds, quotes, eligibility) before they're uttered. Signed `SayToken` capability + callpack artifacts for voice boundaries.

**Risk ranking** — rank highest-risk actions across runs and traces by tool class and blast radius. Offline, no dashboard.

## One-Command Workflows

```bash
# Block a destructive tool call
gait gate eval --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_delete.json --json

# Rank top risks across all your runs
gait report top --runs ./gait-out --traces ./gait-out --limit 5

# Dispatch a durable job with checkpoint
gait job submit --id job_1 --json
gait job checkpoint add --id job_1 --summary "step complete" --json
gait pack build --type job --from job_1 --json

# Gate a voice commitment before it's spoken
gait voice token mint --intent commitment.json --policy policy.yaml --json
```

## Integrations

Gait enforces at the tool boundary, not the prompt boundary. Your dispatcher calls Gait; non-`allow` means non-execute.

Managed/preloaded agent note: if your platform does not expose a tool-call interception point, use observe/report/regress workflows; full fail-closed enforcement requires boundary interception.

Tool boundary (canonical definition):

- The exact call site where your runtime is about to execute a real tool side effect.
- The adapter serializes a structured intent (`IntentRequest`) and calls Gait (`gait gate eval` or `gait mcp serve`).
- Enforcement rule is strict: if verdict is not `allow`, do not execute the tool.

Blessed lane: [`examples/integrations/openai_agents/`](examples/integrations/openai_agents/)

```python
def dispatch_tool(tool_call):
    decision = gait_evaluate(tool_call)
    if decision["verdict"] != "allow":
        return {"executed": False, "verdict": decision["verdict"]}
    return {"executed": True, "result": execute_real_tool(tool_call)}
```

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

## Production Posture

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_delete.json \
  --profile oss-prod --key-mode prod \
  --private-key ./gait-out/keys/prod_private.key --json

gait doctor --production-readiness --json
```

Hardening: [`docs/hardening/v2_2_contract.md`](docs/hardening/v2_2_contract.md) | Runbook: [`docs/hardening/prime_time_runbook.md`](docs/hardening/prime_time_runbook.md)

## Command Surface

```text
gait demo                                         Create a signed pack offline
gait verify <run_id|path>                          Verify integrity offline
gait job submit|status|checkpoint|pause|resume     Durable job lifecycle
gait pack build|verify|inspect|diff                Unified pack operations
gait regress bootstrap|run                         Incident → CI gate
gait gate eval                                     Policy enforcement + signed trace
gait report top                                    Rank highest-risk actions
gait voice token mint|verify                       Voice commitment gating
gait voice pack build|verify|inspect|diff          Voice callpack operations
gait run record|inspect|replay|diff|receipt        Run recording and replay
gait mcp proxy|serve                               MCP transport adapters
gait policy init|validate|fmt|simulate|test        Policy authoring
gait doctor                                        Diagnostics + readiness
gait keys init|rotate|verify                       Signing key lifecycle
gait scout snapshot|diff|signal                    Drift and adoption signals
gait guard pack|verify|retain|encrypt|decrypt      Evidence and encryption
gait registry install|list|verify                  Signed skill-pack registry
gait ui                                            Local playground
```

All commands support `--json`. Most support `--explain`.

## Contract Commitments

- **determinism**: verify, diff, and stub replay produce identical results on identical artifacts
- **offline-first**: core workflows do not require network
- **fail-closed**: high-risk paths block on policy or approval ambiguity
- **schema stability**: versioned artifacts with backward-compatible readers
- **stable exit codes**: `0` success · `1` internal/runtime failure · `2` verification failure · `3` policy block · `4` approval required · `5` regress failed · `6` invalid input · `7` dependency missing · `8` unsafe operation blocked

Normative spec: [`docs/contracts/primitive_contract.md`](docs/contracts/primitive_contract.md) | PackSpec v1: [`docs/contracts/packspec_v1.md`](docs/contracts/packspec_v1.md) | Intent+receipt: [`docs/contracts/intent_receipt_conformance.md`](docs/contracts/intent_receipt_conformance.md)

## Developer Workflow

```bash
make fmt && make lint && make test
make test-e2e
make test-hardening-acceptance
make test-uat-local
```

Push hooks: `make hooks` | Full gate: `GAIT_PREPUSH_MODE=full git push` | Branch protection: `make github-guardrails`

Contributor guide: [`CONTRIBUTING.md`](CONTRIBUTING.md)

## Documentation

1. [`docs/README.md`](docs/README.md) — ownership map
2. [`docs/concepts/mental_model.md`](docs/concepts/mental_model.md) — how Gait works
3. [`docs/architecture.md`](docs/architecture.md) — component boundaries
4. [`docs/flows.md`](docs/flows.md) — end-to-end sequences
5. [`docs/durable_jobs.md`](docs/durable_jobs.md) — durable job lifecycle and differentiation
6. [`docs/contracts/primitive_contract.md`](docs/contracts/primitive_contract.md) — normative spec

Public docs: [https://davidahmann.github.io/gait/](https://davidahmann.github.io/gait/) | Wiki: [https://github.com/davidahmann/gait/wiki](https://github.com/davidahmann/gait/wiki) | Changelog: [CHANGELOG.md](CHANGELOG.md)

## Feedback

Issues: [github.com/davidahmann/gait/issues](https://github.com/davidahmann/gait/issues) | Security: [`SECURITY.md`](SECURITY.md) | Contributing: [`CONTRIBUTING.md`](CONTRIBUTING.md) | Code of conduct: [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)
