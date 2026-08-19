# Gait Action Contract interop fixtures

The nine `pac-*.json` files under `expected/` are exact released bytes copied
from Wrkr Proof v0.5.0's
`scenarios/cross-product/action-contract-interop/expected/` pack. The source
fixture manifest is retained beside them and pins each byte SHA-256, canonical
content digest, contract/family/revision identity, producer, and schema
versions. Gait tests consume these committed bytes only; they never resolve a
sibling checkout or regenerate fixtures in place.
