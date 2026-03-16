# Gait Quickstart

Use this when you need deterministic control plus evidence at agent tool boundaries.

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/install.sh | bash

# Machine-readable install probe
gait version --json
gait doctor --json

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
{"ok":true,"policy_path":".gait.yaml","template":"baseline-highrisk","detected_signals":[{"code":"framework.langchain","category":"framework","value":"langchain","confidence":"high"}],"generated_rules":[{"id":"starter.block.destructive","name":"block-destructive-tools","effect":"block"}]}
{"ok":true,"policy_path":".gait.yaml","default_verdict":"block","rule_count":7,"next_commands":["gait policy validate .gait.yaml --json","gait doctor --json","gait demo --json"],"findings":[{"code":"repo.generated_rules_available","severity":"info","detected_surface":"repo.signals"}]}
```

Expected demo shape:

```text
run_id=run_demo
ticket_footer=GAIT run_id=run_demo ...
verify=ok
```

For SDKs and wrappers, prefer the JSON form and treat the text form as human-facing output only.

For binary discovery and install automation, use `gait version --json` (or `gait --version --json` / `gait -v --json`). `gait --help` is text-only and exits `0`.

Context-required policies must pass `--context-envelope <context_envelope.json>` on `gait gate eval`, `gait mcp proxy`, or `gait mcp serve`; raw intent context claims are not authoritative by themselves.

Then continue with one integration seam:

- one-PR CI adoption: `/docs/adopt_in_one_pr/`
- durable jobs lifecycle: `/docs/durable_jobs/`
- production integration checklist: `/docs/integration_checklist/`
- LangChain middleware contract: `/docs/sdk/python/`

Boundary touchpoints:

- wrapper or sidecar dispatch site: `gait gate eval`
- context-required boundary: `gait gate eval --context-envelope ...` or `gait mcp serve --context-envelope ...`
- machine-readable smoke path: `gait demo --json`

Use `gait policy test` and `gait gate eval --simulate` before enforce rollout on high-risk tool-call boundaries. `gait enforce` is a bounded wrapper for integrations that already emit Gait trace references.

Before high-risk production enforcement, seed the canonical hardened config and require readiness to pass:

```bash
# From a repo checkout:
cp examples/config/oss_prod_template.yaml .gait/config.yaml

# Or, if you installed only the binary:
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/examples/config/oss_prod_template.yaml -o .gait/config.yaml

gait check --json
gait doctor --production-readiness --json
```

Do not treat `oss-prod` enforcement as production-ready until that doctor command reports `ok=true`.

Standard `gait doctor --json` is truthful in a clean writable directory after a binary-only install: repo-only schema/example checks stay scoped to a Gait repo checkout.

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
