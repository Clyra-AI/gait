# GitHub Release Template

Use this structure for tagged releases.

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
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/install.sh | bash
gait doctor --json
gait demo
gait verify run_demo
gait regress init --from run_demo --json
gait regress run --json --junit ./gait-out/junit.xml
```

## Security Boundary Example

```bash
gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json
```

Expected:

- `verdict: block`
- `reason_codes: ["blocked_prompt_injection"]`

## v2.3 Release Gate Snapshot

Attach and summarize:

- `gait-out/v2_3_metrics_snapshot.json`
- `gait-out/integration_lane_scorecard.json`

Required release check:

- `release_gate_passed: true`
- `M1..M4`, `C1..C3`, `D1..D3` all pass

## Integrity Artifacts

- `checksums.txt`
- `checksums.txt.sig`
- `checksums.txt.pem`
- `checksums.txt.intoto.jsonl`
- `sbom.spdx.json`
- `provenance.json`
- `gait.rb` (Homebrew formula asset)

## Upgrade Notes

- breaking changes: `<none | list>`
- schema or exit-code compatibility notes: `<notes>`

## Docs

- `README.md`
- `docs/install.md`
- `docs/integration_checklist.md`
- `docs/contracts/intent_receipt_conformance.md`
- `docs/ci_regress_kit.md`
- `docs/ecosystem/awesome.md`
- `docs/launch/README.md`
