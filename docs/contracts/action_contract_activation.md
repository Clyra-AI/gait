# Proposed Action Contract ingest and activation

Gait consumes one explicitly selected Wrkr `proposed_action_contract` artifact
at a time. A proposal is report-only evidence. It cannot grant authority,
create policy, approve an action, execute an effect, or claim an effect.

```text
gait contract validate --proposal proposal.json --json
gait contract activate --proposal proposal.json --selection fixture-manifest.json \
  --policy-digest sha256:<64 hex> --principal principal:owner \
  --authority-ref approval:owner --target target:deploy \
  --environment production --mode context_only --private-key gait-private.key \
  --valid-from 2026-07-19T00:00:00Z --lifecycle-out lifecycle.jsonl --json
gait contract verify --activation activated.json --proposal proposal.json \
  --public-key gait-public.key --json
gait contract consume proposal.json --selection fixture-manifest.json
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

When `--lifecycle-out` is supplied, it must point to an existing signed,
decision-ready lifecycle prefix authenticated by the same activation key. Gait
verifies that prefix before appending activation transitions and never treats
an unauthenticated decision-ready record as authority.

Adapters that execute after Gate should run
`gait action-contract lifecycle-verify --journal lifecycle.jsonl --public-key
trace.pub` before invoking the executor. This verifies the Gate-owned prefix
and requires both decision-ready and activated state.

The conformance consumer (`gait contract consume <artifact>`) emits a direct,
deterministic JSON receipt. `status: pass` means the selected bytes were
consumed; `self_attestation` is always `false`, and execution/effect claims
are always false. Semantic readiness is reported separately. Consume requires
the local current-selection manifest beside fixture artifacts (or an explicit
`--selection` path), and activation requires an explicit selection manifest.
Production activation requires an explicit Ed25519 private-key source. The
`--allow-development-signing` flag is test-only, requires `environment`
`development` or `test`, and records `development_signing: true`; such output
is rejected by normal verification. Verification always requires the actual
bound proposal bytes. Activation output refuses existing files and symlinks
by default; `--overwrite` explicitly replaces only an existing regular file.

## Pre-execution runtime readiness

Gait also exposes a deterministic runtime projection without executing a tool:

```bash
gait contract classify --proposal proposal.json --json
gait contract readiness --proposal proposal.json --policy-digest sha256:<64-hex> --trusted-validators gait-policy-validator --trusted-validator-key gait-policy-validator=validator.pub.b64 --evaluation-time 2026-07-19T12:00:00Z --json
gait contract explain --proposal proposal.json --json
```

Classification uses the released Wrkr action classes (`read`, `write`,
`deploy`, `delete`, `execute`, `egress`, `credential_access`, and `release`)
and keeps resource lifecycle actions separate from `risk_class`. Inference can
preserve or raise a supplied class, never lower it. Intended outcome and
observed effect are separate fields; classification does not claim execution.
The documented `--action` path accepts the strict versioned
`runtime-action.schema.json` artifact emitted as `classification.action`.
Heuristic raw input, when needed, uses the separate versioned
`runtime-classification-input.schema.json` with `--input`; these selectors
cannot be combined.

Readiness evaluates typed preconditions with statuses `satisfied`,
`unsatisfied`, `inconclusive`, and `not_required`. The CLI requires an explicit
UTC `--evaluation-time` and policy-bound validator public key(s); a freeform
validator name or signature string cannot authorize readiness. Evidence must
be verified, digest-bound, timestamped, fresh within its max age, and carry
non-empty evidence references. Wrkr declarations, judge/advisory, and
self-attested results do not satisfy required checks. Proof v0.7.0
digest-bound relationship refs and the correlation profile are retained on
signed lifecycle records. Callers should use the verified reducer, which
checks every record signature before reconstructing state without an event
store. Lifecycle records require proposal/activation/precondition refs where
their event kind requires them and reducer order is fail-closed.

## Released compatibility fixtures

The v1.4.0 compatibility pack is generated with:

```bash
go run ./scripts/action_contract_fixture_generator --check
```

It binds generated activation bytes to the exact Wrkr v1.14.0 proposal bytes,
records raw and canonical digests, IDs, revisions, schema versions, and
current-selection evidence in `activation-fixture-manifest.json`, and keeps the
fixture public key under `testdata/`. The deterministic seed is test-only; no
private key is committed or read by production/default activation. Generated
activations carry `development_signing: true` and are non-authoritative by
default. The pack is a compatibility/conformance fixture, not a production
approval or execution authority source.
# Lifecycle and advisory evidence

Activated Action Contracts can be paired with deterministic stateful chain
checks, signed advisory evaluate/verify reports, control lifecycle evidence,
and signed lifecycle receipts. Receipt promotion into regress is reference-
first and preserves quarantine/non-authoritative status. None of these
artifacts grants execution authority.

Current companion commands:

```text
gait action-contract advisory evaluate|verify
gait action-contract otel --lifecycle lifecycle.json --otel-out events.jsonl --trusted-key public-key --source-version v1.5.0
gait regress add --from lifecycle-receipt.json --trusted-key receipt-public-key --private-key runpack-private-key --verify-at RFC3339
```

The covered-action boundary can bind a verified proposal and signed activation
directly to `gait gate eval`:

```text
gait gate eval --action-contract proposal.json --activation activated.json --activation-public-key public-key --trusted-validators validator --trusted-validator-key validator=validator.pub.b64
```

`enforce_floor` and `required` activations fail closed unless signature,
proposal digest, runtime classification, and trusted-validator readiness all
verify. Caller-supplied context digests are compared with verified artifacts;
they never grant authority.

Stateful chain policy/state/candidate APIs are deterministic and pre-execution.
Contract-bound approval, delegation, and brokered JIT evidence preserve exact
Action Contract bindings while legacy unbound flows remain compatible. The
v1.5.0 released base is distinct from stop/revocation/invalidation/out-of-scope
synthetic control extensions, which remain quarantine-only and non-authoritative
until released.

Portable contract-bound approval/JIT and delegation fixtures are checked with:

```bash
go run ./scripts/action_contract_gate_fixture_generator --check
```

The signed corpus covers exact and expired approvals, a bound root and
tightened child delegation, and non-expansion failures across action, target,
environment, data, network, operation, target-count, depth, TTL, parent,
origin, and revocation boundaries. Its key is fixture-only; every artifact is
quarantined and non-authoritative.

## Authoritative release evidence

Release tags generate a separate release-owner bundle with
`go run ./scripts/action_contract_authoritative_bundle_generator`. The release
workflow resolves the peeled tag commit, derives a domain-separated Ed25519 key
from the immutable release identity, re-signs the selected activation, readiness claim, lifecycle records,
and typed execution/effect/containment/compensation evidence, then verifies and
uploads the compressed bundle. Its manifest sets `authoritative: true`,
`fixture_only: false`, `development_signing: false`, and `quarantine: false`,
and binds the release tag, peeled commit, public-key digest, artifact digests,
and every referenced schema. The checked-in fixture corpus remains separate and
must never be promoted by changing marker fields.
