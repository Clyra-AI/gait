---
title: "Threat Model"
description: "Baseline OSS threat model for Gait runtime boundaries, artifact integrity, policy enforcement, and key custody."
---

# Gait Threat Model

Status: baseline threat model for current OSS release line (`v2.7.x`).

This document defines what Gait protects, where trust boundaries are, and which controls are expected in production deployment.

## Security Objectives

- prevent unauthorized side effects by enforcing policy at the tool boundary
- make evidence tampering detectable with deterministic verification
- keep critical workflows offline-capable
- fail closed when policy/evidence requirements are not met

## Assets

- policy inputs and policy digests
- intent inputs and intent digests
- trace records and approval/delegation audits
- runpack/pack artifacts and manifests
- signing keys (trace, approval/delegation, release integrity)
- CI evidence artifacts (`regress_result.json`, `junit.xml`, verify reports)

## Trust Boundaries

1. Caller runtime boundary:
   - wrapper, sidecar, or MCP boundary that invokes `gait gate eval`
2. Artifact boundary:
   - zip artifacts consumed by `gait verify` and `gait pack verify`
3. Key boundary:
   - private key custody and rotation path
4. CI boundary:
   - workflow execution, artifact upload, and release integrity pipeline

## Threats And Controls

| Threat | Impact | Primary controls | Residual risk |
| --- | --- | --- | --- |
| Artifact tampering (manifest/file hash mismatch) | corrupted evidence accepted | `gait verify`, `gait pack verify`, digest + signature checks | accepting unverified artifacts in external tooling |
| Wrapper bypass (tool executes without gate check) | unauthorized side effects | fail-closed wrapper contract in integration boundary docs | integrator wiring errors outside Gait |
| Policy ambiguity or missing required context evidence | unsafe allow path | fail-closed policy posture, deterministic `block`/`require_approval` outcomes | misconfigured policies that are too permissive |
| Approval/delegation token misuse (scope, TTL, wrong digest) | improper authorization | digest binding, token verification, approval/delegation audit artifacts | weak operational approval process |
| Signing key compromise | forged traces/artifacts | key separation, rotation, secret-manager/HSM guidance | delayed compromise detection |
| Unsafe replay misuse | unintended real side effects | explicit unsafe replay interlocks and dedicated exit semantics | operator intentionally overriding safeguards |
| Release artifact tampering | compromised install path | signed checksums, SBOM, provenance, release verification steps | downstream consumers skipping verification |
| Sensitive data leakage via raw capture | privacy breach | reference capture default, explicit unsafe/raw flags | local operator misuse of raw mode |

## Operational Controls

- enforce at runtime: only `verdict=allow` executes side effects
- treat `block`, `require_approval`, and evaluation errors as non-executing outcomes
- separate key roles:
  - approval/delegation keys
  - trace signing keys
  - release signing keys
- rotate keys on schedule and on suspected compromise
- verify integrity artifacts before promoting releases

Operational references:

- `docs/approval_runbook.md`
- `docs/hardening/prime_time_runbook.md`
- `docs/hardening/release_checklist.md`
- `SECURITY.md`

## Out Of Scope

- hosted control-plane threat scenarios (OSS contract is offline-first local runtime)
- endpoint/business-logic authorization outside tool-boundary policy decisions
- secrets management implementation details for third-party infrastructure

## Validation Hooks

Recommended recurring checks:

```bash
make test-hardening-acceptance
make test-packspec-tck
bash scripts/test_ci_regress_template.sh
bash scripts/test_skill_supply_chain.sh
```
