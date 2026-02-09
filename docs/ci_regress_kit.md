# CI Regress Kit (Epic A5.1)

This kit makes incident-to-regression checks turnkey in CI.

Canonical default path:

- `.github/workflows/adoption-regress-template.yml`

## GitHub Actions Template

Use `.github/workflows/adoption-regress-template.yml` as the baseline workflow.

Template flow:

1. Build local `gait` binary.
2. Restore fixture (`fixtures/run_demo/runpack.zip` + `gait.yaml`) or bootstrap from a run artifact.
3. Run `gait regress run --json --junit=...`.
4. Fail with stable exit codes (`5` for deterministic regression failure).
5. Upload `regress_result.json`, `junit.xml`, and fixture artifacts.

## Generic Shell CI Snippet (Compatibility Only)

Use this in non-GitHub providers (Jenkins, Buildkite, CircleCI, etc.):

```bash
set -euo pipefail

go build -o ./gait ./cmd/gait
mkdir -p gait-out

if [[ ! -f fixtures/run_demo/runpack.zip || ! -f gait.yaml ]]; then
  ./gait demo
  ./gait regress bootstrap --from run_demo --json --junit=./gait-out/junit.xml
else
  ./gait regress run --json --junit=./gait-out/junit.xml > ./gait-out/regress_result.json
fi

set +e
if [[ ! -f ./gait-out/regress_result.json ]]; then
  ./gait regress run --json --junit=./gait-out/junit.xml > ./gait-out/regress_result.json
fi
status=$?
set -e

if [[ "$status" -eq 0 ]]; then
  echo "regress pass"
elif [[ "$status" -eq 5 ]]; then
  echo "regress fail (stable exit code 5)"
  exit 5
else
  echo "unexpected regress exit code: $status"
  exit "$status"
fi
```

Artifacts to retain:

- `gait-out/regress_result.json`
- `gait-out/junit.xml`
- `gait.yaml`
- `fixtures/`

## Recommended PR Gate

- Run regress on pull requests that touch policy, runpack, regress, gate, and fixtures.
- Require pass (`0`) for merge.
- Treat `5` as deterministic regression failure requiring either code fix or fixture update with explicit review.

Note:

- Keep this file as the only source for CI snippet variants.
- Prefer the workflow template above to avoid drift across docs.
