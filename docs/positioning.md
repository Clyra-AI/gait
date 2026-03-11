# Positioning Guardrails

Use this file when writing website copy, docs, release notes, talks, or sales material.

## Core Message

Gait is an artifact-first policy-as-code runtime for production tool execution.

Runtime governance is often observational; Gait is execution-time decision and proof.

It combines:

- execution-boundary policy enforcement (`gate`)
- verifiable run artifacts (`runpack`)
- deterministic incident-to-regression workflows (`regress`)

## What To Claim

- Deterministic verification/diff/stub replay for the same artifacts.
- Offline-first core workflows.
- Default-safe evidence model (reference receipts by default).
- Stable artifact schemas and exit codes as integration contracts.
- Vendor-neutral adapter model across agent frameworks.
- Truthful onboarding surface: `gait init`, `gait check`, `gait capture`, and `gait regress add` are thin wrappers over shipped Go authority, not alternate policy engines.

## What Not To Claim

- "Autonomous AI safety solved" style guarantees.
- Prompt-layer filtering as a complete control model.
- Hosted governance dashboard capabilities in OSS core.
- Real-time fleet control plane features that are not shipped in OSS v1.

## Product Boundary Language

Prefer:

- "execution boundary"
- "verifiable receipts"
- "deterministic regressions"
- "incident to regression in one path"
- "camera vs gate: monitor plus enforce"

Avoid:

- "single pane of glass"
- "AI governance suite"
- "black-box risk scoring"

OSS vs Enterprise framing:

- OSS includes hardened runtime enforcement, deterministic artifacts, and local operability gates.
- Enterprise adds fleet-wide policy distribution, org workflows, and centralized governance controls.

## Adjacent Stack Language

- "Gait integrates with your existing identity, vault, gateway, and SIEM stack."
- "Guardrails scan content. Gait evaluates structured action intent and enforces policy before execution."
- "Gait produces the evidence your monitoring stack consumes."

## Adapter Neutrality Language

- Frame integrations as the same contract across frameworks.
- State explicitly that adapters do not bypass Gate.
- Avoid framework-specific semantics in messaging.
