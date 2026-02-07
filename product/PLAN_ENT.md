DO NOT IMPLEMENT YET!!!

# PLAN_ENT: Enterprise Expansion Roadmap (Post-OSS)

## Purpose
Define a coherent enterprise roadmap that preserves the OSS contract and introduces paid capabilities as additive infrastructure, not a replacement runtime.

## Product Position
Gait enterprise is a buyer-hosted control-plane add-on for production agent governance:
- policy distribution and approvals
- evidence retention and compliance workflows
- fleet-level operations across many repos/environments

Gait OSS remains the adoption and trust wedge:
- runpack, regress, gate basics, doctor, offline verification

## Non-Negotiable Constraints
- Artifact-first permanence: runpack and gate trace artifacts remain the core contract.
- OSS continuity: offline verification must remain independently possible.
- Additive architecture: enterprise services consume and index artifacts; they do not redefine them.
- Buyer-hosted model: no required vendor-hosted SaaS.
- Licensing orthogonality: entitlement checks do not leak into artifact semantics.

## Current State Baseline (for planning only)
- Least privilege is partial: credential broker exists (`stub|env|command`) but is policy/flag driven, not globally mandatory.
- Tamper evidence is partial: signatures are supported but not universally required across all artifact paths.
- Integrations are CLI-first: no native managed IAM/ticketing/SIEM connector layer yet.
- Replay safety is stub-first: real-tool replay remains intentionally constrained.

## Stage A: Expansion Triggers (A8)
Objective: define when to move from pure self-serve to sales-assisted engagement.

### Story A8.1: PQL thresholds
Tasks:
- define measurable PQL criteria:
  - `>= N` active repos
  - `>= N` high-risk gated tools
  - `>= N` weekly regress runs
  - adoption of approval/evidence artifacts
- define handoff protocol from self-serve to enterprise motion

Acceptance criteria:
- PQL rules are explicit, queryable, and stable from product signals.
- Handoff path has owners, SLA, and qualification checklist.

### Story A8.2: Paid boundary packaging
Tasks:
- document OSS vs paid boundary:
  - OSS core: runpack/regress/gate basics/doctor
  - enterprise: policy distribution, governance workflows, compliance templates, fleet controls
- keep boundary value-aligned, not artificially restrictive

Acceptance criteria:
- packaging logic maps to real adoption pain points.
- no paid feature weakens offline artifact verification in OSS.

### Story A8.3: Enterprise onboarding blueprint
Tasks:
- define 30-day pilot blueprint:
  - pilot repo selection
  - policy governance ownership
  - approval key ownership
  - evidence lifecycle and retention policy

Acceptance criteria:
- pilot runbook can be executed by one platform team with clear success metrics.

## Stage Gate: Entry Criteria for v2.0
Proceed to v2.0 only when:
- A8.1 through A8.3 are complete.
- at least one repeatable enterprise pilot pattern exists.
- OSS artifact contract remains stable and trusted.

## v2.0 Objective
Move from single-repo tooling to enterprise control-plane infrastructure while keeping artifacts as the immutable contract.

## v2.0 Scope (Phased)

### Phase E1: Control Plane Foundation
Deliverables:
- centralized policy/artifact registry (buyer-hosted deployment)
- org/project tenancy model
- RBAC for policy authors, approvers, auditors
- fleet policy distribution model

Acceptance criteria:
- policies can be managed centrally and distributed across multiple repos/environments.
- artifact verification remains possible with offline CLI tooling.

### Phase E2: Identity and Approval Governance
Deliverables:
- enterprise identity integration (OIDC-first)
- signed operator identities for approvals/policy changes
- approval workflow primitives for high-risk actions

Acceptance criteria:
- approval provenance is attributable to enterprise identities.
- policy-change and approval audit trails are deterministic and exportable.

### Phase E3: Evidence, Retention, and Compliance Operations
Deliverables:
- retention policy engine for long-lived evidence management
- encryption/KMS backend abstraction
- compliance-ready evidence packaging templates

Acceptance criteria:
- evidence lifecycle can be centrally governed without changing artifact schemas.
- retention and key ownership can be mapped to enterprise controls.

### Phase E4: Fleet Operations and Integrations
Deliverables:
- fleet rollout orchestration and posture reporting
- prioritized connector set (policy-governed) for enterprise surfaces
- operational exports for enterprise audit/security workflows

Acceptance criteria:
- multiple teams can operate under centralized governance with distributed enforcement.
- posture and rollout state are reportable at fleet level.

### Phase E5: Packaging and Commercialization
Deliverables:
- enterprise add-on packaging as Kubernetes appliance (`Helm` first)
- entitlement provider interface:
  - marketplace entitlement
  - offline signed license file
- pricing plans aligned to enterprise procurement motion

Acceptance criteria:
- installation is low-ops in customer environment.
- entitlement model does not alter artifact compatibility or verification semantics.

## Deployment and GTM Model

### Product packaging
- `Gait OSS Core` (free): CLI + schemas + runpack/regress/gate basics/doctor.
- `Gait Enterprise Add-on` (paid): fleet governance features, delivered buyer-hosted.

### Commercial model
- free OSS core is permanent.
- paid enterprise capabilities are enabled via entitlement.
- trial path should exist where marketplace mechanics support it.

### Cloud sequence
1. AWS first (primary wedge): Kubernetes appliance + marketplace procurement path.
2. Azure second: same architecture and entitlement abstraction.
3. GCP third: same architecture and entitlement abstraction.

## Architectural Rules to Prevent Refactors
- Storage backend abstraction: local FS, object storage, future vaults.
- KMS backend abstraction: provider-agnostic key operations.
- Identity backend abstraction: OIDC-first, cloud-provider mapping behind interface.
- Product logic branches on capability interfaces, not cloud vendor names.

## Explicit Non-Goals (Current Plan)
- building a large always-on hosted SaaS control plane.
- replacing OSS artifact contracts with proprietary runtime formats.
- implementing all enterprise connectors in a single release.

## 30-Day Enterprise Pilot Blueprint (Target Shape)
- Week 1: scope and governance setup
  - pick pilot repos/workflows
  - assign policy and approval owners
- Week 2: enforcement and evidence setup
  - enable policy distribution
  - define approval and evidence lifecycle paths
- Week 3: operational hardening
  - run failure drills and replay/verification drills
  - validate reporting/retention behaviors
- Week 4: decision checkpoint
  - assess outcomes vs success criteria
  - decide expand/iterate/stop

Pilot success criteria:
- measurable reduction in uncontrolled high-risk actions
- reproducible evidence path for security/audit questions
- no regression in OSS verification guarantees

## Decision Summary
- Keep OSS as the trust and adoption engine.
- Make enterprise an additive, buyer-hosted control plane.
- Use marketplace procurement as distribution wedge, not as architecture driver.
- Keep artifacts and offline verification as permanent product contract.
