# PLAN_ENT: Fleet Operator Platform (ENT v2)

Date: 2026-02-07
Source of truth: `product/PRD.md`, `product/ROADMAP.md`, `product/PLAN_v1.6.md`, current OSS implementation
Scope: enterprise add-on roadmap after OSS wedge maturity. This is a strategic execution plan, not immediate OSS scope.

---

## Thesis

Gait Enterprise is the fleet operator platform for agent actions.

It manages policy, safety, and reliability across many teams and agent stacks by combining three durable primitives:
- execution-boundary enforcement (`gate`)
- verifiable artifact standard (`runpack` plus `trace`)
- incident-to-regression safety loop (`regress`)

Enterprise value is additive control-plane infrastructure, not a replacement runtime.

---

## Product Position

### OSS v1.x (wedge)

OSS remains the trust engine:
- offline-first CLI
- deterministic artifacts and verification
- local enforcement and regressions
- vendor-neutral integration contract

### ENT v2 (platform)

Enterprise adds fleet operations:
- centralized policy distribution with RBAC
- fleet posture and compliance metrics
- retention, legal hold, and evidence workflows at scale
- incident and drift signal prioritization across repos and environments

---

## Non-Negotiable Constraints

- Artifact contract permanence: enterprise cannot redefine runpack/trace semantics.
- Offline verification continuity: enterprise outputs must remain independently verifiable.
- Additive architecture: enterprise consumes indexed artifacts; it does not become a required action executor.
- Vendor neutrality: first-class support for multi-model, multi-framework, multi-cloud environments.
- Buyer-hosted first: default deployment path is customer-controlled infrastructure.

---

## Why This Wins

Enterprises with agent proliferation need one operating model for:
- who can do what
- under what policy and approval
- with what proof
- and with what drift risk

The budget category is not "prompt monitoring." It is production action control and evidence operations for autonomous systems.

---

## Current Baseline (Observed)

Strong OSS foundation already present:
- gate/regress/runpack/trace command path in `cmd/gait/*`
- stable schemas and types for intent, result, trace, runpack in `core/schema/v1/*`
- CI and acceptance suites across OS matrix in `.github/workflows/ci.yml`
- adoption and hardening docs and scripts in `docs/` and `scripts/`

Enterprise gaps (expected):
- no multi-tenant control-plane service
- no org-level RBAC and policy distribution service
- no fleet-scale signal engine and incident clustering service
- no enterprise artifact registry and retention APIs
- no marketplace entitlement and enterprise deployment packaging

---

## Entry Gate (When to Start ENT v2 Build)

Proceed only when OSS wedge signals are strong:
- `>= 10` active repos using Gait weekly
- `>= 3` teams running `regress` in CI
- `>= 1` repeatable security/compliance review using runpack plus trace artifacts
- stable primitive contract for two consecutive minor releases

---

## v2 Architecture Model

### Plane 1: Execution Plane (distributed, OSS)

Runs in customer repos/runtimes:
- wrapper/sidecar emits intent
- gate evaluates locally
- runpack/trace artifacts generated locally
- regress runs locally and in CI

### Plane 2: Control Plane (enterprise add-on)

Runs buyer-hosted:
- policy lifecycle and rollout orchestration
- RBAC and approval governance
- posture analytics and incident prioritization
- artifact indexing and retention control

### Plane 3: Evidence Plane (shared contract)

Immutable record system:
- artifact registry/index over runpack/trace/regress outputs
- verification and provenance metadata
- compliance exports and audit bundles

---

## v2.0 Objectives

1. Centralize governance without breaking OSS autonomy.
2. Make fleet posture and drift legible and actionable.
3. Convert incident toil into deterministic prioritized operations.
4. Preserve vendor-neutral, buyer-hosted deployment posture.

---

## Epic E0: Enterprise Readiness and Design Locks

Objective: lock interfaces and boundaries before implementation.

### Story E0.1: Enterprise capability boundary spec

Tasks:
- Define exact OSS vs ENT boundary in one contract doc.
- Specify what data crosses execution plane to control plane.

Repo paths:
- `docs/enterprise/boundary.md`
- `CONTRIBUTING.md`

Acceptance criteria:
- No enterprise capability requires changing OSS artifact semantics.

### Story E0.2: Control-plane API contracts

Tasks:
- Define API contracts for:
  - policy distribution
  - approval governance
  - artifact indexing
  - posture queries
- Include versioning and backward-compatibility rules.

Repo paths:
- `docs/enterprise/apis/`
- `schemas/` (new enterprise API schemas if needed)

Acceptance criteria:
- APIs are explicit enough for independent client/server implementation.

### Story E0.3: Threat model and trust boundaries

Tasks:
- Add enterprise threat model for control plane and data flows.
- Define trust boundaries between execution and control planes.

Repo paths:
- `docs/enterprise/threat_model.md`

Acceptance criteria:
- Security review can identify trust assumptions and residual risks.

---

## Epic E1: Multi-Tenant Control Plane Foundation

Objective: create tenant-safe platform core.

### Story E1.1: Tenancy model and isolation

Tasks:
- Implement org/project/environment tenancy model.
- Define isolation guarantees for data and policy operations.

Acceptance criteria:
- Cross-tenant access is denied by design and verified in tests.

### Story E1.2: RBAC core

Tasks:
- Implement roles for:
  - policy author
  - approver
  - auditor
  - platform admin
- Enforce least-privilege permissions per role.

Acceptance criteria:
- Privilege tests cover deny and allow paths for every role.

### Story E1.3: Audit log foundation

Tasks:
- Add immutable audit events for policy changes, approvals, and retention actions.

Acceptance criteria:
- Every privileged operation emits a signed or tamper-evident audit event.

---

## Epic E2: Policy Distribution and Rollout Orchestration

Objective: manage policy at fleet scale without service disruption.

### Story E2.1: Policy registry and version graph

Tasks:
- Build central policy registry with immutable versions and digests.
- Support environment pinning and rollback.

Acceptance criteria:
- Teams can pin to known-good policy versions and rollback deterministically.

### Story E2.2: Staged rollout engine

Tasks:
- Support rollout stages:
  - observe
  - dry_run
  - enforce
- Add blast-radius controls and rollback triggers.

Acceptance criteria:
- Rollouts can target subsets of repos/environments safely.

### Story E2.3: Policy conformance status

Tasks:
- Report per-repo conformance against required policy baseline.

Acceptance criteria:
- Non-conforming repos are identifiable with remediation guidance.

---

## Epic E3: Identity, Approvals, and Governance

Objective: enterprise-grade accountability for privileged actions.

### Story E3.1: Enterprise identity integration

Tasks:
- Add OIDC-first identity integration for operator attribution.
- Map identity groups to RBAC roles.

Acceptance criteria:
- Approval and policy actions are attributable to enterprise identities.

### Story E3.2: Approval governance service

Tasks:
- Manage approval policies (scope, TTL, quorum, separation-of-duties).
- Validate approval chain policy centrally.

Acceptance criteria:
- High-risk actions require configured approval policy compliance.

### Story E3.3: Key and signing governance

Tasks:
- Add key lifecycle controls for enterprise signing and verification paths.
- Define rotation and revocation behavior.

Acceptance criteria:
- Key rotation does not break verification continuity.

---

## Epic E4: Fleet Posture and Coverage

Objective: provide clear fleet-level control posture.

### Story E4.1: Fleet coverage model

Tasks:
- Aggregate coverage metrics:
  - gated tool coverage
  - high-risk ungated paths
  - approval-required path health

Acceptance criteria:
- Leadership and security can view posture by org/project/environment.

### Story E4.2: Compliance posture views

Tasks:
- Build posture queries and exportable reports for audit/compliance workflows.

Acceptance criteria:
- Reports map directly to evidence and policy versions, not inferred summaries.

### Story E4.3: Drift and regression posture

Tasks:
- Track regress pass/fail rates and drift trends by team.

Acceptance criteria:
- Teams can identify recurring drift hotspots from fleet views.

---

## Epic E5: Fleet Signal Engine

Objective: reduce noise and prioritize what matters now.

### Story E5.1: Incident family deduplication

Tasks:
- Cluster events/runs by deterministic fingerprint.
- Surface one canonical incident family record.

Acceptance criteria:
- Duplicate incidents collapse into stable families with representative artifacts.

### Story E5.2: Drift attribution engine

Tasks:
- Attribute drift to deterministic causes:
  - policy change
  - tool output/schema change
  - reference set change
  - config/runtime change

Acceptance criteria:
- Top drift causes are explicit and actionable.

### Story E5.3: Priority scoring

Tasks:
- Rank by blast radius and privilege posture, not alert volume.
- Use explainable deterministic scoring inputs.

Acceptance criteria:
- Critical destructive/high-risk patterns outrank low-risk noisy changes.

### Story E5.4: Next-best-fix automation

Tasks:
- Auto-suggest:
  - policy fixture additions
  - regress fixture additions
  - rollback candidates

Acceptance criteria:
- Suggestions are reproducible and linked to artifacts.

---

## Epic E6: Artifact Registry and Evidence Operations

Objective: run evidence workflows at scale.

### Story E6.1: Artifact index and retrieval API

Tasks:
- Build index over runpack/trace/regress artifacts with digest integrity pointers.

Acceptance criteria:
- Artifacts are queryable by run_id, digest, policy version, and incident family.

### Story E6.2: Retention and legal hold

Tasks:
- Add retention classes, immutable hold rules, and deletion reports.

Acceptance criteria:
- Retention actions are policy-governed and auditable.

### Story E6.3: Compliance bundle automation

Tasks:
- Generate scenario-based compliance bundles from indexed artifacts.

Acceptance criteria:
- Compliance evidence can be produced without manual artifact hunting.

---

## Epic E7: Incident Automation and Workflow Integrations

Objective: integrate enterprise operations without locking runtime behavior.

### Story E7.1: Ticketing and SOAR integration adapters

Tasks:
- Add outbound integration adapters that attach canonical receipts and evidence references.

Acceptance criteria:
- Integrations consume standard artifact references, not custom payload contracts.

### Story E7.2: SIEM and log export connectors

Tasks:
- Export normalized posture and incident signals to SIEM pipelines.

Acceptance criteria:
- Export format is stable and documented.

### Story E7.3: Change-management gates

Tasks:
- Add deployment gate checks for policy conformance and regression posture.

Acceptance criteria:
- Risky rollouts can be blocked automatically by control-plane policy.

---

## Epic E8: Deployment and Commercial Packaging

Objective: deliver buyer-hosted enterprise with low procurement friction.

### Story E8.1: Kubernetes appliance packaging

Tasks:
- Package enterprise control plane as deployable Kubernetes stack (`Helm` first).

Acceptance criteria:
- Enterprise install is repeatable in customer-owned environments.

### Story E8.2: Entitlement architecture

Tasks:
- Support entitlement providers:
  - marketplace entitlement
  - offline signed license file
- Keep entitlement checks orthogonal to artifact semantics.

Acceptance criteria:
- Licensing does not alter runtime artifact compatibility.

### Story E8.3: Marketplace motion (AWS first)

Tasks:
- Publish deployment/procurement runbook for AWS marketplace first, then Azure/GCP.

Acceptance criteria:
- Procurement can complete without custom architecture branches.

---

## Epic E9: Reliability and Security Hardening for Control Plane

Objective: make enterprise platform operationally trustworthy.

### Story E9.1: Control-plane SLOs and runbooks

Tasks:
- Define SLOs for policy distribution, approval operations, and evidence query latency.
- Add incident runbooks.

Acceptance criteria:
- SLO breaches are measurable and actionable.

### Story E9.2: Disaster recovery and backup

Tasks:
- Add backup/restore strategy for control-plane state and artifact indexes.

Acceptance criteria:
- Recovery process is tested and documented.

### Story E9.3: Security hardening and external review

Tasks:
- Add platform penetration testing and security controls verification process.

Acceptance criteria:
- External review findings are tracked to closure before GA.

---

## Monetization Model

- Enterprise platform subscription by fleet scale and governance capabilities.
- Retention and evidence storage tiers for long-lived operations.
- Premium signal analytics and incident automation modules.

Pricing principle:
- charge for fleet governance and enterprise operations value.
- do not degrade OSS core verification and enforcement trust surface.

---

## Moat Statement

Durable moat is operational control at scale on top of the runpack and trace standard:
- execution chokepoint neutrality across suites
- independent verifiability of artifacts
- deterministic regression and incident signal loops

---

## Explicit Non-Goals (ENT v2)

- replacing OSS CLI with a hosted-only workflow
- proprietary artifact formats that break OSS verification
- dashboard-first development without action-level control outcomes

---

## v2 GA Exit Criteria

- Multi-tenant RBAC and policy distribution proven in production pilots.
- Fleet posture and signal engine reduce actionable queue size measurably.
- Compliance/evidence workflows operate from indexed artifacts with retention controls.
- Buyer-hosted deployment and entitlement paths are stable and supportable.
- OSS and enterprise artifact verification interoperability remains intact.
