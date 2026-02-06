PRD v1.0: Gait

Version: 1.0 (Focused v1, execution-ready)
Date: February 5, 2026
Owner: Product and Security
Status: Execution-ready PRD for build

⸻
	1.	One-paragraph summary

One-line promise
Gait makes production AI agents controllable and debuggable by default: it captures replayable runpacks, turns incidents into regressions, and enforces policy gates on tool calls.

Who it is for
Platform engineers, security engineers, compliance leads, and agent developers shipping agents that touch real tools, credentials, production data, or money.

The shareable artifact
A single run identifier and bundle teams paste into tickets and CI: run_id plus a signed runpack_<run_id>.zip that anyone can verify, diff, and replay in stub mode.

⸻
	2.	Problem and hair-on-fire triggers

Problem statement
Teams are deploying agents that act on production systems, but cannot reliably answer:
	•	What did the agent reference or retrieve?
	•	What did it do, exactly?
	•	What changed between runs?
	•	Why did it succeed or fail?
	•	Was the action allowed under policy?
	•	Can we prove it to security, audit, or incident review?

As “agents build agents,” humans stop reading all generated code. Trust must scale through artifacts, regression, and runtime enforcement, not manual review.

Hair-on-fire triggers (concrete incidents)

Incident A: Silent wrong outcome
	•	A documents intake agent “finishes” but misses a key field. No errors. Downstream workflow breaks days later. Engineering cannot reconstruct retrieval references, tool calls, or deltas.

Incident B: Catastrophic tool call
	•	A support agent triggers a tool call that deletes or overwrites production data. Post-incident, nobody can prove who requested it, what policy applied, or what safeguards failed.

Incident C: Data exposure via tool or retrieval injection
	•	An agent reads “trusted” internal content containing malicious instructions, then exfiltrates sensitive data through an allowed integration. Security needs a tamper-evident chain of what happened and what should have been blocked.

Why current alternatives fail

Prompt guardrails
	•	Validate text in and out. They do not reliably govern tool execution, and do not produce verifiable evidence artifacts.

API scanners and “AI security” detectors
	•	Detect risky prompts or content. They do not enforce action-level policy at the execution boundary, and they do not make incidents replayable.

Generic policy engines (OPA/Sentinel)
	•	Powerful, but heavy. They require specialized policy authoring, are not agent-native, and do not standardize run artifacts, regression fixtures, and proof-grade receipts for AI runs.

⸻
	3.	Personas and JTBD

Persona 1: Platform Engineer (owner of agent runtime)

JTBD
	•	When teams ship agents to production, I need a standard way to reproduce, test, and govern agent actions so the org can scale autonomy without scaling incidents.

Triggers
	•	First production incident with unclear root cause
	•	Leadership asks for a production readiness standard for agents

Adoption blockers
	•	Anything that requires cloud accounts, dashboards, complex infra, or multiple install paths

Persona 2: Security Engineer (approval gatekeeper)

JTBD
	•	When agents can touch privileged tools, I need enforceable boundaries and audit-grade proof of what ran so I can approve deployments without hand-waving.

Triggers
	•	Agent touches prod credentials, money, data export, destructive tools
	•	Compliance wants consistent evidence format

Adoption blockers
	•	Non-deterministic outputs, unclear error taxonomy, lack of fail-closed behavior, inability to verify artifacts offline

Persona 3: Compliance Lead (evidence owner)

JTBD
	•	When auditors ask how we control autonomous behavior, I need standardized, signed artifacts that can be generated repeatedly without manual screenshots.

Triggers
	•	SOC 2 / PCI / HIPAA reviews
	•	Repeated evidence collection toil

Adoption blockers
	•	Tools that only produce logs, not verifiable artifact packs, or that cannot be verified independently

Persona 4: Agent Developer (builder shipping workflows)

JTBD
	•	When my agent breaks or drifts, I need a fast way to capture a run, reproduce it, write a regression test, and avoid repeats without rewriting my stack.

Triggers
	•	“Works on my machine” agent behavior
	•	Model or prompt changes cause drift

Adoption blockers
	•	Requires rewriting agent framework, heavy integration, unclear first win, or noisy output that is not agent-friendly

Priority ranking (v1 focus)
	1.	Agent Developer and Platform Engineer first-win loop (Runpack and Regress)
	2.	Security Engineer runtime boundary (Gate)
	3.	Compliance Lead (enabled by verifiable runpacks and gate traces; full evidence packs in v1.1+)

⸻
	4.	User journeys

Journey A: First 5 minutes (viral first win, offline, <60 seconds)

Entry point
	•	README “Start here” with one path only

Steps
	1.	brew install gait or download single static binary (Go core)
	2.	gait demo

What gait demo does (offline, no keys, no Docker)
	•	Runs a local deterministic “toy agent” simulation with 3 tool calls (safe stubs)
	•	Produces:
	•	run_id
	•	runpack_<run_id>.zip in a local folder
	•	a one-line ticket footer string
	•	User can run gait verify and get deterministic results

Success criteria
	•	Demo completes in <60 seconds on a laptop
	•	Outputs are shareable: a terminal screenshot plus the bundle file
	•	gait verify <run_id> is deterministic and offline

Required outputs shown on screen
	•	run_id=run_
	•	bundle=./gait-out/runpack_<run_id>.zip
	•	ticket_footer=GAIT run_id=<run_id> manifest=sha256:<manifest_hash> (verify: gait verify <run_id>)

Journey B: First week (incident becomes solvable)

Entry point
	•	Agent incident or drift happens

Steps
	1.	Wrap agent execution with minimal recorder hook (Python wrapper or CLI sidecar)
	2.	Run agent once and capture run bundle: gait run record …
	3.	Attach run_id to incident ticket
	4.	Anyone reproduces locally: gait run replay <run_id> (stubbed tools default)
	5.	Compare to previous run: gait run diff <run_id_A> <run_id_B>
	6.	Convert incident to a test: gait regress init –from <run_id>

Artifacts generated
	•	Run bundle, deterministic diff outputs, fixture config, verification result

Success criteria
	•	“What happened” is answerable from artifacts without guesswork
	•	Repro steps are deterministic and executable by a second engineer

Journey C: First month (drift stops shipping)

Entry point
	•	Team adds agent reliability to CI

Steps
	1.	Convert a run bundle into a fixture: gait regress init –from <run_id>
	2.	Add deterministic graders (schema, diff rules, tolerances)
	3.	Run in CI: gait regress run
	4.	If it fails: gait regress report prints stable JSON and a short human summary

Artifacts generated
	•	regress_result.json, diff references, stable exit code, optional JUnit output

Success criteria
	•	Regress fails builds on meaningful drift deterministically
	•	Engineers can quickly see what changed and why

Journey D: Security boundary (gating high-risk actions)

Entry point
	•	Agent needs to execute privileged tool calls

Steps
	1.	Define a baseline policy file: gait policy init baseline-highrisk
	2.	Test policy against fixtures: gait policy test <policy.yaml> <intent_fixture.json>
	3.	Wrap tool calls via Python decorator (no decision logic in Python)
	4.	In production, enforce fail-closed for high-risk tools
	5.	For require_approval verdicts, provide signed approval token and proceed

Artifacts generated
	•	Gate trace record per action, signed verdict, policy version hash, approvals log

Success criteria
	•	Catastrophic actions are blocked deterministically
	•	Security can audit what was allowed, blocked, or approved
	•	Prompt injection cannot expand permissions: untrusted data in context cannot cause tool execution outside policy

⸻
	5.	Product scope (v1 only)

Core modules (v1)
	1.	Runpack (Replay)

	•	Capture a run into a signed, replayable bundle
	•	Verify bundle integrity offline
	•	Diff two runpacks deterministically
	•	Replay in stub mode deterministically (real tool execution requires explicit unsafe flags)

	2.	Regress (Regression from Runpacks)

	•	Convert runpacks into fixtures
	•	Run deterministic graders locally and in CI
	•	Produce stable machine-readable outputs and stable exit codes
	•	Support deterministic graders first; allow optional non-deterministic graders only as explicitly labeled plug-ins

	3.	Gate (Runtime Enforcement)

	•	Evaluate tool-call intent against YAML policy
	•	Verdicts: allow, block, dry_run, require_approval
	•	Signed verdict trace records
	•	Approval tokens (scoped, TTL, reason-coded)
	•	Design rule: Gate evaluates structured tool-call intents only (tool, args, targets, context). Prompts and retrieved content are never policy inputs.

	4.	Doctor (First 5 minutes reliability)

	•	Diagnose install, environment, permissions, and common failures
	•	Output stable JSON plus concise summary

	5.	Adapters (Minimal adoption surface)

	•	A stable Intent capture contract plus a thin Python wrapper that invokes Go locally
	•	One reference integration path shipped in v1 (keep it narrow and high-quality)

Explicitly deferred to v1.1+ (defined now to avoid refactors later)
	6.	Scout (Inventory and Coverage)

	•	Inventory scanning, coverage metrics, and snapshot diffs
	•	Deferred implementation, but interfaces and schemas reserved in v1

	7.	Guard (Evidence Packs)

	•	Evidence pack composition and PDF generation
	•	Deferred implementation, but pack schema and manifest structure reserved in v1

	8.	Registry (Policy packs and adapter registry)

	•	Remote registries, signatures, provenance, allowlists
	•	v1 supports local-only pack installation with signature verification hooks; remote backends added in v1.1+ without schema changes

	9.	MCP placement

	•	MCP remains an adapter surface, not the product center
	•	v1 defines MCP adapter interface and trace schema compatibility; proxy mode deferred to v1.1+

Explicit non-goals (v1)
	•	Agent orchestration or multi-agent planning
	•	Hosted dashboards as the primary product experience
	•	Model hosting, prompt IDEs, or agent UIs
	•	Required SIEM, ticketing, or directory integrations
	•	Replacing existing observability stacks (Gait emits artifacts and optional exports)

⸻
	6.	Functional requirements (FR)

FR-1: Single binary CLI
	•	Go-based static binary for macOS, Linux, Windows
	•	Deterministic verification, diff, and stub replay by default

FR-2: Offline demo and first win
	•	gait demo works offline, no keys, no Docker, <60 seconds
	•	Produces runpack + ticket footer line

FR-3: Run recording (Runpack)
	•	Record run metadata, tool intents, tool results, environment fingerprint, and reference receipts
	•	Default capture is reference-only (no raw sensitive content) unless explicitly enabled
	•	Store in runpack_<run_id>.zip with manifest.json

FR-4: Run verification
	•	gait verify <run_id|path> verifies hashes and signatures offline
	•	Deterministic verification result schema

FR-5: Run replay
	•	gait run replay <run_id> replays with tool stubs by default
	•	Explicit unsafe flags required to allow real tool execution

FR-6: Run diff
	•	gait run diff <run_id_A> <run_id_B> produces deterministic diff outputs
	•	Must support privacy-mode diffing (metadata-only diffs)

FR-7: Determinism contract (hard rule)
	•	Verification is deterministic
	•	Diff is deterministic given two artifacts
	•	Replay is deterministic in stub mode
	•	Recording is schema-stable and normalized, not byte-identical across separate executions

FR-8: Regression initialization
	•	gait regress init –from <run_id> creates fixture + config
	•	Creates local gait.yaml and fixtures/ layout

FR-9: Regression execution
	•	gait regress run executes graders and returns stable exit code contract
	•	Outputs regress_result.json and optional JUnit

FR-10: Grader framework
	•	Plugin interface for graders, deterministic graders first-class
	•	Optional non-deterministic graders must be explicitly labeled and require opt-in flags
	•	Non-deterministic graders must support pinning controls (model/version/config digest)

FR-11: Policy evaluation (Gate core)
	•	YAML policy evaluation in Go only
	•	Supports tool matching, arg constraints (hashes and structured fields), env constraints, rate and scope constraints
	•	Policy inputs are **structured intents only**. Prompts and retrieved content are never policy inputs and must not influence verdicts.
	•	For high-risk tools, intents must carry declared targets/destinations so policy can match on explicit side effects (not free-form text).
	•	Returns verdict + reason codes + violations list

FR-12: Policy test (security and rollout enabler)
	•	gait policy test <policy.yaml> <intent_fixture.json>
	•	Deterministic output with allow or block and reason codes
	•	Stable JSON plus concise summary

FR-13: Approval tokens
	•	gait approve produces signed approval tokens with scope, TTL, requester, reason
	•	Gate validates token before allowing execution

FR-14: Gate wrappers (Python thin layer)
	•	Python captures intent and calls Go core locally
	•	Python intent capture includes optional declared targets/destinations and lightweight provenance metadata (user-supplied vs tool output vs external content) when available
	•	Python must not contain policy decision logic
	•	Python returns GateResult and writes TraceRecord deterministically

FR-15: Trace records
	•	Every gate decision emits a signed trace record JSON
	•	Schema is OTEL-friendly and exporter-ready, but exporters are optional

FR-16: Reserved interfaces and schemas for v1.1+
	•	Scout: inventory provider interface and snapshot schema reserved
	•	Guard: evidence pack schema reserved
	•	Registry: pack metadata and signature hooks reserved
	•	MCP: adapter contract reserved
These are shipped as schema and interface definitions in v1 to avoid later rewrites.

FR-17: Doctor and diagnostics
	•	gait doctor identifies environment issues and prints actionable fixes
	•	Must emit stable JSON plus concise summary
	•	Must include copy-paste fix commands when safe

FR-18: Agent UX requirements
	•	All commands support:
	•	–json stable output
	•	stable exit codes
	•	–explain short command intent
	•	bounded summaries
	•	ticket footer one-liners
	•	failure taxonomy codes
	•	privacy-mode diffs
	•	case reports suitable for minimization later

⸻
	7.	Non-functional requirements (NFR)

NFR-1: Determinism
	•	Deterministic verification, diff, and stub replay
	•	Any non-deterministic mode is opt-in and explicitly labeled

NFR-2: Schema stability and compatibility
	•	All artifacts are versioned with semantic rules
	•	Backward-compatible readers for at least 2 minor versions
	•	Deprecation requires warning period and tooling for migration

NFR-3: Performance targets
	•	Local Gate evaluation overhead p95 <= 5 ms for typical policies
	•	Regress overhead scales linearly with fixtures and graders
	•	Diff and verify must complete fast on laptop-scale artifacts

NFR-4: Offline-first
	•	Core verification, diffing, stub replay, regress, and policy testing require no network

NFR-5: Fail-closed safety defaults
	•	In production mode, for configured high-risk tools, inability to evaluate policy blocks execution
	•	Dry-run mode supported for rollout

NFR-6: Security posture
	•	Signed releases, checksums, SBOM for Go binary
	•	Supply-chain hardening hooks for policy packs and adapters (signatures and provenance)
	•	No unauthenticated network services by default

NFR-7: Portability
	•	Single-binary operation for core
	•	Python layer optional, thin, and framework-agnostic

NFR-8: Observability and export
	•	Local artifacts are authoritative
	•	Optional exporters must never be required for correctness

NFR-9: Privacy controls (default-safe)
	•	Default recording is reference receipts only, not raw content
	•	Explicit redaction reports
	•	Clear warnings when enabling raw capture or storing sensitive payloads

NFR-10: Agent-friendly ergonomics
	•	Output is model-legible: stable keys, stable casing, stable error codes
	•	Short, bounded summaries and next commands guidance

⸻
	8.	Artifact and schema contracts

Canonical artifacts (v1)
	1.	Runpack (Replay)

	•	runpack_<run_id>.zip
	•	Contains:
	•	manifest.json (hashes, versions, signatures, capture_mode)
	•	run.json (run metadata, normalized timeline, environment fingerprint)
	•	intents.jsonl (tool intents, normalized)
	•	results.jsonl (tool results, normalized, with payload redaction rules applied)
	•	refs.json (reference receipts only by default; raw capture is explicit and flagged)
	•	diff.json (optional, generated by gait run diff)

Reference receipts (refs.json) semantics (v1 contract)
	•	Default: references only, not raw content
	•	Each reference receipt includes:
	•	ref_id, source_type, source_locator (opaque ID or path), query_digest, content_digest, retrieved_at, redaction_mode, sensitivity_label (optional), and retrieval_params (top_k, filters)
	•	Raw content capture, if enabled, must:
	•	set capture_mode=raw
	•	include explicit warnings in manifest
	•	support redaction and encryption hooks (v1 schema reserved; implementation can be minimal)

	2.	Regress Result

	•	regress_result.json
	•	Includes: fixture set, grader results, failure taxonomy codes, diff references
	•	Optional: junit.xml

	3.	Gate Trace Record

	•	trace_<trace_id>.json
	•	Includes: tool name, args hash, policy version hash, verdict, violations, latency, signature, approval_token_ref (optional)

	4.	Policy Test Result

	•	policy_test_result.json (or stdout JSON)
	•	Includes: intent fixture digest, policy digest, verdict, reason codes, violations

	5.	Ticket footer line (must be stable)

	•	Example format:
	•	GAIT run_id=<run_id> manifest=sha256: verify=“gait verify <run_id>”

Reserved artifacts for v1.1+ (schema shipped in v1, implementation deferred)
	•	inventory_snapshot.json (Scout)
	•	evidence_pack_.zip with pack_manifest.json (Guard)
	•	registry_pack.json metadata for signed pack distribution (Registry)

Stability rules
	•	All artifacts include: schema_id, schema_version, created_at, producer_version
	•	Breaking change requires new major schema version
	•	CLI provides gait migrate for artifacts when breaking changes occur

Exit codes (contract)
	•	0: success
	•	1: generic failure (unexpected)
	•	2: verification failed
	•	3: policy block (verdict=block)
	•	4: approval required (verdict=require_approval)
	•	5: regress failed (one or more graders failed)
	•	6: invalid input/schema
	•	7: doctor indicates non-fixable missing dependency
	•	8: unsafe operation attempted without explicit flag (safety interlock)

Failure taxonomy (contract)
	•	Short codes, stable, machine-readable, e.g.:
	•	E_POLICY_PARSE
	•	E_POLICY_NO_MATCH
	•	E_POLICY_TEST_FAIL
	•	E_APPROVAL_EXPIRED
	•	E_REPLAY_REAL_TOOL_DISABLED
	•	E_DIFF_NONDETERMINISTIC_INPUT
	•	E_REGRESS_GRADER_FAIL_SCHEMA
	•	E_VERIFY_HASH_MISMATCH

⸻
	9.	Acceptance criteria (v1)

First 5 minutes
	•	gait demo completes offline in <60 seconds
	•	Produces a runpack and ticket footer line
	•	gait verify <run_id> succeeds and is deterministic
	•	gait doctor runs and returns stable JSON

Runpack (Replay)
	•	Can record a run and replay with stubs deterministically
	•	Can diff two runpacks and produce stable diff outputs
	•	Privacy-mode diffing works (metadata-only)

Regress
	•	gait regress init creates a working fixture set
	•	gait regress run fails CI deterministically on drift using deterministic graders
	•	Exit codes match contract
	•	Outputs are stable JSON and include next actions

Gate
	•	Policies evaluate in Go only
	•	Python wrappers cannot change decisions
	•	Fail-closed behavior in production mode for configured high-risk tools
	•	Approval token flow works with TTL and scope

Policy test
	•	gait policy test produces deterministic results and reason codes
	•	Fixture digests match expected, policy digests match expected

Reserved v1.1+ schemas (no refactor commitment)
	•	v1 binary ships schema definitions for Scout, Guard, and Registry reserved artifacts
	•	No future module requires changing v1 runpack or gate trace schemas in breaking ways

Agent UX
	•	Every major command supports –json output and stable exit codes
	•	Bounded summaries do not exceed defined limits
	•	Ticket footer line is stable across versions within a major

⸻
	10.	Technology stack and architecture

Architecture overview
Hybrid by design:
	•	Go is authoritative core for policy evaluation, signing, verification, schema validation, diffs, stub replay, and CLI outputs.
	•	Python is a thin adoption layer that captures intent and invokes Go locally.

Design constraint: no rewrites later
To avoid refactors, v1 must ship:
	•	Stable artifact schemas with reserved fields for future modules
	•	Narrow, explicit module boundaries
	•	Plugin interfaces for graders, adapters, exporters, and registries
	•	Backends as interchangeable implementations behind stable interfaces

Go modules (v1)
	•	cmd/gait: CLI entrypoint
	•	core/schema: schema definitions, validation, versioning rules, reserved v1.1+ schemas
	•	core/sign: signing, verification, key handling, manifests
	•	core/runpack: record, verify, replay(stub), diff
	•	core/regress: fixtures, graders, reports, exit codes
	•	core/gate: policy evaluation, approval tokens, verdict generation, trace records
	•	core/policytest: fixtures, deterministic evaluation runner, reports
	•	core/adapters: adapter interfaces and one reference adapter
	•	core/doctor: diagnostics and fix suggestions
	•	core/registry: interface and local backend only (remote backends in v1.1+)
	•	core/export: exporter interface (OTEL/logs/etc.), no required exporters in v1

Reserved module packages (v1 ships interfaces and schemas; implementations in v1.1+)
	•	core/scout: interfaces + schema only
	•	core/guard: interfaces + schema only
	•	core/mcp: interface only (proxy enforcement in v1.1+)

Python SDK responsibilities and boundaries
	•	Provide decorators/wrappers for tool calls that:
	•	serialize an IntentRequest JSON
	•	call local Go core (subprocess or embedded)
	•	return GateResult JSON
	•	write TraceRecord JSON deterministically
	•	No policy logic, no policy parsing, no decision-making

Inter-process contracts (schemas)
	•	IntentRequest.json
	•	GateResult.json
	•	TraceRecord.json
	•	These are versioned and stable within major versions

Storage defaults
	•	Local filesystem under ~/.gait/ with explicit output directories
	•	No database required for v1
	•	Exporters and remote backends are optional plug-ins

Signing and key management
	•	Dev mode: ephemeral keypair generated locally with clear warnings
	•	Production mode: explicit key provisioning required
	•	Key rotation metadata supported in manifests
	•	All artifacts verifiable offline

⸻
	11.	Launch plan for OSS PLG (v1)

README conversion funnel requirements
	•	Top section: one-line promise + Start here
	•	One install path only
	•	One offline demo only
	•	One visible artifact only
	•	Then: Use it in your agent (smallest snippet)
	•	Then: Turn incidents into regressions
	•	Then: Gate high-risk tools
	•	Then: Policy test and rollout patterns
	•	Then: Extensibility (adapters, graders, exporters) as future-proofing, not a required step

Day-1 ecosystem repos (keep lean for v1)
	•	gait-adapters (one or two high-quality adapters, not many)
	•	gait-graders (deterministic starter graders only)
	•	gait-recipes (copy-paste workflows: incident runpack, CI regress, prod gating baseline)

Deferred to v1.1+ (do not build before v1 traction)
	•	awesome-gait-policies at large scale
	•	remote registries and signed pack distribution
	•	compliance pack PDFs and deep evidence mapping
	•	broad framework coverage

Security policy and disclosure readiness
	•	SECURITY.md with reporting channel
	•	Signed releases and checksums
	•	Supply-chain posture documented for adapters and graders

Post-launch operational plan
	•	Triage labels, issue templates, known-issues FAQ
	•	Rapid patch cadence for first 14 days
	•	Mandatory doctor improvements prioritized if onboarding friction appears

⸻
	12.	Risks and mitigations

Risk: Category confusion (Runpack vs Regress vs Gate)
Mitigation
	•	Market and README ladder is linear: demo, runpack, regress, gate
	•	Keep command names and help text aligned to the ladder

Risk: Too broad, becomes “AI SRE platform” prematurely
Mitigation
	•	Artifacts-first posture, no dashboards required
	•	Defer Scout and Guard implementations until runpack adoption is proven

Risk: Supply-chain attack via adapters, graders, future packs
Mitigation
	•	Signed releases and checksums now
	•	Registry interfaces and provenance hooks now
	•	Remote pack distribution deferred until verification posture is hardened

Risk: Non-determinism breaks trust
Mitigation
	•	Deterministic verification, diff, and stub replay are non-negotiable
	•	Non-deterministic graders are opt-in and explicitly labeled

Risk: “What the agent saw” becomes privacy-toxic
Mitigation
	•	Default to reference receipts only
	•	Raw capture is explicit opt-in with warnings and manifest flags
	•	Redaction mode is always recorded and auditable

Risk: Incumbents bundle “good enough” gating and eval
Mitigation
	•	Win by being the standard run artifact and fastest first win
	•	Make verification and replay universal, not tied to any vendor

What would change my mind (signals)
	•	If teams refuse to store runpacks for privacy reasons, double down on reference-only and redaction, and prioritize encrypted-at-rest local stores in v1.1+
	•	If regress adoption is weak, simplify graders to a minimal golden fixture model with one high-signal diff output
	•	If policy authoring is the blocker, prioritize policy test UX and baseline templates before adding more modules
	•	If MCP adoption stalls, keep MCP as adapter-only and do not invest in proxy mode until external pull appears

⸻

PRD completeness checklist

Product summary
	•	Included one-line promise, target users, shareable artifact

Problem and triggers
	•	Included 3 concrete incident scenarios
	•	Included why alternatives fail

Personas and JTBD
	•	Included 4 personas, JTBD, triggers, blockers, priority

User journeys
	•	Included first 5 minutes, first week, first month
	•	Included offline <60s demo
	•	Included paste ID into ticket workflow
	•	Included policy test journey

Scope
	•	Included Runpack, Regress, Gate, Doctor, minimal adapters
	•	Explicitly deferred Scout, Guard, Registry remote backends, MCP proxy
	•	Included explicit non-goals

Requirements
	•	FR list includes policy, approvals, dry-run, artifacts, signing, verification, replay, diffing, regress, policy test, CLI UX
	•	NFR list includes determinism contract, schema stability, performance, security, offline, portability, privacy, agent UX

Artifacts and contracts
	•	Defined canonical artifacts and reserved schemas for v1.1+
	•	Defined exit code contract and failure taxonomy
	•	Defined “what the agent saw” as reference receipts by default

Acceptance criteria
	•	Included local runnable acceptance tests and time-to-first-win constraints
	•	Included no-refactor commitment via reserved schemas and interfaces

Technology and architecture
	•	Defined Go modules, Python boundaries, inter-process schemas, storage, signing modes
	•	Defined interface-based backends for future expansion without rewrites

OSS PLG launch
	•	README funnel requirements
	•	Lean day-1 ecosystem repos
	•	Security readiness and post-launch cadence

Constraints honored
	•	Structured text only
	•	No tables
	•	No em dashes
	•	No clarifying questions
	•	Optimized for OSS PLG adoption, offline-first, default-safe behavior
