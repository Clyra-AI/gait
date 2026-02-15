# Gait Quickstart

Use this when you need deterministic control + evidence at agent tool boundaries.

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/install.sh | bash

# Guided tour
gait tour

# Create a signed pack from a synthetic agent run
gait demo

# Prove it's intact
gait verify run_demo

# Turn it into a CI regression gate
gait regress bootstrap --from run_demo --junit ./gait-out/junit.xml

# Try durable jobs and policy demos
gait demo --durable
gait demo --policy
```

Then continue with:

- one-PR CI adoption: `/docs/adopt_in_one_pr/`
- durable jobs lifecycle: `/docs/durable_jobs/`
- production integration checklist: `/docs/integration_checklist/`

Use `gait policy test` and `gait gate eval --simulate` before enforce rollout on high-risk tool-call boundaries.
