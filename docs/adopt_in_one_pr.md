---
title: "Adopt In One PR"
description: "Canonical one-PR workflow: emit a pack, verify it, and fail CI with stable exit code 5 on forced regression drift."
---

# Adopt In One PR

This is the canonical GitHub Actions adoption loop for Gait.

It does three things in one workflow:

1. emits a PackSpec artifact
2. verifies artifact integrity
3. fails the PR on a forced deterministic regression (`exit 5`)

Copy this file into `.github/workflows/gait-adopt-one-pr.yml`:

```yaml
name: gait-adopt-one-pr

on:
  pull_request:

permissions:
  contents: read

jobs:
  regress-gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install gait
        shell: bash
        run: |
          curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/install.sh | bash
          echo "${HOME}/.local/bin" >> "$GITHUB_PATH"

      - name: Emit and verify pack
        shell: bash
        run: |
          mkdir -p ./gait-out/adopt_one_pr
          gait demo --json > ./gait-out/adopt_one_pr/demo.json
          gait pack build --type run --from run_demo --out ./gait-out/adopt_one_pr/pack_run_demo.zip --json > ./gait-out/adopt_one_pr/pack_build.json
          gait pack verify ./gait-out/adopt_one_pr/pack_run_demo.zip --json > ./gait-out/adopt_one_pr/pack_verify.json

      - name: Initialize deterministic fixture
        shell: bash
        run: |
          gait regress init --from run_demo --json > ./gait-out/adopt_one_pr/regress_init.json

      - name: Force deterministic regression drift
        shell: bash
        run: |
          cat > fixtures/run_demo/candidate_record.json <<'JSON'
          {
            "run": {
              "schema_id": "gait.runpack.run",
              "schema_version": "1.0.0",
              "created_at": "2026-02-12T00:00:00Z",
              "producer_version": "0.0.0-dev",
              "run_id": "run_demo_candidate",
              "env": {"os": "linux", "arch": "amd64", "runtime": "go"},
              "timeline": [{"event": "run_started", "ts": "2026-02-12T00:00:00Z"}]
            },
            "intents": [
              {
                "schema_id": "gait.runpack.intent",
                "schema_version": "1.0.0",
                "created_at": "2026-02-12T00:00:00Z",
                "producer_version": "0.0.0-dev",
                "run_id": "run_demo_candidate",
                "intent_id": "intent_1",
                "tool_name": "tool.write",
                "args_digest": "1111111111111111111111111111111111111111111111111111111111111111"
              }
            ],
            "results": [
              {
                "schema_id": "gait.runpack.result",
                "schema_version": "1.0.0",
                "created_at": "2026-02-12T00:00:00Z",
                "producer_version": "0.0.0-dev",
                "run_id": "run_demo_candidate",
                "intent_id": "intent_1",
                "status": "error",
                "result_digest": "2222222222222222222222222222222222222222222222222222222222222222"
              }
            ],
            "refs": {
              "schema_id": "gait.runpack.refs",
              "schema_version": "1.0.0",
              "created_at": "2026-02-12T00:00:00Z",
              "producer_version": "0.0.0-dev",
              "run_id": "run_demo_candidate",
              "receipts": []
            },
            "capture_mode": "reference"
          }
          JSON

          gait run record --input fixtures/run_demo/candidate_record.json --out-dir fixtures/run_demo --json > ./gait-out/adopt_one_pr/candidate_record_result.json

          python3 - <<'PY'
          import json
          from pathlib import Path

          fixture_path = Path("fixtures/run_demo/fixture.json")
          payload = json.loads(fixture_path.read_text(encoding="utf-8"))
          payload["candidate_runpack"] = "runpack_run_demo_candidate.zip"
          fixture_path.write_text(json.dumps(payload, indent=2) + "\n", encoding="utf-8")
          PY

      - name: Run regress (expected to fail with stable exit 5)
        id: regress
        shell: bash
        run: |
          set +e
          gait regress run --json --junit ./gait-out/adopt_one_pr/junit.xml > ./gait-out/adopt_one_pr/regress_result.json
          status=$?
          set -e

          echo "exit_code=${status}" >> "$GITHUB_OUTPUT"

          if [[ "${status}" -ne 5 ]]; then
            echo "expected stable regress exit code 5, got ${status}" >&2
            exit 1
          fi
          exit "${status}"

      - name: Upload PR evidence artifacts
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: gait-adopt-one-pr-artifacts
          path: |
            gait-out/adopt_one_pr/
            fixtures/run_demo/
```

Expected PR behavior:

- workflow fails because forced regress drift exits `5`
- artifact bundle is uploaded with:
  - `pack_build.json`
  - `pack_verify.json`
  - `regress_result.json`
  - `junit.xml`

If you want a passing lane after validating setup, remove the forced-drift step.
