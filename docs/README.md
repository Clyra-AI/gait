# Gait Documentation Map

This file defines where each topic lives so docs stay accurate and non-duplicative.

Primary category line:

- Gait is the offline-first policy-as-code runtime for AI agent tool calls.

Supporting promise line:

- Prove the install fast, enforce at the tool boundary when you own the seam, and graduate to hardened `oss-prod` readiness explicitly.

## Canonical Surface Taxonomy

Core surfaces:

- Jobs
- Packs
- Gate
- Regress
- Doctor

Extended first-class surfaces:

- Voice
- Context Evidence
- Action Contracts
- Effect Evidence and Contracts

## Start Here

1. `README.md` for the product wedge and first five commands
2. `docs/policy_authoring.md` for `.gait.yaml`, validation, and rollout workflow
3. `docs/integration_checklist.md` for the official LangChain lane, the OpenAI reference demo, and the reference-adapter promotion rules
4. `docs/agent_integration_boundary.md` for tool-boundary placement rules
5. `docs/install.md` for the truthful binary-install lane (`doctor`, `init`, `check`, `demo`, `verify`)
6. `docs/ci_regress_kit.md` for incident-to-CI adoption

## Core Product Docs

- Policy bootstrap and authoring: `docs/policy_authoring.md`
- Integration checklist: `docs/integration_checklist.md`
- Integration boundary: `docs/agent_integration_boundary.md`
- Architecture: `docs/architecture.md`
- Runtime flows: `docs/flows.md`
- Durable jobs lifecycle: `docs/durable_jobs.md`
- Voice mode boundary: `docs/voice_mode.md`
- Simple tool-boundary scenario: `docs/scenarios/simple_agent_tool_boundary.md`
- Install paths: `docs/install.md`
- Packaging boundary (OSS vs enterprise): `docs/packaging.md`
- Project defaults: `docs/project_defaults.md`
- UI localhost path: `docs/ui_localhost.md`
- Demo output legend: `docs/demo_output_legend.md`

## Policy And Governance

- Authoring: `docs/policy_authoring.md`
- Rollout: `docs/policy_rollout.md`
- Approval operations: `docs/approval_runbook.md`
- External tool allowlist recipe: `docs/external_tool_registry_policy.md`
- Managed/preloaded agent boundary: `docs/agent_integration_boundary.md`
- MCP capability matrix: `docs/mcp_capability_matrix.md`

## Contracts And Compatibility

- Primitive contract: `docs/contracts/primitive_contract.md`
- ContextSpec v1 contract: `docs/contracts/contextspec_v1.md`
- PackSpec v1 contract: `docs/contracts/packspec_v1.md`
- PackSpec TCK: `docs/contracts/packspec_tck.md`
- Pack producer kit: `docs/contracts/pack_producer_kit.md`
- PackSpec compatibility matrix: `docs/contracts/compatibility_matrix.md`
- Action Contract activation: `docs/contracts/action_contract_activation.md`
- Action Contract schemas: `schemas/v1/action-contract/README.md`
- Effect evidence and contracts: `docs/contracts/effects.md`
- Effect schemas: `schemas/v1/effects/README.md`
- Effect evidence and contracts: `docs/contracts/effects.md`
- Failure taxonomy and exit-code reference: `docs/failure_taxonomy_exit_codes.md`
- Artifact graph: `docs/contracts/artifact_graph.md`
- Intent+receipt conformance: `docs/contracts/intent_receipt_conformance.md`
- Endpoint action taxonomy: `docs/contracts/endpoint_action_model.md`
- Skill provenance: `docs/contracts/skill_provenance.md`
- UI contract: `docs/contracts/ui_contract.md`

## Operations And Hardening

- Hardening contract: `docs/hardening/v2_2_contract.md`
- Prime-time runbook: `docs/hardening/prime_time_runbook.md`
- Runtime SLOs: `docs/slo/runtime_slo.md`
- Retention profiles: `docs/slo/retention_profiles.md`
- Maintainer CI/runtime policy and local workflow guard: `CONTRIBUTING.md` (checkout, setup-go, setup-python, setup-node, and CodeQL action majors)
- CI regress runbook: `docs/ci_regress_kit.md`
- One-PR adoption page: `docs/adopt_in_one_pr.md`
- Threat model: `docs/threat_model.md`
- UAT plan: `docs/uat_functional_plan.md`
- Test cadence: `docs/test_cadence.md`
- Hardening release checklist: `docs/hardening/release_checklist.md`

## Adoption And Ecosystem

- Integration checklist: `docs/integration_checklist.md`
- SDK docs index: `docs/sdk/README.md`
- Python SDK contract: `docs/sdk/python.md`
- Deployment patterns: `docs/deployment/cloud_runtime_patterns.md`
- Zero-trust positioning: `docs/zero_trust_stack.md`
- Ecosystem index: `docs/ecosystem/awesome.md`
- Ecosystem contribution flow: `docs/ecosystem/contribute.md`
- Launch/distribution assets: `docs/launch/README.md`
- Activation KPI definition (v2.6): `docs/launch/activation_kpi_v2_6.md`
- Internal distribution planning artifact: `docs/PLAN_v2.7_distribution.md`
- Marketplace action publishing path: `docs/marketplace_action_publishing.md`
- Canonical MCP boundary demo: `docs/scenarios/mcp_canonical_boundary.md`
- Content cadence plan (v2.6): `docs/launch/content_cadence_v2_6.md`
- Hero GIF/script alignment decision log (v2.6): `docs/launch/hero_demo_asset_review_v2_6.md`

## Ownership Rules

- `docs/contracts/*` are normative. If any other doc conflicts, contracts win.
- `README.md` is the truthful onboarding surface for `gait init`, `gait check`, `gait demo`, and the first CI regression path.
- `docs/install.md` is the truthful binary-install surface for `gait doctor`, `gait init`, `gait check`, and the first signed artifact path after a prebuilt install.
- Ops procedures belong in runbooks (`approval_runbook`, `policy_rollout`, `ci_regress_kit`, hardening docs).
- Wiki (`docs/wiki/*`) is a convenience layer; `docs/*` remains authoritative.

## Tooling References

- Docs site source: `docs-site/`
- Docs deployment workflow: `.github/workflows/docs.yml`
- Local workflow runtime guard: `python3 scripts/check_github_action_runtime_versions.py .github/workflows docs/adopt_in_one_pr.md`
- Wiki publish script: `scripts/publish_wiki.sh`
