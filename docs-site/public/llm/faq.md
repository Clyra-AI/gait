# Gait FAQ (LLM Context)

## What is the primary job of Gait?

Gait enforces fail-closed policy before agent tool side effects execute and keeps signed evidence you can verify offline.

## What should teams run first?

Run `gait version --json`, `gait doctor --json`, `gait init --json`, `gait check --json`, `gait demo` for the operator path, `gait demo --json` for wrappers/SDKs, then `gait verify run_demo --json` and `gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml`.

`gait doctor --json` stays install-safe in a clean writable directory. `gait init --json` returns `detected_signals`, conservative `generated_rules`, and `unknown_signals`. `gait check --json` reports structured `findings` and install-safe `next_commands` in addition to compatibility `gap_warnings`.

## What problem does Gait solve for long-running agent work?

Multi-step and multi-hour agent jobs fail mid-flight, losing state and provenance. Gait dispatches durable jobs with checkpointed state, pause/resume/stop/cancel, and deterministic stop reasons so work survives failures and stays auditable.

## Is Gait a hosted SaaS dashboard?

No. Gait is CLI-first and offline-first for core workflows. Capture, verify, diff, policy evaluation, regressions, and voice/context verification all run locally without network dependencies.

## Where should policy be enforced?

At tool-call execution intent, not prompt text alone. Non-allow gate outcomes do not execute side effects. Policy is expressed in YAML and evaluated deterministically.

## What is the tool boundary in concrete terms?

The tool boundary is the exact call site in your wrapper or adapter where a real tool side effect is about to execute. The adapter sends structured intent to Gait and only executes the tool when verdict is `allow`.

## How do I turn a failed agent run into a CI gate?

Run `gait regress bootstrap --from <run_id> --junit output.xml`. This converts the run into a permanent regression fixture. Exit 0 means pass, exit 5 means drift. Wire the JUnit output into any CI system.

## Can Gait gate voice agent actions?

Yes. Voice mode gates high-stakes spoken commitments before they are uttered. A signed SayToken capability token must be present for gated speech, and every call produces a signed callpack artifact.

## What is context evidence?

Context evidence is deterministic proof of what context material the model was working from at decision time. Gait captures privacy-aware context envelopes and enforces fail-closed policy when evidence is missing for high-risk actions.

For context-required policies, the gate only trusts a verified `--context-envelope` input; raw context digest, mode, or age claims inside the intent are not sufficient on their own.

## How do I know install is valid and high-risk enforcement is production-ready?

Use `gait version --json` as the machine-readable install probe. For `oss-prod`, use the canonical template at `examples/config/oss_prod_template.yaml`: copy it from a repo checkout, or fetch that same file after a binary-only install, then run `gait check --json` and require `gait doctor --production-readiness --json` to return `ok=true` before treating high-risk enforcement as production-ready.

## Can I replay an agent run without re-executing real API calls?

Yes. `gait run replay` uses recorded results as deterministic stubs so you can debug safely. `gait pack diff` then shows exactly what changed between two runs, including context drift classification.

## How does Gait integrate with agent frameworks?

Gait provides wrapper or sidecar, Python SDK, and MCP boundary modes. The official LangChain surface is middleware with optional callback correlation; enforcement still happens only at the tool boundary. Claude Code remains a reference adapter, and its hook/runtime/input errors fail closed by default unless an operator explicitly opts into unsafe fail-open behavior.

Official lanes today are OpenAI Agents and LangChain. Reference adapters stay in-repo, but they are not promoted into official launch claims until they clear the scorecard threshold. CrewAI is not an official lane today.

If you use `gait test`, `gait enforce`, or `gait trace`, the child integration must emit a `trace_path=<path>` seam. Wrapper JSON makes that explicit with `boundary_contract=explicit_trace_reference`, `trace_reference_required=true`, and stable `failure_reason` values such as `missing_trace_reference` or `invalid_trace_artifact`.

`gait mcp verify` is also local-snapshot-based: `mcp_trust.snapshot` points to a local file, and `gait mcp verify --json` reports `trust_model=local_snapshot` and `snapshot_path` rather than performing a hosted registry lookup.

## Why not just use LangSmith, Langfuse, or AgentOps?

Those tools are useful for hosted tracing and analytics. Gait solves a different problem: it gates execution before tool side effects happen and emits signed evidence that can be reused in CI, incident review, and audit trails.

The practical model is camera plus gate, not camera or gate.

## Can Gait pre-approve known multi-step scripts?

Yes. Use `gait approve-script` to mint signed registry entries bound to policy digest and script hash, then evaluate with `gait gate eval --approved-script-registry ... --approved-script-public-key ...`. Invalid or tampered registry state fails closed in high-risk paths, and missing verify keys only disable the fast path in standard low-risk mode.

## How should teams start?

Bootstrap `.gait.yaml` with `gait init` and `gait check`, run `gait demo --json` for automation, then wire one integration seam from the integration checklist at the real tool-dispatch boundary.
