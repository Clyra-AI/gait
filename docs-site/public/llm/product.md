# Gait Product Summary

Gait is an offline-first runtime for production AI agents that dispatches durable jobs, captures signed evidence, and enforces fail-closed policy at the tool boundary.

It provides seven OSS primitives:

1. **Jobs**: Dispatch multi-step, multi-hour agent work with checkpoints, pause/resume/cancel, approval gates, and deterministic stop reasons.
2. **Packs**: Unified portable artifact envelope (PackSpec v1) for run, job, and call evidence with Ed25519 signatures and SHA-256 manifest.
3. **Gate**: Evaluate structured tool-call intent against YAML policy with fail-closed enforcement. Supports multi-step script rollups, Wrkr context enrichment, and signed approved-script fast-path allow.
4. **Regress**: Convert any incident or failed run into a deterministic CI regression fixture with JUnit output and stable exit codes.
5. **Voice**: Gate high-stakes spoken commitments (refunds, quotes, eligibility) before they are uttered. Signed SayToken capability tokens and callpack artifacts for voice boundaries.
6. **Context Evidence**: Deterministic proof of what context the model was working from at decision time. Privacy-aware envelopes with fail-closed enforcement when evidence is missing.
7. **Doctor**: Diagnose first-run environment issues with stable JSON output.

Gait is vendor-neutral and offline-first for core workflows: capture, verify, diff, policy evaluation, regressions, and voice/context verification all run without network dependencies.

Tool boundary (canonical):

- exact call site where runtime is about to execute a real tool side effect
- adapter sends structured intent to Gait
- only `allow` executes tool side effects; non-allow outcomes are non-executing

When to use:

- agent tool calls can cause side effects and need fail-closed control
- incidents must become deterministic CI regressions
- teams need signed portable evidence instead of dashboard-only traces

When not to use:

- no Gait CLI/artifact path is available in the runtime
- workflow has no tool-side effects and no evidence requirements

Canonical docs:

- `/docs/adopt_in_one_pr/`
- `/docs/durable_jobs/`
- `/docs/integration_checklist/`
- `/docs/architecture/`
- `/docs/flows/`
- `/docs/threat_model/`
- `/docs/failure_taxonomy_exit_codes/`
