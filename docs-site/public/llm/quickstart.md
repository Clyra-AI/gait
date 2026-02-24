# Gait Quickstart

Use this when you need deterministic control + evidence at agent tool boundaries.

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/install.sh | bash

# Guided tour
gait tour

# Create a signed pack from a synthetic agent run
gait demo

# Prove it's intact
gait verify run_demo

# Export OTEL + Postgres index SQL from the same pack
gait pack build --type run --from run_demo --out ./gait-out/pack_run_demo.zip
gait pack export ./gait-out/pack_run_demo.zip --otel-out ./gait-out/pack_run_demo.otel.jsonl --postgres-sql-out ./gait-out/pack_index.sql

# Turn it into a CI regression gate
gait regress bootstrap --from run_demo --junit ./gait-out/junit.xml

# Try durable jobs and policy demos
gait demo --durable
gait demo --policy
gait policy init baseline-highrisk --out ./gait.policy.yaml --json
```

Then continue with:

- one-PR CI adoption: `/docs/adopt_in_one_pr/`
- durable jobs lifecycle: `/docs/durable_jobs/`
- production integration checklist: `/docs/integration_checklist/`

Use `gait policy test` and `gait gate eval --simulate` before enforce rollout on high-risk tool-call boundaries.

For emergency preemption drills:

```bash
gait job submit --id job_safe --json
gait job stop --id job_safe --actor secops --json
```

For script automation boundaries, add:

```bash
gait approve-script --policy ./policy.yaml --intent ./script_intent.json --registry ./approved_scripts.json --approver secops --json
gait list-scripts --registry ./approved_scripts.json --json
gait gate eval --policy ./policy.yaml --intent ./script_intent.json --approved-script-registry ./approved_scripts.json --json
```
