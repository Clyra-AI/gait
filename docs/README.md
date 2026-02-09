# Gait Documentation Map

This index defines documentation ownership so docs stay aligned with code and avoid overlap.

## Source Of Truth By Topic

- Product-first overview and first win: `README.md`
- Documentation entrypoint and ownership map: `docs/README.md`
- Public docs and marketing site source: `docs-site/`
- GitHub Pages deployment workflow: `.github/workflows/docs.yml`
- System architecture and component boundaries: `docs/architecture.md`
- End-to-end runtime and operational flows: `docs/flows.md`
- Core concepts and terminology: `docs/concepts/mental_model.md`
- Normative artifact and schema contracts: `docs/contracts/primitive_contract.md`
- Endpoint action taxonomy contract: `docs/contracts/endpoint_action_model.md`
- Skill provenance contract: `docs/contracts/skill_provenance.md`
- Integration steps and acceptance checklist: `docs/integration_checklist.md`
- Runtime defaults and repeated-flag reduction: `docs/project_defaults.md`
- Policy rollout stages: `docs/policy_rollout.md`
- Approval workflow and operations: `docs/approval_runbook.md`
- Incident-to-regression CI workflow: `docs/ci_regress_kit.md`
- Evidence templates: `docs/evidence_templates.md`
- Packaging boundary (OSS vs Enterprise): `docs/packaging.md`
- Installer and platform support: `docs/install.md`
- Homebrew tap publishing path: `docs/homebrew.md`
- Local UAT and functional testing plan: `docs/uat_functional_plan.md`

## Overlap Rules

- `README.md` explains why and how to start quickly. It should not duplicate full runbooks.
- `docs/concepts/mental_model.md` explains terms and mental model. It should not redefine normative contracts.
- `docs/contracts/*` are normative. If another doc conflicts, contracts win.
- `docs/architecture.md` and `docs/flows.md` are the canonical visual references.
- Operational procedures live in dedicated runbooks (`approval_runbook`, `policy_rollout`, `ci_regress_kit`).

## Current Alignment Notes

- CLI command surface in docs aligns with `cmd/gait/main.go` command groups.
- Contract docs align with schema packages under `core/schema/v1` and validators under `core/schema/validate`.
- Integration docs align with current adapter examples under `examples/integrations/*`.
- Installer docs align with `scripts/install.sh` support: Linux/macOS script path and Windows manual path.
- `docs-site` ingests docs markdown directly from this repository and renders static pages for `https://davidahmann.github.io/gait/`.

## Suggested Reading Order

1. `README.md`
2. `docs/README.md`
3. `docs/concepts/mental_model.md`
4. `docs/architecture.md`
5. `docs/flows.md`
6. `docs/contracts/primitive_contract.md`
7. `docs/integration_checklist.md`
8. `docs/policy_rollout.md`
9. `docs/approval_runbook.md`
10. `docs/homebrew.md`
