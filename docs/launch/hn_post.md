# Hacker News Launch Package

Use this as a baseline for launch threads. Keep it factual and concrete.

## Candidate Titles

- Show HN: Gait - offline-first control plane for production agent tool calls
- Show HN: Gait - deterministic incident-to-regress workflow for AI agents
- Launch HN: Gait - policy enforcement + verifiable receipts for agent actions

## Post Body Template

```
We are launching Gait, an offline-first CLI that makes production AI agent actions controllable and debuggable by default.

Core loop:
- capture a run artifact (`runpack_<run_id>.zip`)
- verify/replay/diff deterministically
- enforce policy at tool-call boundary (`gate`)
- turn incidents into deterministic CI regressions (`regress`)

It is intentionally artifact-first and vendor-neutral.
No hosted service required for core workflows.

Quick first win:
1) gait doctor --json
2) gait demo
3) gait verify run_demo
4) gait regress bootstrap --from run_demo --json

Repo: https://github.com/davidahmann/gait
```

## First Comment Template (important)

```
If helpful, here is the exact safety boundary example:

gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json

Expected:
- verdict: block
- reason_codes: ["blocked_prompt_injection"]

And here is the incident-to-regress path:

gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml
```

## Rules During HN Window (first 12 hours)

- Answer with commands, outputs, and artifact paths; avoid marketing language.
- Prioritize questions about safety model, determinism, and integration friction.
- Capture objections into `docs/launch/faq_objections.md` updates after launch.
