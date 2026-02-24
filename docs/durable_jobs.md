---
title: "Durable Jobs"
description: "How Gait durable jobs work, where artifacts are emitted, and when this runtime-control model is preferable to checkpoint-only observability stacks."
---

# Durable Jobs

Use this page when your agent work can run for minutes to hours and you need deterministic lifecycle control with verifiable evidence.

## What A Durable Job Is

A durable job is a checkpointed execution record managed locally by Gait with explicit lifecycle commands:

- `submit`
- `status`
- `checkpoint add|list|show`
- `pause`
- `stop`
- `approve`
- `resume`
- `cancel`
- `inspect`

The job surface is for runtime control and evidence, not prompt orchestration.

## When To Use This

- multi-step agent workflows can fail mid-run and must resume deterministically
- human approvals are required before continuation
- operators need inspectable state transitions and stable stop reasons
- CI or incident workflows need portable evidence from job state

## When Not To Use This

- tasks are short-lived and retries are trivial
- no Gait CLI/artifact path is available in the runtime
- you only need hosted traces and dashboards without local enforcement or artifact verification

## Minimal Lifecycle

```bash
gait job submit --id job_1 --identity worker_1 --policy ./policy.yaml --json
gait job checkpoint add --id job_1 --type progress --summary "step 1 complete" --json
gait job pause --id job_1 --json
gait job stop --id job_1 --actor secops --json
gait job approve --id job_1 --actor reviewer_1 --reason "validated input" --json
gait job resume --id job_1 --actor worker_1 --reason "continue after approval" --policy ./policy.yaml --identity-revocations ./revoked_identities.txt --identity-validation-source revocation_list --json
gait job inspect --id job_1 --json
gait job status --id job_1 --json
```

`resume` is fail-closed for policy-bound jobs: if a paused job has a bound policy digest, you must provide current policy evaluation metadata (for example `--policy`) before continuation.

## Artifact And Verification Path

Durable jobs produce state under the job root (default `./gait-out/jobs`) and can be promoted to a pack:

```bash
gait pack build --type job --from job_1 --json
gait pack verify ./gait-out/pack_job_1.zip --json
gait pack inspect ./gait-out/pack_job_1.zip --json
```

Portable evidence outputs:

- job lifecycle state/events under `./gait-out/jobs`
- `pack_<id>.zip` (PackSpec v1 envelope)
- deterministic verify/inspect JSON for CI, incident handoff, and audits

## Emergency Stop Contract

`gait job stop` is an out-of-band emergency control. Once acknowledged:

- job status becomes `emergency_stopped`
- stop reason becomes `emergency_stopped`
- MCP proxy/serve paths block calls for that `job_id` with reason code `emergency_stop_preempted`
- blocked post-stop dispatches are journaled as `dispatch_blocked` events for offline proof

## How This Differs From Checkpoint/Observability Tools

| Dimension | Gait durable jobs | LangChain/LangFuse-style checkpoint and observability stacks |
| --- | --- | --- |
| Primary role | runtime control + evidence contract | orchestration tracing, hosted observability, debugging UX |
| Enforcement boundary | tool boundary with fail-closed non-execute rule | usually orchestration-time controls, not portable side-effect enforcement contract |
| Artifact portability | signed/offline-verifiable packs and traces | service-backed trace state, often not cryptographically portable by default |
| CI regression loop | first-class `regress` fixture and stable exit semantics | typically custom harnesses around exported traces |
| Offline operation | core verify/diff/regress/job operations run locally | hosted components commonly required for full feature set |

This is a complementary model: teams can keep hosted observability while using Gait for enforceable runtime boundaries and deterministic evidence.

## Better Fit Vs Not Necessary

Better fit:

- regulated or high-risk tool execution
- production incident-to-regression loops
- multi-team workflows requiring independently verifiable evidence artifacts

Not necessary:

- experimentation with no external side effects
- prototypes where deterministic replay and auditability are out of scope

## Integration Anchors

- CLI entrypoint: `cmd/gait/job.go`
- runtime implementation: `core/jobruntime/`
- pack conversion/verification: `core/pack/`
- representative adapter path: `examples/integrations/openai_agents/quickstart.py`
