v1.1+ Roadmap for Gait

Roadmap principles
	1.	Preserve core contracts

	•	No breaking changes to v1 runpack, gate trace, policy test schemas within the v1 major line.
	•	Additive evolution only: new fields, new artifacts, new modules, new backends behind stable interfaces.

	2.	Stay on the execution path

	•	Prioritize features that increase enforcement, reproducibility, and operational trust.
	•	Avoid “dashboard-first” scope until the artifact standard is sticky.

	3.	Ship in rungs

	•	Each release must strengthen the ladder: Runpack → Regress → Gate → Coverage → Evidence → Enterprise Control Plane.

Current v1 baseline shipped (for roadmap context)

	•	Reserved v1.1+ interface packages already exist in core: Scout, Guard, MCP.
	•	Reserved v1.1+ schemas already ship in v1: inventory snapshot, evidence pack manifest, and registry pack metadata.
	•	CLI includes `gait run record`, `gait migrate`, and `--explain` on major commands.
	•	Major CLI commands accept flags both before and after positional arguments.

⸻

v1.1: Coverage and Pack Foundations

Primary objective
Make “what is protected vs unprotected” visible and measurable, and make evidence packaging real, without adding heavy enterprise dependencies.

Key capabilities
	1.	Scout v1 (Inventory and Coverage)

	•	Inventory providers for at least 2 agent frameworks plus MCP configs.
	•	Coverage metrics: discovered tools, gated tools, high-risk ungated tools, coverage percent.
	•	Deterministic snapshot diffs and drift reports.

	2.	Guard v1 (Evidence Packs, JSON-first)

	•	evidence_pack zip with pack_manifest.json as the canonical artifact.
	•	Includes: inventory snapshots, gate trace summaries, regress summaries, referenced runpack summaries.
	•	Offline pack verify with deterministic results.

	3.	Registry v1 (Local plus simple remote, signatures enforced)

	•	Remote registry read support with signature verification and allowlist controls.
	•	Metadata standard for policy packs and adapters (risk_class, use_case, compatibility, provenance).
	•	Caching and pinning behavior for reproducibility.

	4.	Basic minimization v1 (Reducer, not a full minimizer)

	•	“Reduce runpack” to smallest set of steps and references that still triggers a selected failure predicate.
	•	Deterministic reducer report and minimized runpack artifact.

Acceptance criteria
	•	A team can generate an inventory snapshot and coverage report in minutes.
	•	A security reviewer can verify an evidence pack offline and detect tampering.
	•	Remote policy packs install with signature verification and pinning.

⸻

v1.2: Enforcement Depth and Credential Safety

Primary objective
Make Gate suitable for higher-stakes production use through stronger intent normalization, approvals hygiene, and credential posture.

Key capabilities
	1.	Gate v1.2 (Richer policy model, still YAML)

	•	Policy rules for data classes and destination controls (export restrictions, egress constraints).
	•	Dataflow constraints using declared targets and provenance metadata (e.g., forbid passing external/tool-derived strings into high-risk egress tools without explicit approval).
	•	Rate and scope controls per tool and per identity.
	•	Policy simulation mode for rollout, including “what would have been blocked.”

	2.	Approval workflow hardening

	•	Approval token chains: who approved, what scope, what TTL, what reason code.
	•	Optional multi-party approval requirements for selected tool classes.
	•	Deterministic approval audit log artifact.

	3.	Credential broker interface (no provider lock-in)

	•	Define the broker contract for just-in-time credentials.
	•	v1.2 ships one reference broker implementation (local stub plus one real provider).
	•	Gate can require “broker-issued credential present” for allow on high-risk tools.

	4.	Replay with safe sandboxes (expanded)

	•	Additional stub fidelity: deterministic stubs for common tool types.
	•	Guardrails for “real tool replay” with explicit, per-tool allow flags and environment interlocks.

Acceptance criteria
	•	High-risk tools cannot execute without explicit policy allow and valid approval when required.
	•	Approval artifacts and gate traces are sufficient for incident review without guesswork.
	•	Credential brokering can be turned on for at least one real integration end-to-end.

⸻

v1.3: MCP Proxy Mode and Protocol-Adjacent Control

Primary objective
Become a standard enforcement point for tool-call protocols without turning into a protocol platform.

Key capabilities
	1.	MCP proxy enforcement (optional)

	•	Enforce Gate policies on MCP tool calls in proxy mode.
	•	Produce signed traces for MCP traffic compatible with existing gate trace schemas.
	•	Deterministic policy outcomes for MCP calls.

	2.	Adapter expansion (quality over quantity)

	•	Add 3 to 5 high-signal adapters that represent real enterprise usage.
	•	Each adapter must support runpack capture, gate enforcement, and regress fixture creation.

	3.	Exporters (optional)

	•	OTEL exporter and one log-based exporter, never required for correctness.
	•	Exporters must preserve artifact integrity and reference run_id.

Acceptance criteria
	•	A team can add MCP proxy mode without rewriting their agent code.
	•	Tool calls via MCP are gated and traced with the same evidence guarantees.

⸻

v1.4: Evidence Packs for Audits and Incident Response

Primary objective
Make Guard the default “audit packet generator” without needing a hosted UI.

Key capabilities
	1.	Guard v1.4 (Audit mapping and templates)

	•	Evidence pack templates by audit scenario (SOC 2 style, PCI style) as metadata and structure.
	•	Pack includes a control index and evidence pointers, still JSON-first.
	•	Optional PDF rendering as a convenience layer, not the source of truth.

	2.	Retention and encryption upgrades

	•	Encrypted local artifact store option with key management hooks.
	•	Retention policies for traces and packs, with deterministic deletion reports.

	3.	“One command incident kit”

	•	gait incident pack –from <run_id> –window  to bundle all related artifacts.
	•	Includes runpack, gate traces, regress results, policy digests, approvals chain.

Acceptance criteria
	•	A compliance lead can produce repeatable evidence with minimal human toil.
	•	An incident responder can reconstruct the full chain from one pack.

⸻

v1.5 

 “Gait Skills”

An installable skill set that teaches agents how to use the Gait CLI correctly, safely, and deterministically.

Design constraints:
	1.	No product logic in skills. Skills call gait and parse --json.
	2.	No standing permissions. Skills never embed credentials.
	3.	Safe by default. Skills use stub replay and policy test unless explicitly instructed otherwise.
	4.	Deterministic outputs. Skills depend on the stable schemas and exit code contract.

The minimal skill set to ship (v1.1)
	1.	Skill: “Capture a runpack”

	•	Purpose: wrap an agent run, generate run_id, runpack.zip, ticket footer, and verify.
	•	Value: instant “paste into ticket” loop.

	2.	Skill: “Turn an incident into a regression”

	•	Purpose: regress init from run_id, run deterministic graders, emit CI-friendly results.
	•	Value: prevents repeats, makes drift visible.

	3.	Skill: “Policy test and rollout”

	•	Purpose: run gait policy test on intent fixtures, simulate allow or block, emit reason codes.
	•	Value: unlocks security review and safe rollout.

These map exactly to our v1 ladder and do not expand scope.

Distribution note (v1.x):

- Homebrew tap publication is deferred until post-v1.5 contract freeze criteria are met (stable install artifacts, stable exit codes/schemas, and release integrity artifacts on every cut).


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

⸻

Roadmap gates (signals to pull work forward or push it back)
	1.	If Runpack adoption is high but Gate adoption is low

	•	Improve policy authoring UX and policy test workflows before expanding adapters.

	2.	If privacy concerns block runpack storage

	•	Pull forward encrypted stores, reference-only defaults, and stronger redaction tooling.

	3.	If teams demand fleet management early

	•	Pull forward registry and policy distribution, but keep artifacts as the core contract.

	4.	If MCP becomes the dominant tool boundary

	•	Pull forward MCP proxy mode and MCP-first adapters.

	5.	If incumbents bundle “good enough” gating

	•	Double down on verifiable artifacts, replay, diffs, and independent verification as the durable moat.
