---
title: "Compatibility Matrix"
description: "Public compatibility mapping for Gait CLI, PackSpec version, and verifier behavior."
---

# Compatibility Matrix

This matrix defines compatibility between producer and consumer surfaces.

## Version Semantics

- This page is the contract-level source for version compatibility.
- Evergreen operational docs should not carry release tags in titles.
- Internal rollout labels belong in changelog and plan documents, not evergreen compatibility copy.

## Version Matrix

| Gait CLI | PackSpec | `gait pack verify` behavior | Legacy runpack verify via `pack verify` |
| --- | --- | --- | --- |
| current `1.x` release line | 1.0.0 | verifies PackSpec v1 (`run`, `job`, `call`) with additive verifier hardening and context-aware diff metadata where applicable | supported |

| Action Contract producer/consumer | Artifact schema | Contract schema | Compatibility behavior |
| --- | --- | --- | --- |
| Wrkr `proposed_action_contract` `v1.14.0` -> Gait v1.4.0 | 1 | 3 | Gait validates one explicit report-only proposal; the committed activation compatibility pack covers valid scenarios with current-selection evidence; activations are separate signed artifacts and new revisions require reactivation |
| Gait consumer receipt | 1 | n/a | Direct deterministic JSON receipt; `self_attestation=false`, no execution/effect claim |
| Gait runtime classification/readiness/lifecycle | 1 | n/a | Additive pre-execution schemas; required readiness evidence is fail-closed, lifecycle records use Proof v0.6.1 relationship refs/correlation, and no surface claims execution/effect |

| Effect snapshot / contract / grade | 1.0.0 | 1.0.0 | Bounded before/after evidence with Proof JCS digests; pure `expect`/`forbid`/`invariant` grading is pass, fail, or inconclusive and never executes effects |

The activation compatibility pack under `testdata/action-contract-interop/v1/`
is generated from exact Wrkr proposal bytes by
`scripts/action_contract_fixture_generator --check`. Its six valid scenario
activations are marked `development_signing: true`, use the labeled fixture-only
seed documented by the manifest, and are rejected by default verification. The
three intentionally invalid Wrkr scenarios remain represented with explicit
non-activation reason codes. These fixtures prove byte/schema/signature
compatibility, not production authority or execution approval.

## Stability Guarantees

Within major `1.x` of PackSpec:
- additive fields are allowed
- unknown fields must be ignored by compatible readers
- required fields are not removed/renamed

Breaking changes require:
- major schema version bump
- explicit migration guidance
- compatibility-window policy documented in release notes

## Producer Guidance

If you are emitting PackSpec outside Gait runtime:
- implement RFC 8785 canonicalization for digest/signature inputs
- keep zip output deterministic
- validate outputs with `gait pack verify` in CI, and treat wrong-key signature failures as hard verification failures even in standard mode when a verify key is supplied

Reference kit: `docs/contracts/pack_producer_kit.md`

Action Contract details: `docs/contracts/action_contract_activation.md` and
`schemas/v1/action-contract/README.md`.
