# Gait Documentation Map

This file defines where each topic lives so docs stay accurate and non-duplicative.

## Start Here

1. `README.md` for product overview and first win
2. `docs/concepts/mental_model.md` for terminology and execution model
3. `docs/architecture.md` for component boundaries
4. `docs/flows.md` for end-to-end runtime and ops sequences
5. `docs/contracts/primitive_contract.md` for normative behavior

## Core Product Docs

- Architecture: `docs/architecture.md`
- Runtime flows: `docs/flows.md`
- Install paths: `docs/install.md`
- Packaging boundary (OSS vs enterprise): `docs/packaging.md`
- Project defaults: `docs/project_defaults.md`
- UI localhost path: `docs/ui_localhost.md`

## Policy And Governance

- Authoring: `docs/policy_authoring.md`
- Rollout: `docs/policy_rollout.md`
- Approval operations: `docs/approval_runbook.md`
- External tool allowlist recipe: `docs/external_tool_registry_policy.md`

## Contracts And Compatibility

- Primitive contract: `docs/contracts/primitive_contract.md`
- ContextSpec v1 contract: `docs/contracts/contextspec_v1.md`
- PackSpec v1 contract: `docs/contracts/packspec_v1.md`
- PackSpec TCK: `docs/contracts/packspec_tck.md`
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
- CI regress runbook: `docs/ci_regress_kit.md`
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
- Content cadence plan (v2.6): `docs/launch/content_cadence_v2_6.md`
- Hero demo asset review (v2.6): `docs/launch/hero_demo_asset_review_v2_6.md`

## Ownership Rules

- `docs/contracts/*` are normative. If any other doc conflicts, contracts win.
- `README.md` is onboarding and positioning, not a full runbook dump.
- Ops procedures belong in runbooks (`approval_runbook`, `policy_rollout`, `ci_regress_kit`, hardening docs).
- Wiki (`docs/wiki/*`) is a convenience layer; `docs/*` remains authoritative.

## Tooling References

- Docs site source: `docs-site/`
- Docs deployment workflow: `.github/workflows/docs.yml`
- Wiki publish script: `scripts/publish_wiki.sh`
