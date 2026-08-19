# Action Contract schemas

This directory defines the cross-product Action Contract boundary:

- Wrkr `proposed_action_contract` artifact schema `1`, with embedded contract schema `3`.
- Gait `activated_action_contract` artifact schema `1`.
- Gait consumer receipt schema `1`.
- Gait runtime action classification schema `1`, readiness schema `1`, and
  signed lifecycle-record schema `1`.
- Gait raw classification-input schema `1` is a separate heuristic input
  surface; `--action` accepts only the schema-validated runtime-action artifact.
- The lifecycle correlation field uses the versioned Proof control,
  containment, and telemetry correlation profile with digest-bound references.

Wrkr proposals are report-only evidence. Gait accepts one explicit proposal,
requires a Gait-owned current-selection manifest for activation/consumer
handoff, and emits a distinct signed activation artifact. Activation never
grants authority implicitly, creates policy, approves execution, or claims an
effect. New Wrkr revisions require a new explicit activation.

The canonical schemas are package-embedded for validation; the copies under
`core/actioncontract/schemaassets/` are tested byte-for-byte against this
directory. All digest/signature inputs use Proof RFC 8785 JCS primitives.

Compatibility is additive within these major schema versions. Unsupported,
malformed, duplicate-key, digest-mismatched, stale, superseded, or
unsupported-constraint inputs fail closed with stable reason codes.

The runtime projection is pre-execution only. `action_class`,
`composition_role`, data/trust/transition classes, intended outcome, and
resource lifecycle actions are deterministic and independent of `risk_class`.
`observed_effect` is optional evidence and is never synthesized from an
intended outcome. Readiness statuses are per-precondition and fail closed for
required missing, stale, inconclusive, self-attested, or non-policy-named
validators; authoritative readiness additionally requires an explicit UTC
evaluation time and a policy-bound public-key signature over the evidence
digest. The signed digest is the exported JCS digest of the normalized typed
precondition with verification metadata and derived status/reasons cleared;
mutating any semantic claim invalidates it. Boundary references remain
separate from evidence references.
