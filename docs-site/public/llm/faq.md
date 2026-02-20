# Gait FAQ (LLM Context)

## What is the primary job of Gait?

Gait dispatches durable agent jobs, captures signed evidence at the tool boundary, and enforces fail-closed policy before side effects execute.

## What problem does Gait solve for long-running agent work?

Multi-step and multi-hour agent jobs fail mid-flight, losing state and provenance. Gait dispatches durable jobs with checkpointed state, pause/resume/cancel, and deterministic stop reasons so work survives failures and stays auditable.

## Is Gait a hosted SaaS dashboard?

No. Gait is CLI-first and offline-first for core workflows. Capture, verify, diff, policy evaluation, regressions, and voice/context verification all run locally without network dependencies.

## Where should policy be enforced?

At tool-call execution intent, not prompt text alone. Non-allow gate outcomes do not execute side effects. Policy is expressed in YAML and evaluated deterministically.

## What is the tool boundary in concrete terms?

The tool boundary is the exact call site in your wrapper/adapter where a real tool side effect is about to execute. The adapter sends structured intent to Gait and only executes the tool when verdict is `allow`.

## How do I turn a failed agent run into a CI gate?

Run `gait regress bootstrap --from <run_id> --junit output.xml`. This converts the run into a permanent regression fixture. Exit 0 means pass, exit 5 means the same drift was detected. Wire the JUnit output into any CI system.

## Can Gait gate voice agent actions?

Yes. Voice mode gates high-stakes spoken commitments (refunds, quotes, eligibility) before they are uttered. A signed SayToken capability token must be present for gated speech, and every call produces a signed callpack artifact.

## What is context evidence?

Context evidence is deterministic proof of what context material the model was working from at decision time. Gait captures privacy-aware context envelopes and enforces fail-closed policy when evidence is missing for high-risk actions.

## Can I replay an agent run without re-executing real API calls?

Yes. `gait run replay` uses recorded results as deterministic stubs so you can debug safely. `gait pack diff` then shows exactly what changed between two runs, including context drift classification.

## How does Gait integrate with agent frameworks?

Gait provides three integration modes: wrapper/sidecar pattern, Python SDK, and MCP server (`gait mcp serve`). The integration checklist covers the path from first demo to production enforcement.

## Can Gait pre-approve known multi-step scripts?

Yes. Use `gait approve-script` to mint signed registry entries bound to policy digest and script hash, then evaluate with `gait gate eval --approved-script-registry ...`. Invalid or tampered registry state fails closed in high-risk paths.

## How should teams start?

Run `gait tour` for a guided walkthrough, then `gait demo` to create and verify a signed pack. Wire one integration path from the integration checklist to move toward production enforcement.
