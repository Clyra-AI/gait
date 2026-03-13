# Gait Quickstart

Use this when you need deterministic control plus evidence at agent tool boundaries.

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/install.sh | bash

# Bootstrap repo policy-as-code
gait init --json
gait check --json

# Create a signed pack from a synthetic agent run
gait demo

# Machine-readable wrapper/SDK path
gait demo --json

# Prove it's intact
gait verify run_demo --json

# Turn it into a CI regression gate
gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml
```

Expected bootstrap shape:

```json
{"ok":true,"policy_path":".gait.yaml","template":"baseline-highrisk"}
{"ok":true,"policy_path":".gait.yaml","default_verdict":"block","rule_count":7}
```

Expected demo shape:

```text
run_id=run_demo
ticket_footer=GAIT run_id=run_demo ...
verify=ok
```

For SDKs and wrappers, prefer the JSON form and treat the text form as human-facing output only.

Then continue with one integration seam:

- one-PR CI adoption: `/docs/adopt_in_one_pr/`
- durable jobs lifecycle: `/docs/durable_jobs/`
- production integration checklist: `/docs/integration_checklist/`
- LangChain middleware contract: `/docs/sdk/python/`

Use `gait policy test` and `gait gate eval --simulate` before enforce rollout on high-risk tool-call boundaries. `gait enforce` is a bounded wrapper for integrations that already emit Gait trace references.

Wrapper lane example:

```bash
gait test --json -- python3 examples/integrations/openai_agents/quickstart.py --scenario allow
gait trace --json -- python3 examples/integrations/openai_agents/quickstart.py --scenario allow
gait capture --from run_demo --json
gait regress add --from ./gait-out/capture.json --json
```

For MCP server admission, keep trust inputs local:

```bash
gait mcp verify --policy ./examples/integrations/mcp_trust/policy.yaml --server ./examples/integrations/mcp_trust/server_github.json --json
```

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
