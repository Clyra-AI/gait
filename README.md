# Gait — Policy-as-Code for AI Agent Tool Calls

Use Gait when an agent can cause real side effects and you need an execution-time verdict plus portable proof.

Gait is an offline-first CLI and runtime boundary. It is not an agent framework, not a hosted dashboard, and not a model host.

Start with `gait init` and `gait check`, evaluate structured tool-call intent with `gait gate eval`, and turn incidents into deterministic CI regressions with `gait capture`, `gait regress add`, or `gait regress bootstrap`.

Docs: [clyra-ai.github.io/gait](https://clyra-ai.github.io/gait/) | Install: [`docs/install.md`](docs/install.md) | Homebrew: [`docs/homebrew.md`](docs/homebrew.md)

## Overview

Gait gives you a truthful first-run path: bootstrap `.gait.yaml`, inspect the live policy contract, gate real tool execution, and keep portable evidence.

Managed/preloaded agent note: managed agents can use Gait at the tool boundary, but Gait does not host the model or replace your runtime.

## Why Gait

- Enforce `allow` / `block` / `require_approval` at the tool boundary before real side effects execute.
- Keep signed traces, packs, and callpacks as ticket-ready evidence instead of dashboard-only screenshots.
- Convert failures into deterministic CI regressions with stable exit codes and offline verification.
- Add durable jobs, MCP trust preflight, and voice gating on the same artifact and policy contract.

## The Boundary Contract

The integration rule is simple:

1. normalize a real tool action into structured intent
2. ask Gait for a verdict
3. execute the side effect only when `verdict == "allow"`
4. keep the resulting signed trace or pack

```python
def dispatch_tool(tool_call):
    decision = gait_evaluate(tool_call)
    if decision["verdict"] != "allow":
        return {"executed": False, "verdict": decision["verdict"]}
    return {"executed": True, "result": execute_real_tool(tool_call)}
```

Official onboarding lanes:

- Inline wrapper or sidecar: call `gait gate eval` in your dispatcher before real execution.
- MCP boundary: use `gait mcp verify` for trust preflight, then `gait mcp proxy` or `gait mcp serve`.
- Python SDK: thin subprocess ergonomics over the local Go binary, including official LangChain middleware with optional callback correlation.
- Observe-only path: `gait trace --json -- <child command...>` for integrations that already emit Gait trace references.

Other frameworks are named publicly only when an in-repo lane exists and clears the integration scorecard threshold.

Start here:

- [`examples/integrations/openai_agents/`](examples/integrations/openai_agents/)
- [`examples/integrations/langchain/`](examples/integrations/langchain/)
- [`docs/integration_checklist.md`](docs/integration_checklist.md)
- [`docs/agent_integration_boundary.md`](docs/agent_integration_boundary.md)
- [`docs/mcp_capability_matrix.md`](docs/mcp_capability_matrix.md)
- [`docs/sdk/python.md`](docs/sdk/python.md)

## When To Use Gait

- Tool-calling AI agents need enforceable allow/block/approval decisions.
- You need signed portable evidence for PRs, incidents, tickets, or audits.
- You want deterministic regressions that fail CI with stable exit behavior.
- You need additive durable jobs, MCP trust preflight, or voice gating on the same contract.

## When Not To Use Gait

- No local Gait CLI or Gait artifact path exists in the execution environment.
- Your workflow has no tool-side effects and no evidence requirements.
- You only need hosted observability dashboards and do not need offline verification or deterministic replay.

## Quickstart (Real Commands, Under 60 Seconds)

### Fast 20-Second Proof

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/install.sh | bash

# Bootstrap repo policy-as-code
gait init --json
gait check --json

# Create a portable demo artifact and verify it
gait demo
gait verify run_demo --json

# Turn the same artifact into a CI gate
gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml
```

`gait init --json` writes `.gait.yaml` and returns a real scaffold summary like:

```json
{
  "ok": true,
  "policy_path": ".gait.yaml",
  "template": "baseline-highrisk",
  "next_commands": [
    "gait check --policy .gait.yaml --json",
    "gait policy validate .gait.yaml --json",
    "gait gate eval --policy .gait.yaml --intent examples/policy/intents/intent_delete.json --json"
  ]
}
```

`gait check --json` validates the repo policy contract and reports the live policy shape:

```json
{
  "ok": true,
  "policy_path": ".gait.yaml",
  "default_verdict": "block",
  "rule_count": 7,
  "summary": "policy ok: default_verdict=block rules=7 gap_warnings=1"
}
```

`gait demo` prints a portable proof trail:

```text
run_id=run_demo
ticket_footer=GAIT run_id=run_demo ...
verify=ok
next=gait verify run_demo --json,gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml,...
```

For wrappers and SDKs, use `gait demo --json`; machine-readable clients should not scrape the human text form.

Integration touchpoints:

- Wrapper or sidecar: call `gait gate eval` at the exact tool-dispatch site before side effects execute.
- Context-required policies: pass `--context-envelope <context_envelope.json>` at that boundary on `gait gate eval`, `gait mcp proxy`, or `gait mcp serve`; raw `intent.context_set_digest`, `intent.context_evidence_mode`, or `context.auth_context.context_age_seconds` claims are treated as non-authoritative until the envelope is verified.
- MCP serve nuance: if the server starts with `--allow-client-artifact-paths`, same-host callers may provide `call.context.context_envelope_path`; otherwise keep context proof fixed at the serve boundary with `--context-envelope`.
- SDKs and automation: use `gait demo --json` for smoke checks and handoffs; the text form is human-facing only.
- Policy authors: when same-priority rules overlap, Gait applies the most restrictive verdict for that priority tier. Use a strictly lower numeric `priority` when one rule must win.

Migration notes:

- If an older integration relied on raw intent context fields to satisfy `require_context_evidence`, move that proof to a verified `--context-envelope` input.
- If tooling parsed `gait demo` text output, switch it to `gait demo --json` or the Python SDK helper surface.

No account. No API key. No hosted dependency.

## Simple End-To-End Scenario

See [`docs/scenarios/simple_agent_tool_boundary.md`](docs/scenarios/simple_agent_tool_boundary.md) and the promoted wrapper quickstart at `examples/integrations/openai_agents/quickstart.py`.

## What Ships In OSS

**Gate** — evaluate structured tool-call intent against YAML policy with fail-closed enforcement. Non-allow means non-execute. When multiple rules at the same priority match, Gait resolves that priority tier to the most restrictive verdict instead of depending on rule names.

**Evidence** — signed traces, runpacks, packs, and callpacks you can verify, diff, and attach to tickets, PRs, audits, and incidents.

**Regress** — `gait capture`, `gait regress add`, and `gait regress bootstrap` turn incidents into deterministic CI gates with JUnit output.

**Durable jobs** — checkpointed long-running work with pause/resume/cancel, approvals, and deterministic stop reasons.

**MCP trust** — evaluate local trust snapshots with `gait mcp verify`, then enforce via `gait mcp proxy` or `gait mcp serve`.

**Voice and context evidence** — fail-closed gating for spoken commitments and missing-context high-risk decisions.

## Differentiation

Gait is complementary to observability products.

- Observability tools help you inspect what happened after the fact.
- Gait decides whether a tool action may execute, emits a signed trace for that decision, and makes the artifact reusable in CI.
- External scanners and registries can feed Gait. Gait enforces at the action boundary; it does not replace the scanner.

## CI Adoption

```bash
gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml
```

- exit `0` means pass
- exit `5` means regression drift
- GitHub Actions template: [`.github/workflows/adoption-regress-template.yml`](.github/workflows/adoption-regress-template.yml)
- GitLab, Jenkins, CircleCI, and hook guidance: [`docs/ci_regress_kit.md`](docs/ci_regress_kit.md)
- Copy-paste rollout: [`docs/adopt_in_one_pr.md`](docs/adopt_in_one_pr.md)

## Compliance And Evidence

Put this copy at the bottom of launch surfaces: Gait produces signed evidence artifacts for operational proof.

- `gait verify`, `gait pack verify`, and `gait trace verify` work offline.
- Packs use Ed25519 signatures plus SHA-256 manifests.
- Artifacts are versioned, deterministic, and designed for change control, audit trails, PRs, and incident handoff.

Normative references:

- [`docs/contracts/primitive_contract.md`](docs/contracts/primitive_contract.md)
- [`docs/contracts/packspec_v1.md`](docs/contracts/packspec_v1.md)
- [`docs/contracts/intent_receipt_conformance.md`](docs/contracts/intent_receipt_conformance.md)
- [`docs/failure_taxonomy_exit_codes.md`](docs/failure_taxonomy_exit_codes.md)

Stable exit codes:

- `0` success
- `1` internal/runtime failure
- `2` verification failure
- `3` policy block
- `4` approval required
- `5` regress failed
- `6` invalid input
- `7` dependency missing
- `8` unsafe operation blocked

## Documentation

1. [`docs/README.md`](docs/README.md)
2. [`docs/policy_authoring.md`](docs/policy_authoring.md)
3. [`docs/integration_checklist.md`](docs/integration_checklist.md)
4. [`docs/agent_integration_boundary.md`](docs/agent_integration_boundary.md)
5. [`docs/ci_regress_kit.md`](docs/ci_regress_kit.md)
6. [`docs/mcp_capability_matrix.md`](docs/mcp_capability_matrix.md)

## Developer Workflow

![CI](https://github.com/Clyra-AI/gait/actions/workflows/ci.yml/badge.svg)
![CodeQL](https://github.com/Clyra-AI/gait/actions/workflows/codeql.yml/badge.svg)

```bash
make fmt && make lint && make test
make test-docs-consistency
make test-docs-storyline
```

Hooks: `make hooks` | Contributor guide: [`CONTRIBUTING.md`](CONTRIBUTING.md)

## Command Surface

```text
gait init|check                                    Repo policy bootstrap and validation
gait gate eval                                     Policy enforcement + signed trace
gait test|enforce                                  Bounded wrappers for explicit Gait-aware integrations
gait capture                                       Persist portable capture receipt from explicit source
gait regress add|init|bootstrap|run                Incident -> CI gate
gait mcp verify|proxy|bridge|serve                 MCP trust preflight and transport adapters
gait trace|trace verify                            Observe-only trace wrapper and trace integrity verification
gait demo|verify                                   First artifact and offline verification
gait pack build|verify|inspect|diff|export         Unified pack operations
gait job submit|status|checkpoint|pause|resume     Durable job lifecycle
gait job stop|approve|cancel|inspect               Emergency stop, approval, and inspection
gait voice token mint|verify                       Voice commitment gating
gait voice pack build|verify|inspect|diff          Voice callpack operations
gait doctor [--production-readiness] [adoption]    Diagnostics + readiness
gait policy init|validate|fmt|simulate|test        Policy authoring workflows
gait keys init|rotate|verify                       Signing key lifecycle
gait ui                                            Local playground
gait version [--json] [--explain]                  Print version
```

Most commands support `--json`. Machine-readable version output is available via `gait version --json`, `gait --version --json`, and `gait -v --json`. Root help (`gait --help`) is text-only and exits `0`. Most commands support `--explain`.

## Feedback

Issues: [github.com/Clyra-AI/gait/issues](https://github.com/Clyra-AI/gait/issues) | Security: [`SECURITY.md`](SECURITY.md) | Contributing: [`CONTRIBUTING.md`](CONTRIBUTING.md) | Code of conduct: [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)
