# Authorization Bundle Contract

`gait pack build --type authorization --from <authorization_bundle.json>` builds
a PackSpec authorization bundle that links a Gate decision trace to its related
authorization evidence.

Core pieces:

- payload schema:
  `schemas/v1/gate/authorization_bundle.schema.json`
- pack type: `authorization`
- verify path: `gait pack verify <pack.zip> --json`

Linked evidence can include:

- signed trace record
- approval audit
- credential evidence
- delegation audit
- context evidence
- outcome receipt
- freeze-window, kill-switch, and sandbox decision state

Verification behavior:

- pack manifest integrity is still owned by PackSpec verification
- the authorization payload adds its own linked-evidence digests
- tampered linked evidence is detected even if the outer manifest is rebuilt
- missing linked evidence is surfaced as a verification failure signal

Example:

```bash
gait pack build \
  --type authorization \
  --from examples/policy/authorization_bundle/authorization_bundle.json \
  --json

gait pack verify ./pack_<id>.zip --json
```
