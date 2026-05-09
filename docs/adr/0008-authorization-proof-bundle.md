# ADR 0008: Authorization Proof Bundle

- Status: Accepted
- Date: 2026-05-09
- Related epic: `GSC3.2`

## Context

Operators need one offline-verifiable artifact that links the Gate decision
trace to the approval, credential, delegation, context, sandbox, and outcome
evidence around the same action.

## Decision

Add an `authorization` PackSpec subtype:

- input payload schema under `schemas/v1/gate/authorization_bundle.schema.json`
- build via `gait pack build --type authorization --from <authorization_bundle.json>`
- verify through existing `gait pack verify`
- linked evidence digests live in the authorization payload
- final pack integrity remains owned by the outer PackSpec manifest and
  signatures

## Alternatives Considered

1. Separate non-PackSpec proof artifact.
   - Rejected: duplicates packaging and verification logic.
2. Embed the final pack manifest digest inside the inner authorization payload.
   - Rejected: self-referential and unstable once the payload itself is part of
     the packed contents.

## Consequences

- One named artifact can link authorization evidence for ticket-ready proof.
- Offline verification reuses existing pack tooling.
- Linked evidence tamper can be detected independently from outer pack rebuilds.
