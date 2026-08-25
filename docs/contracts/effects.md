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
authoritative pass. Verified grading requires an externally supplied trusted collector public key matching the snapshot provenance signature; self-carried keys are not authority.

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
  --trusted-collector-key collector.pub \
  --expected-action-digest sha256:<64-hex> \
  --junit effects.junit.xml --json
```

Authoritative grading also requires a caller-supplied expected action,
activation, or Proof digest matching the signed snapshot correlation. Fixture
test provenance is available only to library/test fixture lanes and is never
accepted as production authority by the CLI.
When configured on a regression fixture, the effects grader is included in the
normal deterministic regress result and JUnit output. `inconclusive` maps to a
fail-closed regression grader result while preserving the semantic status in
the details. Redacted or reference-only evidence remains reviewable and never
claims an unobserved value.

Fixture metadata binds all three local paths explicitly:
`effect_snapshot`, `effect_contract`, and `effect_public_key`, plus at least
one caller-owned expected correlation digest:
`effect_expected_action_digest`, `effect_expected_activation_digest`, or
`effect_expected_proof_digest`. A missing or escaping trusted-key path or
expected digest is a failed grader input, never an implicit self-verification
fallback.

The committed `testdata/effects/v1` pack is labeled for the planned Gait
`v1.5.0` effects compatibility line. Its `fixture_test_only` key and golden
result are test evidence only and do not authorize collectors or effects. The
manifest binds the exact merged runtime classification result, signed lifecycle JSONL, and
released Gait v1.4.0 compensation activation bytes, so downstream consumers
can verify the complete proposal-to-activation-to-runtime-to-effect lineage.
Regenerate/check exact bytes with
`go run ./scripts/effects_fixture_generator --update` and
`go run ./scripts/effects_fixture_generator --check`.
## Local effect capture

`gait effects observe` records one bounded filesystem, HTTP, or generic
observation. `gait effects capture` combines fixed before/after observations
into a signed complete snapshot; one-shot observations remain partial and
observed-only. HTTP capture is offline-safe by default and requires explicit
unsafe-local opt-in for local targets.

Current commands:

```text
gait effects observe --resource filesystem|http|resource [--path path|--url url|--reference ref] --out observation.json [--observed-at RFC3339] [--allow-unsafe-local] [--json]
gait effects capture --resource filesystem|http|resource --before-observation before.json --after-observation after.json [--path path|--url url|--reference ref] --private-key key --out snapshot.json --action-digest sha256:<hex> [--json]
```

Paired capture produces the complete signed snapshot path. A single
observation remains partial and observed-only.
