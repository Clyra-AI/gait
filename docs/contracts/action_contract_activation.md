# Proposed Action Contract ingest and activation

Gait consumes one explicitly selected Wrkr `proposed_action_contract` artifact
at a time. A proposal is report-only evidence. It cannot grant authority,
create policy, approve an action, execute an effect, or claim an effect.

```text
gait contract validate --proposal proposal.json --json
gait contract activate --proposal proposal.json \
  --policy-digest sha256:<64 hex> --principal principal:owner \
  --authority-ref approval:owner --target target:deploy \
  --environment production --mode context_only \
  --valid-from 2026-07-19T00:00:00Z --json
gait contract consume proposal.json
```

Validation checks the Wrkr envelope and v3 contract schema/producer, identity
and revision linkage, source/composition/evidence refs, `report_only` boundary,
supported typed constraints, and the Proof RFC 8785 JCS canonical content
digest. Stale, contradictory, superseded, malformed, unsupported, or digest
mismatched proposals receive stable `reason_codes` and never activate.

Activation creates a distinct immutable `activated_action_contract` artifact.
Its signed content binds the proposal artifact/digest/revision, Gait policy
digest, activating principal and authority references, target/environment,
activation mode (`context_only`, `enforce_floor`, or `required`), validity, and
explicit exceptions. A new Wrkr revision must be activated again.

The conformance consumer (`gait contract consume <artifact>`) emits a direct,
deterministic JSON receipt. `status: pass` means the selected bytes were
consumed; `self_attestation` is always `false`, and execution/effect claims
are always false. Semantic readiness is reported separately.
