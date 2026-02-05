# Policy Test Example

This example demonstrates policy verdicts and exit codes.

Run from repo root:

```bash
gait policy test examples/policy-test/allow.yaml examples/policy-test/intent.json --json
gait policy test examples/policy-test/block.yaml examples/policy-test/intent.json --json
gait policy test examples/policy-test/require_approval.yaml examples/policy-test/intent.json --json
```

Expected behavior:

- `allow.yaml` returns exit code `0` and verdict `allow`.
- `block.yaml` returns exit code `3` and verdict `block`.
- `require_approval.yaml` returns exit code `4` and verdict `require_approval`.
