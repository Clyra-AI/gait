---
name: code-review
description: Perform Codex-style full-repository review for Gait (not PR-limited), with severity-ranked findings focused on regressions, fail-closed safety, determinism, portability, and docs/CLI contract correctness.
disable-model-invocation: true
---

# Full-Repo Code Review (Gait)

Execute this workflow for: "review the codebase", "audit repo health", "run a full code review", or "find risks in Gait."

## Reviewer Personality

- Contract-first: behavior and guarantees over style.
- Regression-first: look for latent breakage paths.
- Fail-closed safety bias: block safety/control weakening.
- Scenario-driven: each finding includes concrete break path and impact.
- Portability-aware: Linux/macOS/CI/toolchain/path behavior.
- Signal over noise: findings-first, severity-ranked output.

## Scope

- Repository root: `/Users/davidahmann/Projects/gait`
- Review entire repo, not only current diffs.
- Prioritize high-risk surfaces first, then remaining components.

## High-Risk Surfaces (Priority Order)

1. `core/gate`, `core/contextproof`, `core/pack`, `core/runpack`, `core/regress`, `core/jobruntime`
2. `cmd/gait` CLI behavior, flags, exit codes, JSON outputs
3. `core/mcp` and adapter boundaries
4. `sdk/python` wrapper behavior and error mapping
5. `schemas/v1` and compatibility-sensitive artifacts
6. `docs`, `README.md`, and `docs-site` command/contract accuracy

## Workflow

1. Build repository map and contract map from code/tests/help text.
2. Run baseline validation where feasible (lint/build/tests) and record gaps if not run.
3. Review each subsystem for:
   - Safety/control bypasses
   - Determinism or reproducibility breaks
   - Integrity verification weakening
   - False-green test/CI paths
   - Portability/toolchain/path assumptions
   - Schema/CLI contract drift
   - Docs/examples that do not match real behavior
4. Verify findings with concrete evidence (file refs, commands, test output).
5. Rank findings by severity and confidence.
6. Report minimum blocker set for safe release posture.

## Severity Model

- P0: release blocker, severe safety/integrity break, high reputational risk.
- P1: major behavioral regression or control bypass with real user impact.
- P2: meaningful correctness/portability/docs-contract issue.
- P3: minor maintainability concern.

## Finding Format

- `Severity`: P0/P1/P2/P3
- `Title`: short and action-oriented
- `Location`: file + line
- `Problem`: what is wrong
- `Break Scenario`: concrete failure path
- `Impact`: user/safety/CI/compliance effect
- `Fix Direction`: minimal safe correction

## Review Rules

- Findings are primary output; summaries stay brief.
- Do not report style nits unless they cause runtime/contract risk.
- Do not claim tests/commands were run if they were not.
- Separate facts from inference.
- If no findings, explicitly state `No material findings` and list residual risks/testing gaps.

## Command Anchors

- `gait doctor --json` to verify baseline runtime diagnostics and dependency posture.
- `gait gate eval --policy <policy.yaml> --input <intent.json> --json` to validate policy verdict/exit behavior.
- `gait pack verify <artifact.zip> --json` to check artifact integrity and signature status.

## Output Contract

1. `Findings` (required, ordered by severity)
2. `Subsystem Coverage` (Green/Yellow/Red per major area)
3. `Open Questions / Assumptions` (if any)
4. `Residual Risk / Testing Gaps`
5. `Final Judgment`:
   - technical health today
   - minimum blockers (if any)
   - top 3 risk concentrations
