---
title: "Compatibility Matrix"
description: "Public compatibility mapping for Gait CLI, PackSpec version, and verifier behavior."
---

# Compatibility Matrix

This matrix defines compatibility between producer and consumer surfaces.

## Version Semantics

- This page is the contract-level source for version compatibility.
- Evergreen operational docs should not carry release tags in titles.
- Release-lane context belongs in release notes and plan documents (for example `docs/PLAN_v2.7_distribution.md`).

## Version Matrix

| Gait CLI | PackSpec | `gait pack verify` behavior | Legacy runpack verify via `pack verify` |
| --- | --- | --- | --- |
| v2.4.x | 1.0.0 | verifies PackSpec v1 (`run`, `job`) | supported |
| v2.5.x | 1.0.0 | verifies PackSpec v1 (`run`, `job`, `call`) | supported |
| v2.6.x | 1.0.0 | verifies PackSpec v1 (`run`, `job`, `call`) + context-aware diff metadata | supported |
| v2.7.x | 1.0.0 | same as v2.6 with CI/template hardening updates | supported |

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
- validate outputs with `gait pack verify` in CI

Reference kit: `docs/contracts/pack_producer_kit.md`
