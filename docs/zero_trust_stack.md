# Gait In Your Zero Trust Stack

This document explains where Gait fits, what it integrates with, and what it intentionally does not replace.

## One-Line Positioning

Guardrails scan content. Gait evaluates structured action intent before execution and emits verifiable proof.

Short version: scanners and monitoring observe; Gait gates execution. You want both.

## Layer Model

1. Identity and credential systems (CyberArk, HashiCorp Vault, cloud IAM):
   - establish who can act and how credentials are issued
2. Guardrail and gateway systems:
   - scan prompts and outputs for injection/leakage patterns
3. Gait execution boundary:
   - evaluate `IntentRequest` against policy
   - fail closed for high-risk ambiguity
   - emit signed traces and runpack evidence
4. Monitoring and SIEM systems (Splunk, Datadog, Elastic):
   - ingest Gait artifacts/events for fleet analytics and response

## What Gait Owns

- deterministic policy decisions at tool-call boundary (`gait gate eval`)
- approval enforcement and signed approval audit records
- signed trace records and verifiable runpack artifacts
- deterministic incident-to-regression loop

## What Gait Does Not Own

- identity provider lifecycle
- credential vault lifecycle
- prompt/output scanning engines
- hosted SIEM/dashboard products

## Integration Patterns

### Identity And Auth Context Passthrough

`IntentRequest.context` supports additive optional fields for external identity context:

- `auth_context` (object)
- `credential_scopes` (array of strings)
- `environment_fingerprint` (string)

These are passthrough fields for policy and audit context. They do not change default OSS execution semantics.

### Tool Registry To Policy

Keep external registry ownership in your platform stack; convert allowlist exports into Gait policy:

- `docs/external_tool_registry_policy.md`

### SIEM Export

Use `gait mcp proxy` (one-shot) or `gait mcp serve` (long-running local service) and ingest JSONL/OTEL exports in existing observability stack:

- `docs/siem_ingestion_recipes.md`

## Runtime Governance vs ACP

Runtime governance (as commonly used) is often observational and alert-driven.

ACP in Gait is decision-and-proof at execution time:

- runtime governance asks: "is behavior acceptable?" and monitors
- ACP asks: "is this action allowed now?" and decides before side effects

Use this language consistently in docs and customer conversations.
