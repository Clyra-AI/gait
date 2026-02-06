DO NOT IMPLEMENT YET!!!

## Epic A8: Enterprise Expansion Triggers (Post-Activation) 

Objective: define when to transition from pure self-serve to sales-assisted motion.

### Story A8.1: Product-qualified lead (PQL) thresholds

Tasks:
- Define PQL criteria:
  - >= N active repos
  - >= N high-risk gated tools
  - >= N weekly regress runs
  - adoption of approval/evidence artifacts
- Document handoff protocol for enterprise engagement.

Acceptance criteria:
- PQL rules are explicit and measured from product signals.

### Story A8.2: Paid boundary packaging

Tasks:
- Document free vs paid boundary:
  - OSS core: runpack/regress/gate basics/doctor
  - enterprise: policy distribution, governance workflows, compliance templates, fleet controls
- Keep boundary aligned with user value, not artificial limits.

Acceptance criteria:
- Pricing/packaging logic is coherent with actual adoption behavior.

### Story A8.3: Enterprise onboarding blueprint

Tasks:
- Add enterprise rollout blueprint:
  - pilot repo selection
  - policy governance owners
  - approval key ownership
  - evidence lifecycle

Acceptance criteria:
- Enterprise team can run a 30-day pilot with clear success criteria.

---

v2.0: Enterprise ACP Platform Expansion

Primary objective
Move from “tooling for artifacts” to “control plane infrastructure,” while keeping artifacts as the contract.

Key capabilities
	1.	Central policy and artifact registry (optional hosted, self-hostable)

	•	Multi-tenant registry for policy packs, adapters, graders, and signed releases.
	•	Organization RBAC for policy authors, approvers, and auditors.
	•	Fleet policy distribution to many environments.

	2.	Enterprise identity integration

	•	SSO and RBAC bindings to enterprise identity providers.
	•	Signed operator identities for approvals and policy changes.

	3.	Multi-language capture surfaces

	•	Additional SDKs beyond Python for intent capture and gate invocation.
	•	Maintain “decision in Go core” invariant.

	4.	Advanced minimization and replay determinism

	•	Smarter minimization with deterministic failure predicates and step selection.
	•	Differential replay to isolate root causes across model and prompt versions.

	5.	Commercial packaging options

	•	Enterprise support, policy registry, approval workflows, credential brokering integrations, long retention, compliance templates, and fleet management as the monetizable layer.

Acceptance criteria
	•	Enterprises can run Gait as a platform across multiple teams and environments with centralized control and distributed enforcement.
	•	Artifact verification remains possible offline and independently.


Reality check

If customers “do not want to host anything,” you only have two viable outcomes:
	1.	Vendor-hosted SaaS. You said no.
	2.	Customer-hosted deployment that feels “managed” (low ops, uses managed cloud services, upgrades are boring, uninstall is easy).

In enterprise security, the second is often preferred anyway. They want the control plane in their account and under their keys.

So your strategy should be: buyer-hosted, marketplace-billed, low-ops appliance.

Recommended tech and distribution strategy

1) Product packaging
	1.	Gait OSS Core (free, Vision 1 and 2 core)

	•	CLI + schemas + runpack, regress, gate, doctor.
	•	Distributed via GitHub releases + package managers.
	•	No licensing logic.

	2.	Gait Enterprise Add-on (paid, “Fleet” capabilities without SaaS)

	•	A Kubernetes add-on delivered as Helm and, where possible, a native managed add-on experience. AWS Marketplace explicitly supports fulfillment via Helm charts and Amazon EKS add-ons for container products.  ￼
	•	Runs inside the customer cluster, stores state in cloud-native managed services (object storage, KMS, logs) or remains mostly stateless where possible.

What “Enterprise” should mean in a no-host model
	•	Central policy distribution (GitOps-friendly)
	•	Artifact retention and encryption at rest
	•	Org RBAC for approvals and policy changes (implemented via the customer’s IdP integrations where possible)
	•	Fleet rollouts, posture reporting, and pack generation at scale
	•	Premium connectors and credential brokering

2) Licensing and monetization model

Your proposed model is viable:
	1.	Free core always

	•	Keep runpack verification, diff, stub replay, regress basics, and baseline gate free. That builds standardization and adoption.

	2.	Paid enterprise features unlocked by entitlement

	•	Annual subscription, plus optional usage-based for trials and smaller teams. AWS Marketplace supports annual pricing models and contract-style durations, and container offerings support contract pricing via AWS License Manager integration.  ￼

	3.	Trial mechanics

	•	For AWS Marketplace, container products can have free trials, including for EKS pods, with trial duration and included pod count set by the seller (buyer still pays infra).  ￼
	•	For Google Cloud Marketplace, free product trials are a supported concept.  ￼
	•	Azure Marketplace supports transactable container offers and metering models; you can structure “trial-like” entry via plans and billing approaches, but the exact implementation varies by offer type.  ￼

3) How to avoid refactors later

Design the enterprise add-on as additive services that consume v1 artifacts, not a replacement.

Non-negotiable architectural constraints
	1.	Artifact-first permanence

	•	Runpack and gate trace schemas remain the core contract forever. Enterprise services only index, retain, and package them.

	2.	Plug-in backends, not rewrites

	•	Storage backend interface: local FS (v1), S3/GCS/Blob (v1.1+), customer vaults later.
	•	KMS backend interface: AWS KMS, Azure Key Vault, GCP KMS.
	•	Identity backend interface: OIDC plus cloud-native identity.

	3.	Licensing is orthogonal

	•	License checks sit behind a single EntitlementProvider interface:
	•	Marketplace entitlement (when running in a cloud account that can call entitlement APIs)
	•	Offline signed license file (for airgapped or on-prem)
	•	Your product logic never branches on “AWS vs Azure vs GCP,” only on “entitlement provider available.”

Hyperscaler marketplace plan

AWS: your best first bet

Why
	•	Strong support for container products delivered via Helm and EKS add-ons.  ￼
	•	Clear patterns for hourly or pod-based metering and contract entitlements.  ￼
	•	Free trial support for container products exists.  ￼

Recommended AWS offer structure
	1.	OSS: GitHub + Homebrew
	2.	Paid: AWS Marketplace EKS add-on (preferred) plus Helm fulfillment (fallback)
	3.	Pricing: start with hourly or usage-based for trial motion, then push annual contract for enterprise procurement
	4.	Entitlement: AWS License Manager integration for contract pricing where you want clean entitlements.  ￼

Azure: “Kubernetes apps” and transactable offers
	•	Azure supports deploying Kubernetes applications from Azure Marketplace into AKS through the portal experience.  ￼
	•	Azure container offers support usage-based monthly billing plans or BYOL patterns depending on offer configuration.  ￼

Recommended Azure approach
	•	Ship the same Helm chart, wrapped as the Azure Marketplace Kubernetes application offer.
	•	Keep billing marketplace-driven, not your own.

GCP: Kubernetes apps via Cloud Marketplace
	•	Cloud Marketplace supports Kubernetes apps deployment flows and supports free trials for Marketplace products.  ￼

Recommended GCP approach
	•	Publish as a Kubernetes app; keep the artifact and licensing model identical.

The simplest “no-host enterprise” story that actually sells
	1.	“Runs in your cluster, under your keys.”
	2.	“Installed from your cloud marketplace, billed on your cloud bill.”
	3.	“No new SaaS vendor risk surface.”
	4.	“Artifacts remain verifiable offline, even if you uninstall us.”

AWS Marketplace explicitly positions container products as deployable across EKS and beyond, using Helm charts or EKS add-ons after subscription, which fits this story cleanly.  ￼

Execution sequencing
	1.	Nail OSS v1 adoption (runpack is the viral primitive).
	2.	Ship AWS Marketplace first (EKS add-on if possible).
	3.	Only then replicate to Azure and GCP using the same chart and entitlement interface.
	4.	Treat “Fleet” as paid, but keep it lightweight: policy distribution, artifact retention, approvals, posture reporting, pack generation.

One hard recommendation

Do not try to make “Fleet” a big always-on platform in v2. Make it a small appliance that makes enterprise procurement easy and risk acceptable. Marketplaces are the procurement wedge; artifacts are the trust wedge; Kubernetes is the delivery wedge.

If you want, I can translate this into a concrete v2 packaging spec: chart layout, components, required managed services per cloud, and the exact entitlement provider interface.