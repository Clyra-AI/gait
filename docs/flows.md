# Gait Flow Diagrams

This document is the canonical runtime flow reference for OSS v1.

## Actor and Plane Legend

Actor legend:

- `Agent Runtime`: your external agent framework/runtime.
- `Adapter`: wrapper/sidecar/middleware that calls Gait and enforces non-`allow` as non-executable.
- `gait CLI/Core`: local Gait command path and Go policy/artifact engine.
- `Operator`: human/platform engineer/approver.
- `CI`: pipeline that runs deterministic regress and policy checks.

Plane legend:

- Runtime plane: live tool-call decision boundary.
- Operator plane: local commands, demos, inspections.
- CI plane: fixture-driven regressions and release gates.

## 1) First-Win Flow (Install -> Demo -> Verify)

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant CLI as gait CLI
    participant Core as Go Core
    participant FS as Local Filesystem

    Dev->>CLI: install + run gait demo
    CLI->>Core: execute demo command
    Core->>FS: write runpack + manifest
    Core-->>CLI: run_id + ticket_footer + verify result
    CLI-->>Dev: first-win output
    Dev->>CLI: gait verify run_demo
    CLI->>Core: verify artifact signatures + hashes
    Core-->>CLI: deterministic verify result
    CLI-->>Dev: verify ok / explicit failure
```

What this flow is:

- Operator plane onboarding to generate and verify first artifacts.

What this flow is not:

- Not an external agent runtime integration flow.
- `Go Core` here is Gait internals, not "the agent."

Value: produces a portable artifact and verifiable ticket footer in minutes.

## 1b) Unified Job + Pack Flow (v2.4)

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant Job as gait job
    participant Pack as gait pack
    participant FS as Local Filesystem

    Dev->>Job: submit + checkpoint + approve + resume
    Job->>FS: append deterministic job state/events
    Dev->>Pack: pack build --type job --from <job_id>
    Pack->>FS: write pack_<id>.zip + pack_manifest.json
    Dev->>Pack: pack verify + pack inspect + pack diff
    Pack-->>Dev: deterministic verification and CI-safe diff
```

What this flow is:

- Operator plane durable execution/evidence lifecycle.

What this flow is not:

- Not the adapter-level runtime enforcement path for third-party agents.

Outcome: durable runtime control and portable evidence under one pack contract.

## 1c) Context Evidence Proof Flow (v2.5)

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant Record as gait run record
    participant Gate as gait gate eval
    participant Pack as gait pack inspect/diff
    participant Regress as gait regress run
    participant FS as Local Filesystem

    Dev->>Record: run record --context-envelope --context-evidence-mode required
    Record->>FS: write runpack + refs.context_set_digest
    Record->>FS: write context_envelope.json when receipts exist
    Dev->>Gate: gate eval (high-risk intent)
    Gate-->>Dev: allow/block with context reason codes + trace context_set_digest
    Dev->>Pack: pack inspect / pack diff
    Pack-->>Dev: deterministic context summary + drift classification
    Dev->>Regress: regress run
    Regress-->>Dev: context conformance grader result (semantic/runtime/none)
```

What this flow is:

- Operator + CI proofrail flow for context evidence integrity.

What this flow is not:

- Not a replacement for runtime adapter enforcement.

Outcome: context usage is deterministic, auditable, and release-gatable without weakening offline-first behavior.

## 2) Execution-Boundary Gate Flow

```mermaid
sequenceDiagram
    participant Agent as Agent Runtime
    participant Adapter as Wrapper/Sidecar
    participant CLI as gait gate eval
    participant Gate as Policy Engine
    participant Tool as Real Tool Executor

    Agent->>Adapter: tool call intent
    Adapter->>CLI: gait gate eval --policy --intent --json
    CLI->>Gate: validate intent + evaluate policy
    Gate-->>CLI: verdict + reason codes + trace
    CLI-->>Adapter: structured GateResult
    alt verdict == allow
        Adapter->>Tool: execute once
        Tool-->>Adapter: result
    else verdict != allow
        Adapter-->>Agent: block/approval-required/error
    end
```

What this flow is:

- The runtime plane chokepoint for real side-effect control.

What this flow is not:

- Not optional if you need fail-closed enforcement.

Rule: only wrapped paths may execute tools; non-`allow` verdicts never execute side effects.

## 3) Incident -> Regression -> CI Gate

```mermaid
sequenceDiagram
    participant Eng as Engineer
    participant CLI as gait CLI
    participant Repo as Repo Workspace
    participant CI as CI Pipeline

    Eng->>CLI: gait regress init --from <run>
    CLI->>Repo: write `gait.yaml` + `fixtures/<run>/runpack.zip`
    CLI->>Repo: write `regress_result.json` + optional `junit.xml`
    Eng->>CI: push fixture/config changes
    CI->>CLI: `gait regress run --json --junit`
    CLI-->>CI: deterministic pass/fail + exit code
    CI-->>Eng: green build or explicit drift failure
```

What this flow is:

- CI plane conversion of incidents into permanent gates.

What this flow is not:

- Not live runtime gating for current tool calls.

Outcome: one incident becomes a permanent deterministic regression check.

## 4) High-Risk Approval Flow

```mermaid
sequenceDiagram
    participant Operator as Approver
    participant CLI as gait CLI
    participant Gate as Gate Evaluator
    participant Broker as Credential Broker

    CLI->>Gate: evaluate high-risk intent
    Gate-->>CLI: require_approval
    Operator->>CLI: gait approve --intent-digest --policy-digest ...
    CLI-->>Operator: signed approval token
    Operator->>CLI: re-run gate with token chain
    CLI->>Broker: resolve required credentials
    Broker-->>CLI: scoped credential evidence
    CLI->>Gate: evaluate with token + credentials
    Gate-->>CLI: allow or block with trace + reason codes
```

What this flow is:

- Runtime plane human-in-the-loop approval path when policy returns `require_approval`.

What this flow is not:

- Not only a CI concept. CI may verify this path, but runtime execution is where approvals block/allow side effects.

Trigger summary:

- runtime: `gait gate eval` returns exit `4`/`require_approval` for matched high-risk rules.
- CI: same exit/verdict can block promotion pipelines until approved evidence path is present.

Outcome: high-risk actions require explicit, auditable approval and credential proof.

## 5) MCP Runtime Interception Service

```mermaid
sequenceDiagram
    participant Runtime as Agent Runtime
    participant Service as gait mcp serve
    participant Gate as Gate Evaluator
    participant FS as Local Filesystem

    Runtime->>Service: POST /v1/evaluate (adapter + call payload)
    Service->>Gate: decode + evaluate intent
    Gate-->>Service: verdict + reasons + trace metadata
    Service->>FS: emit signed trace (and optional runpack)
    Service-->>Runtime: deterministic JSON response (exit_code + verdict)
```

What this flow is:

- Runtime plane HTTP decision surface for agent runtimes that can call a local service.

What this flow is not:

- Not a full replacement for all CLI flows (`regress`, `doctor`, `pack inspect/diff`, etc).
- Not automatic tool enforcement: caller runtime must still block execution on non-`allow`.

Rule: default bind is loopback and non-`allow` outcomes remain non-executing at the caller.

Hardening note: for non-loopback service binds, configure `--auth-mode token --auth-token-env <VAR>`, bounded `--max-request-bytes`, strict verdict mode (`--http-verdict-status strict`), and retention caps (`--trace-max-*`, `--runpack-max-*`, `--session-max-*`).

Enforcement note: `POST /v1/evaluate` returns a decision payload only. The runtime that called the endpoint must still enforce `if verdict != allow: do not execute side effects`.

Transport endpoints:

- `POST /v1/evaluate` -> JSON response
- `POST /v1/evaluate/sse` -> `text/event-stream` response
- `POST /v1/evaluate/stream` -> `application/x-ndjson` response

## 6) Long-Running Session Checkpoint Chain

```mermaid
sequenceDiagram
    participant Runtime as Runtime/Adapter
    participant CLI as gait run session/*
    participant Journal as Session Journal (JSONL)
    participant Pack as Checkpoint Runpack
    participant Verify as gait verify session-chain

    Runtime->>CLI: run session start
    CLI->>Journal: append header
    loop tool-call decisions
      Runtime->>CLI: run session append
      CLI->>Journal: append event (sequence++)
    end
    Runtime->>CLI: run session checkpoint
    CLI->>Pack: emit deterministic runpack for new sequence range
    CLI->>Journal: append checkpoint record + chain update
    Runtime->>Verify: verify session-chain
    Verify-->>Runtime: linkage + per-checkpoint integrity status
```

What this flow is:

- Runtime/operational durability path for long-running sessions with incremental checkpoints.

What this flow is not:

- Not limited to only one special runtime mode; it is explicit checkpoint support for multi-step/multi-day execution.

Outcome: multi-day execution can be checkpointed incrementally and verified as a linked chain.

Operational note: use `gait run session compact --journal <path>` to prune already-checkpointed events while preserving chain verification.

## 7) Delegation Token Enforcement

```mermaid
sequenceDiagram
    participant Lead as Delegator Agent
    participant CLI as gait delegate mint / gate eval
    participant Runtime as Delegate Runtime
    participant Gate as Policy + Delegation Verifier

    Lead->>CLI: delegate mint (delegator, delegate, scope, ttl)
    CLI-->>Lead: signed delegation token
    Runtime->>CLI: gate eval (intent + delegation token)
    CLI->>Gate: evaluate policy + verify delegation token/signature
    Gate-->>CLI: allow/block + reason codes + delegation audit
    CLI-->>Runtime: deterministic verdict
```

What this flow is:

- Runtime plane constrained delegation using signed token evidence and scope bindings.

What this flow is not:

- Not a full IdP/OIDC token exchange system. Enterprise identity/token lifecycle remains external; Gait validates delegation evidence presented at evaluation time.

Rule: when policy requires delegation, missing/invalid delegation evidence remains non-executable (`block`).
