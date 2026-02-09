# Skill Bundle Verification Example

This folder contains a minimal example registry manifest for a skill bundle.

Use `gait registry verify` with publisher allowlist and report output:

```bash
gait registry verify \
  --path examples/skills/registry_pack_example.json \
  --public-key <path-to-public-key> \
  --publisher-allowlist acme \
  --report-out ./gait-out/registry_verify_report.json \
  --json
```

Expected behavior:

- signature mismatch fails verification (`exit 5`) for unsigned/untrusted content
- report JSON is still written when `--report-out` is provided
