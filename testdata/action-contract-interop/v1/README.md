# Gait Action Contract interop fixtures

The nine `pac-*.json` files under `expected/` are exact released Wrkr v3 bytes
from the `v1.14.0` producer fixture pack. The local fixture manifest is the
current-selection evidence for these files and pins each byte SHA-256,
canonical content digest, contract/family/revision identity, producer, and
schema versions. Gait tests consume these committed bytes only; they never
resolve a sibling checkout or regenerate fixtures in place. Packet Markdown,
packet JSON, and per-scenario exporter manifests are intentionally out of scope
because activation and consumer conformance use only the raw artifacts.
