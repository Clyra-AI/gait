# Hacker News Launch Package

Use this as a baseline for launch threads. Keep it factual and concrete.

## Candidate Titles

- Show HN: Gait - policy-as-code for production agent tool calls
- Show HN: Gait - deterministic incident-to-regress workflow for AI agents
- Launch HN: Gait - policy enforcement + verifiable receipts for agent actions

## Post Body Template

```
We are launching Gait, an offline-first CLI for policy-as-code at AI agent tool boundaries.

Core loop:
- bootstrap repo policy (`gait init`, `gait check`)
- enforce policy at tool-call boundary (`gait gate eval`)
- capture signed evidence
- turn incidents into deterministic CI regressions

It is intentionally artifact-first and vendor-neutral.
No hosted service required for core workflows.

Quick first win:
1) gait init --json
2) gait check --json
3) gait demo
4) gait verify run_demo --json
5) gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml

Repo: https://github.com/Clyra-AI/gait
```

## First Comment Template (important)

```
If helpful, here is the exact safety boundary example:

gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json

Expected:
- verdict: block
- reason_codes: ["blocked_prompt_injection"]

And here is the incident-to-regress path:

gait capture --from run_demo --json
gait regress add --from ./gait-out/capture.json --json
gait regress run --json --junit ./gait-out/junit.xml
```

## Rules During HN Window (first 12 hours)

- Answer with commands, outputs, and artifact paths; avoid marketing language.
- Prioritize questions about safety model, determinism, and integration friction.
- Capture objections into `docs/launch/faq_objections.md` updates after launch.
