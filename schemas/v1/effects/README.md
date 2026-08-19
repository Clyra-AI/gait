# Effect evidence schemas

This directory defines the versioned effect evidence boundary:

- `effect_snapshot.schema.json` describes bounded before/after observations for
  Postgres, filesystem, HTTP, and generic resources.
- `effect_contract.schema.json` describes typed `expect`, `forbid`, and
  `invariant` predicates.
- `effect-grading-result.schema.json` describes deterministic pass/fail/
  inconclusive results and per-predicate evaluations.

Snapshots are reference-first and carry Proof RFC 8785 JCS content digests.
`verified`, `observed_only`, `partial`, and `unknown` enforcement states are
explicit; grading never executes an external effect. Schema changes are
additive within `1.0.0` and breaking changes require a major version.

An effect snapshot carries Ed25519 collector provenance. Authoritative grading
requires an externally configured trusted public key; the committed fixture
pack uses `fixture_test_only` provenance and is never a production authority.
