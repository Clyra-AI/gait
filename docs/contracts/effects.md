---
title: "Effect Evidence and Contracts"
description: "Deterministic before/after effect snapshots and pure typed contract grading."
---

# Effect Evidence and Contracts

Gait can carry bounded before/after observations for Postgres, filesystem,
HTTP, and generic resource lifecycles. A snapshot is evidence, not an effect
executor: collectors provide selectors, digests, counts, identities, owners,
TTL observations, collector/capture metadata, redaction mode, completeness,
and enforcement status.

Snapshots use the versioned schema
`schemas/v1/effects/effect_snapshot.schema.json` and a Proof RFC 8785 JCS
`canonical_content_digest`. `verified` evidence is distinct from
`observed_only`; `partial` and `unknown` evidence cannot produce an
authoritative pass.

An `effect_contract` contains typed `expect`, `forbid`, and `invariant`
predicates over stable fields such as `before.count`, `after.digest`,
`after.owner`, and `after.state`. Grading is pure and deterministic:
`pass`, `fail`, or `inconclusive`, with stable reason codes and per-predicate
evaluations. No database, filesystem, HTTP, Docker, or other external effect
is executed by grading.

```bash
gait effects grade \
  --snapshot effect_snapshot.json \
  --contract effect_contract.json \
  --junit effects.junit.xml --json
```

When configured on a regression fixture, the effects grader is included in the
normal deterministic regress result and JUnit output. `inconclusive` maps to a
fail-closed regression grader result while preserving the semantic status in
the details. Redacted or reference-only evidence remains reviewable and never
claims an unobserved value.
