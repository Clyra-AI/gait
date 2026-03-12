# Gait Product Summary

Gait is the offline-first policy-as-code runtime for AI agent tool calls. It bootstraps repo policy with `gait init` and `gait check`, enforces fail-closed verdicts at the tool boundary, captures signed evidence, and turns incidents into deterministic CI regressions.

It provides seven OSS primitives:

1. **Gate**: Evaluate structured tool-call intent against YAML policy with fail-closed enforcement. Supports destructive plan/apply boundaries, destructive budgets, multi-step script rollups, Wrkr context enrichment, and signed approved-script fast-path allow.
2. **Evidence**: Signed traces, runpacks, packs, and callpacks with Ed25519 signatures and SHA-256 manifests.
3. **Regress**: Convert incidents into deterministic CI regression fixtures with JUnit output and stable exit codes through `gait capture`, `gait regress add`, and `gait regress bootstrap`.
4. **Jobs**: Dispatch multi-step, multi-hour agent work with checkpoints, pause/resume/stop/cancel, approval gates, deterministic stop reasons, and emergency-stop preemption evidence.
5. **Voice**: Gate high-stakes spoken commitments before they are uttered with SayToken capability tokens and callpack artifacts.
6. **Context Evidence**: Deterministic proof of what context the model was working from at decision time. Privacy-aware envelopes with fail-closed enforcement when evidence is missing.
7. **Doctor**: Diagnose first-run environment issues with stable JSON output.

Secondary boundary surfaces:

- **MCP Trust**: evaluate local trust snapshots for MCP server admission with `gait mcp verify`, `gait mcp proxy`, and `gait mcp serve`.
- **Trace**: observe-only wrapper mode with `gait trace` for integrations that already emit Gait trace references.
- **LangChain Middleware**: official Python middleware with optional callback correlation; callbacks never decide allow or block behavior.

Gait is vendor-neutral and offline-first for core workflows: capture, verify, diff, policy evaluation, regressions, and voice/context verification all run without network dependencies.

Tool boundary (canonical):

- exact call site where runtime is about to execute a real tool side effect
- adapter sends structured intent to Gait
- only `allow` executes tool side effects; non-allow outcomes are non-executing

When to use:

- agent tool calls can cause side effects and need fail-closed control
- incidents must become deterministic CI regressions
- teams need signed portable evidence instead of dashboard-only traces
- teams want truthful repo bootstrap commands before wiring a full integration

When not to use:

- no Gait CLI/artifact path is available in the runtime
- workflow has no tool-side effects and no evidence requirements
- you only want a hosted observability dashboard without offline verification or deterministic replay

Canonical docs:

- `/docs/policy_authoring/`
- `/docs/integration_checklist/`
- `/docs/agent_integration_boundary/`
- `/docs/adopt_in_one_pr/`
- `/docs/ci_regress_kit/`
- `/docs/mcp_capability_matrix/`
- `/docs/failure_taxonomy_exit_codes/`
