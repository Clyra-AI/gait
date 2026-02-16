---
name: app-audit
description: Run an evidence-based audit for Gait OSS project across product, architecture, DX, security, and GTM readiness; output a strict go/no-go report with P0-P3 blockers and launch risk by technology, messaging, and expectations.
disable-model-invocation: true
---

# Gait OSS Audit

Execute this workflow when asked to perform app review, release readiness, architecture clarity, or public-launch audit for Gait.

## Scope

- Repository: `/Users/davidahmann/Projects/gait`
- Analyze current code/docs only; do not invent features/markets.
- Default mode is read-only unless user explicitly asks for fixes.

## Workflow

1. Build whiteboard mental model from onboarding to sustained use.
2. Map personas, JTBD, and MVP user-story coverage.
3. List required inputs/config/secrets/dependencies and setup friction.
4. Evaluate “aha” moments per primary user story.
5. Run technical validation on affected surfaces (build/lint/tests as needed).
6. Audit docs for integration clarity:
   - where Gait sits in runtime path
   - what is customer code vs Gait vs tool/provider
   - sync vs async behavior and failure handling
7. Compare stated product intent vs implemented behavior.
8. Assess security posture and fail-closed guarantees.
9. Assess market wedge sharpness for existing personas only.
10. Produce final go/no-go verdict with minimum blocker set.

## Non-Negotiables

- Evidence-first: every claim must cite command output or file path.
- Boundary-first: explicitly separate ownership and integration points.
- Incident-first: lead with failure scenarios and operational impact.
- No cosmetics as blockers.
- Distinguish facts vs inference.

## Command Anchors

- `gait doctor --json` to capture environment diagnostics in machine-readable form.
- `gait pack inspect <artifact.zip> --json` to inspect artifact envelope integrity and payload shape.
- `gait gate eval --policy <policy.yaml> --input <intent.json> --json` to validate fail-closed policy behavior.

## Severity

- P0: release blocker / high reputational risk
- P1: major launch risk
- P2: meaningful gap, non-blocking for project
- P3: polish

## Output Contract

- Section 1: End-to-end product model
- Section 2: Persona/story coverage map
- Section 3: Inputs/config friction table
- Section 4: Aha analysis
- Section 5: Technical audit + release readiness
- Section 6: Business/market fit assessment
- Section 7: Final verdict (go/no-go) + top 3 launch risks

Each section must include:
- Findings
- Evidence references
- Risk color (Green/Yellow/Red)
- Blockers (if any)
- Minimum fix set (only release-critical)
