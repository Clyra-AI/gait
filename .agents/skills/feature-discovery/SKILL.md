---
name: feature-discovery
description: Perform one-week, evidence-backed AI/agent market scanning and propose only high-leverage strategic product upgrades for Gait, with source-linked justification and no implementation.
disable-model-invocation: true
---

# Strategic Feature Discovery (Gait)

Execute this workflow when asked to identify strategic upgrades for Gait based on the latest AI and agent market developments.

## Scope

- Project context: `/Users/davidahmann/Projects/gait`
- Date anchor: determine the current local date/time at runtime before collecting sources.
- Time window: exactly the last 7 days from the run date (inclusive), using that runtime date anchor.
- Focus: AI/agent developments materially relevant to Gait’s durable runtime, policy enforcement, provenance, fail-closed execution, and enterprise adoption.
- Output target: `/Users/davidahmann/Projects/gait/product/ideas.md`
- This skill is strategy-only. No implementation tasks.

## Workflow

1. Resolve current local date/time and timezone, then compute the 7-day window.
2. Print the exact analysis window as absolute dates (`YYYY-MM-DD` to `YYYY-MM-DD`) before analysis.
3. Collect AI/agent developments published within that window from credible sources.
4. Filter to major market shifts only, excluding minor or incremental updates.
5. Map each market signal to Gait’s product wedge and moat.
6. Propose exactly 3 strategic upgrade features only when evidence is strong.
7. If evidence is weak or not material, output no recommendations.
8. Overwrite `product/ideas.md` from scratch with the final output.

## Strategic Filter (all required)

- Materiality: likely to impact market expectations in the next 6-18 months.
- Differentiation: improves moat; not parity-only busywork.
- Defensibility: strengthens trust, control, provenance, durability, or switching costs.
- Adoption leverage: improves enterprise buy confidence or expansion potential.
- Product relevance: can be framed as a durable product capability.

## Non-Negotiables

- Evidence-first: every recommendation must cite sources.
- Date discipline: reject sources outside the computed 7-day window.
- No minor suggestions, polish items, or local optimizations.
- No implementation details, tickets, sprint plans, or code steps.
- Clearly separate facts from inference.
- No tables in output.

## Command Anchors

- Validate current capability baselines before proposing upgrades:
  - `gait doctor --json`
  - `gait gate eval --policy <policy.yaml> --input <intent.json> --json`
  - `gait pack inspect <artifact.zip> --json`

## Source Quality Bar

- Prefer primary sources first:
- Official vendor/model announcements
- Official docs/specification updates
- Major platform release notes
- Regulator or standards-body publications
- Use secondary reporting only as corroboration, never as sole evidence.

## Output Contract

Write only a clean structured list to `product/ideas.md`:

1. `Recommendation`: one-line strategic feature name.
2. `Why Now`: market trigger and timing rationale.
3. `How (Strategic)`: product capability direction only (no implementation).
4. `Moat/Benefit`: defensibility and business impact.
5. `Evidence`: source links used for that recommendation.

Rules:
- Output exactly 3 recommendations when qualified evidence exists.
- If not enough material signals exist, write only:
  - `No worthy strategic upgrades identified in the last 7 days.`
  - `Evidence note:` short explanation with links reviewed.
- No preamble, no summary section, no implementation plan, no tables.
