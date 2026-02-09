# GitHub Release Template

Use this structure for all tagged releases.

## Title

`vX.Y.Z - <short release theme>`

## Header

Gait is the offline-first Agent Control Plane for production tool actions.

## What Shipped

- `<feature 1>` with path references
- `<feature 2>` with path references
- `<feature 3>` with path references

## First 5 Minutes

```bash
curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/install.sh | bash
gait doctor --json
gait demo
gait verify run_demo
gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml
```

## Security Boundary Example

```bash
gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json
```

Expected:

- `verdict: block`
- `reason_codes: ["blocked_prompt_injection"]`

## Integrity Artifacts

- `checksums.txt`
- `checksums.txt.sig`
- `checksums.txt.pem`
- `checksums.txt.intoto.jsonl`
- `sbom.spdx.json`
- `provenance.json`

## Upgrade Notes

- Breaking changes: `<none | list>`
- Schema or exit-code compatibility notes: `<notes>`

## Docs

- `README.md`
- `docs/install.md`
- `docs/ci_regress_kit.md`
- `docs/ecosystem/awesome.md`
- `docs/launch/README.md`
