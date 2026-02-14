# Gait Architecture

This document is the canonical architecture view for OSS v1.

## Integration-First Architecture

```mermaid
flowchart LR
    agent["Agent Runtime\n(OpenAI Agents / LangChain / Autogen / etc)"] --> adapter["Adapter / Wrapper / Sidecar\n(normalize tool call)"]
    adapter --> gatecli["Gait Decision Surface\n(gait gate eval OR gait mcp serve)"]
    gatecli --> policy["Go Gate Engine\n(policy + approval/delegation checks)"]
    policy --> decision["GateResult\nallow | block | require_approval | dry_run"]
    decision --> adapter
    adapter -->|allow only| tool["Real Tool Executor"]

    gatecli --> trace["Signed Trace\ntrace_*.json"]
    gatecli --> runpack["Runpack / Pack\nrunpack_*.zip | pack_*.zip"]
    runpack --> regress["Regress Fixtures\nfixtures/* + gait.yaml"]
```

What this is:

- The runtime integration boundary for tool-call control.
- The path that determines whether side effects execute.

What this is not:

- Not an orchestrator diagram for LLM planning/state machines.
- Not a hosted control plane architecture.
- Not evidence that Go Core is "the agent". The agent runtime is external; Gait is the control/evidence layer.

## Component Architecture (Implementation Internals)

```mermaid
flowchart LR
    operator["Developer / Platform Engineer / CI"] --> cli["gait CLI (cmd/gait)"]

    subgraph core["Go Core (core/*)"]
        dispatch["Command dispatch + exit contracts"]
        runpackc["Runpack (core/runpack)"]
        gate["Gate + policy eval (core/gate)"]
        regressc["Regress (core/regress)"]
        doctor["Doctor (core/doctor)"]
        scout["Scout signal + snapshots (core/scout)"]
        guard["Guard/evidence (core/guard)"]
        sign["JCS + signing (core/jcs, core/sign)"]
        schema["Schema validation (core/schema)"]
        fsx["Deterministic FS utilities (core/fsx, core/zipx)"]
    end

    cli --> dispatch
    dispatch --> runpackc
    dispatch --> gate
    dispatch --> regressc
    dispatch --> doctor
    dispatch --> scout
    dispatch --> guard
    runpackc --> sign
    gate --> sign
    regressc --> runpackc
    doctor --> schema
    dispatch --> schema
    dispatch --> fsx

    subgraph adapters["Adoption Layer (non-authoritative)"]
        py["Python SDK (sdk/python)"]
        sidecar["Sidecar patterns (examples/sidecar)"]
        framework["Framework adapters (examples/integrations/*)"]
        mcpserve["Local MCP interception service (gait mcp serve)"]
    end

    py --> cli
    sidecar --> cli
    framework --> py
    mcpserve --> gate

    subgraph artifacts["Artifact Surface (durable contract)"]
        runpackZip["gait-out/runpack_{run_id}.zip"]
        sessionChain["gait-out/sessions/*_chain.json"]
        traceJson["trace_{id}.json"]
        delegationAudit["delegation_audit_{id}.json"]
        regressJson["regress_result.json"]
        junitXml["junit.xml"]
        evidenceZip["Evidence packs (gait guard/incident pack)"]
    end

    runpackc --> runpackZip
    runpackc --> sessionChain
    gate --> traceJson
    gate --> delegationAudit
    regressc --> regressJson
    regressc --> junitXml
    guard --> evidenceZip
```

## Runtime Boundaries

- **Authoritative boundary**: Go core owns policy decisions, canonicalization, signing, schema validation, determinism, and exit codes.
- **Adoption boundary**: Python and sidecars should only serialize intent, call CLI, and interpret structured results.
- **Durable contract boundary**: artifacts and schemas are the long-lived API surface, not in-memory implementation details.

## State And Persistence

- Working artifacts: `./gait-out/`
- Session journals and chains: `./gait-out/sessions/*` (append-only journal + checkpoint chain)
- Session hot-path state index: `*.journal.jsonl.state.json` (lock-protected append state cache)
- MCP serve runtime traces: `./gait-out/mcp-serve/traces`
- MCP serve retention/rotation applies at trace/runpack/session directory boundaries when configured
- Regress fixtures/config: `fixtures/` and `gait.yaml`
- Optional local caches: `~/.gait/runpacks`, `~/.gait/registry`
- Schema contracts: `schemas/v1/*` with matching Go types/validators under `core/schema/*`

## Failure Posture

- Default-safe behavior is enforced in command handlers: non-`allow` gate outcomes do not execute side effects.
- High-risk production profile (`oss-prod`) remains fail-closed when policy or credentials cannot be evaluated.
- Delegation-constrained policies remain fail-closed when required delegation evidence is absent/invalid.
- Verification and regression failures return stable non-zero exit codes for automation.
