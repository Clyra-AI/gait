# Gait Product Summary

Gait is the offline-first policy-as-code runtime for AI agent tool calls. It bootstraps repo policy with `gait init` and `gait check`, enforces fail-closed verdicts at the tool boundary, captures signed evidence, and turns incidents into deterministic CI regressions.

Supporting promise: prove the install fast, enforce at the tool boundary when you own the seam, and graduate to hardened `oss-prod` readiness explicitly.

Repo bootstrap stays machine-readable: `gait doctor --json` is truthful for binary installs, `gait init --json` returns repo `detected_signals`, conservative `generated_rules`, and `unknown_signals`, and `gait check --json` reports structured `findings` plus install-safe `next_commands` for readiness follow-up.

Primary adoption checkpoints:

1. **Fast proof**: `gait version --json`, `gait doctor --json`, `gait demo`, `gait verify run_demo --json`, and `gait regress bootstrap --from run_demo --json --junit ...` validate install, evidence, and CI wiring.
2. **Strict inline enforcement**: place Gait at the real wrapper, sidecar, middleware, or MCP execution seam so non-`allow` outcomes do not execute side effects.
3. **Hardened `oss-prod` readiness**: seed `examples/config/oss_prod_template.yaml`, run `gait check --json`, then require `gait doctor --production-readiness --json` to return `ok=true`.

Secondary boundary surfaces:

- **Doctor**: truthful first-run diagnostics and the explicit production-readiness gate.
- **Evidence**: signed traces, runpacks, packs, and callpacks with Ed25519 signatures and SHA-256 manifests.
- **Regress**: convert incidents into deterministic CI regression fixtures with JUnit output and stable exit codes through `gait capture`, `gait regress add`, and `gait regress bootstrap`.
- **Jobs**: dispatch multi-step, multi-hour agent work with checkpoints, pause/resume/stop/cancel, approval gates, deterministic stop reasons, and emergency-stop preemption evidence.
- **Voice**: gate high-stakes spoken commitments before they are uttered with SayToken capability tokens and callpack artifacts.
- **Context Evidence**: deterministic proof of what context the model was working from at decision time, with fail-closed checks bound to a verified `--context-envelope`.
- **MCP Trust**: evaluate local trust snapshots for MCP server admission with `gait mcp verify`, `gait mcp proxy`, and `gait mcp serve`.
- **Effect Evidence**: grade signed, bounded before/after `effect_snapshot` evidence against typed `effect_contract` predicates with `gait effects grade`; authoritative passes require a trusted collector public key.
- **Trace**: observe-only wrapper mode with `gait trace` for integrations that already emit Gait trace references.
- **LangChain Middleware**: official Python middleware with optional callback correlation; callbacks never decide allow or block behavior, and demo capture stays bound to `gait demo --json`.
- **OpenAI Agents Reference Demo**: in-repo boundary demo showing the wrapper contract with deterministic allow, block, and approval outcomes. It is not a package-backed official SDK lane.
- **Reference Adapters**: `examples/integrations/claude_code/` remains a reference lane; its hook/runtime/input errors fail closed by default and `GAIT_CLAUDE_UNSAFE_FAIL_OPEN=1` is an unsafe opt-in override.

Official framework lane today:

- LangChain middleware

The OpenAI Agents path in this repo is a reference boundary demo. Reference adapters stay in-repo but outside official-lane claims until they clear the promotion scorecard. CrewAI is not an official lane today.

Gait is vendor-neutral and offline-first for core workflows: capture, verify, diff, policy evaluation, regressions, and voice/context verification all run without network dependencies.

Tool boundary (canonical):

- exact call site where runtime is about to execute a real tool side effect
- adapter sends structured intent to Gait
- only `allow` executes tool side effects; non-allow outcomes are non-executing
- quickstart/demo proof is valuable without that seam, but it is not evidence of strict inline enforcement by itself

When to use:

- agent tool calls can cause side effects and need fail-closed control
- incidents must become deterministic CI regressions
- teams need signed portable evidence instead of dashboard-only traces
- teams want truthful repo bootstrap commands before wiring a full integration
- teams need a documented path from demo mode to `oss-prod` readiness using `examples/config/oss_prod_template.yaml` from a repo checkout, or that same file fetched after a binary-only install, plus `gait doctor --production-readiness --json`

When not to use:

- no Gait CLI/artifact path is available in the runtime
- workflow has no tool-side effects and no evidence requirements
- you only want a hosted observability dashboard without offline verification or deterministic replay

Observability comparison:

- LangSmith, Langfuse, and AgentOps focus on hosted tracing and analytics.
- Gait is the execution-boundary gate that decides whether the tool action may run and emits signed evidence for CI and audit reuse.
- The model is camera plus gate, not camera or gate.

Canonical docs:

- `/docs/policy_authoring/`
- `/docs/integration_checklist/`
- `/docs/agent_integration_boundary/`
- `/docs/adopt_in_one_pr/`
- `/docs/ci_regress_kit/`
- `/docs/mcp_capability_matrix/`
- `/docs/failure_taxonomy_exit_codes/`
