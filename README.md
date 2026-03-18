# Gait

Policy-as-code for AI agent tool calls.

Gait sits at the execution boundary between an agent decision and a real tool call. It evaluates structured intent, blocks non-`allow` decisions before side effects land, emits signed proof you can verify offline, and turns failures into deterministic CI regressions.

Offline-first. Fail-closed. Portable evidence.

Docs: [clyra-ai.github.io/gait](https://clyra-ai.github.io/gait/) | Install: [`docs/install.md`](docs/install.md) | Examples: [`examples/integrations/`](examples/integrations/) | Command docs: [`docs/README.md`](docs/README.md)

## In Brief

Gait is a local CLI and runtime boundary for tool-calling agents. It is not an agent framework, not a model host, and not a hosted dashboard.

Managed/preloaded agent note: if you cannot intercept tool execution before side effects, Gait still provides observe, verify, capture, and regress workflows. Strict inline fail-closed enforcement starts only when you control the execution boundary.

## Install

Choose one install path:

### Homebrew

```bash
brew install Clyra-AI/tap/gait
```

### Go Install

```bash
go install github.com/Clyra-AI/gait/cmd/gait@latest
```

### Release Installer

```bash
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/install.sh | bash
```

## Start Here

Choose the path that matches what you need right now.

### Fast 20-Second Proof

Use this when you want to validate the install, create one real artifact, and wire the first deterministic CI gate without integrating into an agent yet.

```bash
gait version --json
gait doctor --json
gait demo
gait verify run_demo --json
gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml
```

This path gives you:

- a truthful install and environment check via `gait doctor --json`
- one signed demo artifact you can verify offline
- one deterministic regress gate you can drop into CI immediately

### Add Repo Policy To A Real Project

Use this when you want a repo-root policy file and a local contract check.

```bash
gait init --json
gait check --json
```

This path writes `.gait.yaml`, reports the live policy contract, and returns install-safe next commands.

### Integrate At The Runtime Boundary

Use this when your agent already makes real tool calls and you want enforcement at the execution seam.

Official lanes:

- OpenAI Agents wrapper lane: [`examples/integrations/openai_agents/`](examples/integrations/openai_agents/)
- LangChain middleware lane: [`examples/integrations/langchain/`](examples/integrations/langchain/)

Other supported boundary paths:

- generic wrapper or sidecar calling `gait gate eval` before real execution
- MCP trust and transport boundary via `gait mcp verify`, `gait mcp proxy`, or `gait mcp serve`

No account. No API key. No hosted dependency.

## Why Gait

Agent frameworks decide what to do. Gait decides whether the tool action may execute.

Use Gait when you need to:

- enforce `allow`, `block`, or `require_approval` before a real side effect happens
- keep signed traces, packs, and callpacks as ticket-ready evidence
- convert incidents into deterministic CI regressions with stable exit codes
- add MCP trust preflight, context evidence, durable jobs, or voice gating on the same artifact contract

## When To Use Gait

- Your agent can cause real side effects and you need `allow`, `block`, or `require_approval` at execution time.
- You want signed portable evidence for PRs, incidents, tickets, or audits.
- You need deterministic regressions that fail CI with stable exit behavior.
- You want one contract across wrappers, middleware, sidecars, and MCP boundaries.

## When Not To Use Gait

- You do not have a real interception seam before tool execution.
- Your workflow has no tool-side effects and no evidence requirement.
- You only need hosted observability and do not need offline verification or deterministic regression.
- You need a vulnerability scanner, model host, or orchestration framework rather than an execution-boundary control layer.

## The Boundary Contract

Every integration path should implement the same rule:

1. Normalize a real tool action into structured intent.
2. Ask Gait for a verdict.
3. Execute the side effect only when `verdict == "allow"`.
4. Keep the signed trace, runpack, or pack.

```python
def dispatch_tool(tool_call):
    decision = gait_evaluate(tool_call)
    if decision["verdict"] != "allow":
        return {"executed": False, "verdict": decision["verdict"]}
    return {"executed": True, "result": execute_real_tool(tool_call)}
```

This is the core contract across wrappers, middleware, sidecars, and MCP boundaries.

## Runtime Integration Paths

### OpenAI Agents

This is the blessed top-of-funnel runtime lane: a local wrapper at the tool boundary with deterministic allow, block, and approval quickstarts.

```bash
python3 examples/integrations/openai_agents/quickstart.py --scenario allow
python3 examples/integrations/openai_agents/quickstart.py --scenario block
python3 examples/integrations/openai_agents/quickstart.py --scenario require_approval
```

See [`examples/integrations/openai_agents/`](examples/integrations/openai_agents/).

### LangChain

The official LangChain surface is middleware with optional callback correlation. Enforcement happens in `wrap_tool_call`; callbacks are additive only.

`run_session(...)` and other Python run-capture helpers delegate digest completion to `gait run record` in Go rather than hashing artifact fields in Python. Normalize `set` values to JSON lists before calling the SDK; unsupported non-JSON values now fail deterministically instead of being coerced into digest-affecting output.

```bash
(cd sdk/python && uv sync --extra langchain --extra dev)
(cd sdk/python && uv run --python 3.13 --extra langchain python ../../examples/integrations/langchain/quickstart.py --scenario allow)
```

See [`examples/integrations/langchain/`](examples/integrations/langchain/) and [`docs/sdk/python.md`](docs/sdk/python.md).

### Generic Wrapper Or Sidecar

If your framework is not an official lane, put Gait at the dispatcher boundary and call `gait gate eval` immediately before the real side effect.

See [`docs/agent_integration_boundary.md`](docs/agent_integration_boundary.md), [`docs/integration_checklist.md`](docs/integration_checklist.md), and [`examples/sidecar/README.md`](examples/sidecar/README.md).

### Bounded Wrapper Commands

`gait test`, `gait enforce`, and `gait trace` are real commands, but they are bounded wrappers for explicit Gait-aware integrations that emit trace references. They do not auto-instrument arbitrary runtimes.

```bash
gait trace --json -- <child command...>
gait test --json -- <child command...>
gait enforce --json -- <child command...>
```

## Simple End-To-End Scenario

See [`docs/scenarios/simple_agent_tool_boundary.md`](docs/scenarios/simple_agent_tool_boundary.md) and the promoted wrapper quickstart at `examples/integrations/openai_agents/quickstart.py`.

## Policy Onboarding

The repo-root policy contract is `.gait.yaml`.

`gait init --json` writes the starter file. `gait check --json` validates it and reports the live contract.

Minimal shape:

```yaml
schema_id: gait.gate.policy
schema_version: 1.0.0
default_verdict: block
mcp_trust:
  enabled: true
  snapshot: ./examples/integrations/mcp_trust/trust_snapshot.json
rules:
  - name: require-approval-tool-write
    priority: 20
    effect: require_approval
    match:
      tool_names: [tool.write]
```

Policy docs:

- [`docs/policy_authoring.md`](docs/policy_authoring.md)
- [`schemas/v1/gate/policy.schema.json`](schemas/v1/gate/policy.schema.json)

## Regress And CI

Gait turns incidents into deterministic CI gates.

One-command bootstrap path:

```bash
gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml
```

Explicit handoff path:

```bash
gait capture --from run_demo --json
gait regress add --from ./gait-out/capture.json --json
gait regress run --json --junit ./gait-out/junit.xml
```

CI adoption assets:

- GitHub Actions template: [`.github/workflows/adoption-regress-template.yml`](.github/workflows/adoption-regress-template.yml)
- GitLab, Jenkins, CircleCI, and hook guidance: [`docs/ci_regress_kit.md`](docs/ci_regress_kit.md)
- One-PR rollout guide: [`docs/adopt_in_one_pr.md`](docs/adopt_in_one_pr.md)

Stable regress exits:

- `0` pass
- `5` deterministic regression failure

Stable exit codes:

- `0` success
- `1` internal or runtime failure
- `2` verification failure
- `3` policy block
- `4` approval required
- `5` deterministic regression failure
- `6` invalid input
- `7` dependency missing
- `8` unsafe operation blocked

## MCP Trust

Gait can preflight and enforce MCP trust at the connection boundary.

Current shipped model:

- external scanners or registries produce a local trust snapshot
- `gait mcp verify`, `gait mcp proxy`, and `gait mcp serve` consume that local file
- Gait enforces the decision at the boundary; it does not replace the scanner
- duplicate normalized `server_id` / `server_name` entries invalidate the snapshot, and required high-risk trust paths fail closed on that ambiguity

This is the right split with tools such as Snyk: external tooling finds the issue, and Gait enforces the runtime response.

See [`examples/integrations/mcp_trust/README.md`](examples/integrations/mcp_trust/README.md), [`docs/mcp_capability_matrix.md`](docs/mcp_capability_matrix.md), and [`docs/external_tool_registry_policy.md`](docs/external_tool_registry_policy.md).

## Gait Vs Observability

Gait is complementary to observability products such as LangSmith, Langfuse, and AgentOps.

- LangSmith, Langfuse, and AgentOps focus on hosted tracing, analytics, and after-the-fact inspection.
- Gait evaluates structured action intent, enforces at the execution boundary, and emits signed artifacts you can reuse in CI and incident response.
- The practical model is camera plus gate: use observability to inspect, and use Gait to block or gate high-risk tool actions before they land.

## What Gait Does Not Do

- Gait is not an agent framework, orchestrator, model host, or hosted dashboard.
- Gait does not auto-instrument arbitrary runtimes without an interception seam.
- Gait is not a vulnerability scanner.
- Gait does not replace your tracing, analytics, SIEM, or scanner stack.

## What Ships In OSS

- **Gate**: structured policy evaluation with fail-closed enforcement
- **Evidence**: signed traces, runpacks, packs, and callpacks
- **Regress**: deterministic incident-to-CI workflows
- **Durable jobs**: checkpointed long-running work with pause, resume, cancel, and approvals
- **MCP trust**: trust preflight plus proxy, bridge, and serve boundaries
- **Voice and context evidence**: fail-closed gating for spoken commitments and missing-context high-risk actions

## Compliance And Evidence

Every Gait decision can produce signed proof artifacts that map to operational and audit evidence.

- `gait verify`, `gait pack verify`, and `gait trace verify` work offline
- packs use Ed25519 signatures plus SHA-256 manifests
- duplicate ZIP entry names are treated as verification failures rather than ambiguous soft passes
- artifacts are deterministic, versioned, and designed for PRs, incidents, change control, and audits

Framework mapping and evidence docs:

- [`docs/contracts/primitive_contract.md`](docs/contracts/primitive_contract.md)
- [`docs/contracts/packspec_v1.md`](docs/contracts/packspec_v1.md)
- [`docs/contracts/intent_receipt_conformance.md`](docs/contracts/intent_receipt_conformance.md)
- [`docs/failure_taxonomy_exit_codes.md`](docs/failure_taxonomy_exit_codes.md)

## Learn More

- Install: [`docs/install.md`](docs/install.md)
- Policy authoring: [`docs/policy_authoring.md`](docs/policy_authoring.md)
- Integration checklist: [`docs/integration_checklist.md`](docs/integration_checklist.md)
- Boundary guide: [`docs/agent_integration_boundary.md`](docs/agent_integration_boundary.md)
- Python SDK: [`docs/sdk/python.md`](docs/sdk/python.md)
- MCP capability matrix: [`docs/mcp_capability_matrix.md`](docs/mcp_capability_matrix.md)
- CI regress kit: [`docs/ci_regress_kit.md`](docs/ci_regress_kit.md)
- Docs index: [`docs/README.md`](docs/README.md)

## Command Surface

```text
gait init|check                                    Repo policy bootstrap and validation
gait gate eval                                     Policy evaluation + signed trace
gait test|enforce                                  Bounded wrappers for explicit Gait-aware integrations
gait capture                                       Persist portable capture receipt from explicit source
gait regress add|init|bootstrap|run                Incident -> CI gate
gait mcp verify|proxy|bridge|serve                 MCP trust preflight and transport adapters
gait trace|trace verify                            Observe-only wrapper and trace integrity verification
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

## Feedback

Issues: [github.com/Clyra-AI/gait/issues](https://github.com/Clyra-AI/gait/issues) | Security: [`SECURITY.md`](SECURITY.md) | Contributing: [`CONTRIBUTING.md`](CONTRIBUTING.md) | Code of conduct: [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)
