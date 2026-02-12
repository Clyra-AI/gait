# Control and Prove Agent Tool Calls with Verifiable Runpacks

Gait captures a signed runpack for every tool-using AI run so you can verify, replay (stub mode), and diff behavior offline. Add regressions to stop drift in CI. Optionally gate high-risk tool calls with policy and approvals.

![PR Fast](https://github.com/davidahmann/gait/actions/workflows/pr-fast.yml/badge.svg)
![CodeQL](https://github.com/davidahmann/gait/actions/workflows/codeql.yml/badge.svg)
![Intent+Receipt Conformance](https://github.com/davidahmann/gait/actions/workflows/intent-receipt-conformance.yml/badge.svg)

Public docs: [https://davidahmann.github.io/gait/](https://davidahmann.github.io/gait/)  
Wiki: [https://github.com/davidahmann/gait/wiki](https://github.com/davidahmann/gait/wiki)  
Runpack format: [`docs/contracts/primitive_contract.md`](docs/contracts/primitive_contract.md)  
Changelog: [CHANGELOG.md](CHANGELOG.md)

Primary CTA: `gait demo` (offline, <60s)  
Secondary CTA: verify artifacts with `gait verify <path>`

- verifiable receipts: signed runpacks and trace records
- debuggable by default: replay and diff two runs to see what changed
- prevent repeats: convert a run into a regression test in CI

Outputs: `run_id`, `runpack_<run_id>.zip`, and a ticket footer you can paste into incidents.

## Try It (Offline, <60s)

Install

```bash
# macOS
brew install gait

# Linux / Windows
# download binary from GitHub Releases:
# https://github.com/davidahmann/gait/releases
```

Run demo

```bash
gait demo
```

Verify

```bash
gait verify ./gait-out/runpack_run_demo.zip
```

Install details: [`docs/install.md`](docs/install.md) and [`docs/homebrew.md`](docs/homebrew.md)

## Gait In 20 Seconds

![Gait runpack-first terminal demo](docs/assets/gait_demo_20s.gif)

Regenerate asset: `bash scripts/record_runpack_hero_demo.sh`

## Why Gait

AI agents now execute high-authority actions: write data, mutate repos, call external APIs, rotate infra. Most stacks still rely on prompt scanning and after-the-fact observability.

Gait keeps the contract deterministic and offline-first:

- runpack: signed artifact per run (`gait verify`, `gait run replay`, `gait run diff`)
- regress: convert incidents into CI checks (`gait regress init`, `gait regress run`)
- optional gate: enforce policy and approvals at tool-call time (`gait gate eval`)

If your agent touched production, attach the runpack.

## First Win

```bash
gait demo
gait verify run_demo
gait run receipt --from run_demo
```

Expected output includes:

- `run_id=run_demo`
- signed bundle under `gait-out/`
- `ticket_footer=GAIT run_id=...` for PRs/incidents

## Optional Local UI

Run a local-only UI for guided demo and onboarding flows:

```bash
go build -o ./gait ./cmd/gait
./gait ui --open-browser=false
# open http://127.0.0.1:7980
```

Details: [`docs/ui_localhost.md`](docs/ui_localhost.md) and [`docs/contracts/ui_contract.md`](docs/contracts/ui_contract.md)

## Core OSS Surfaces

- `runpack`: record, inspect, verify, diff, receipt, replay (stub default)
- `regress`: incident-to-regression workflow with CI/JUnit outputs
- `gate`: policy evaluation for tool intent with signed trace output
- `doctor`: first-run diagnostics and production-readiness checks
- `scout`: local snapshot/diff/signal analysis for drift clustering
- `guard` and `incident`: deterministic evidence and incident bundles
- `registry`: signed/pinned skill-pack install and verify workflows
- `mcp proxy/bridge/serve`: transport adapters that enforce through Gate

## Turn Incidents Into CI Regressions

One command path:

```bash
gait regress bootstrap --from run_demo --json --junit ./gait-out/junit.xml
```

Explicit path:

```bash
gait regress init --from run_demo --json
gait regress run --json --junit ./gait-out/junit.xml
```

Deterministic failure contract:

- exit `0` = pass
- exit `5` = regression failed

Template workflow: [`.github/workflows/adoption-regress-template.yml`](.github/workflows/adoption-regress-template.yml)
Drop-in action: [`.github/actions/gait-regress/README.md`](.github/actions/gait-regress/README.md)

## Optional: Enforce At The Tool Boundary

Gait does not auto-intercept your framework. Your dispatcher must call Gait and enforce non-`allow` as non-executable.

```python
def dispatch_tool(tool_call):
    decision = gait_evaluate(tool_call)
    if decision["verdict"] != "allow":
        return {"executed": False, "verdict": decision["verdict"]}
    return {"executed": True, "result": execute_real_tool(tool_call)}
```

Minimal evaluation command:

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_delete.json \
  --trace-out ./gait-out/trace_delete.json \
  --json
```

Policy authoring and rollout docs: [`docs/policy_authoring.md`](docs/policy_authoring.md), [`docs/policy_rollout.md`](docs/policy_rollout.md), [`docs/approval_runbook.md`](docs/approval_runbook.md)

## Long-Running Sessions And Delegation

Checkpoint multi-day runs without losing deterministic history:

```bash
gait run session start --journal ./gait-out/sessions/demo.journal.jsonl --session-id sess_demo --run-id run_demo --json
gait run session append --journal ./gait-out/sessions/demo.journal.jsonl --tool tool.write --verdict allow --intent-id intent_1 --json
gait run session checkpoint --journal ./gait-out/sessions/demo.journal.jsonl --out ./gait-out/runpack_demo_cp_0001.zip --json
gait verify session-chain --chain ./gait-out/sessions/demo.journal_chain.json --json
```

Use delegation tokens for controlled multi-agent handoffs:

```bash
gait delegate mint --delegator agent.lead --delegate agent.specialist --scope tool:tool.write --scope-class write --ttl 1h --private-key ./delegation_private.key --out ./gait-out/delegation_token.json --json
gait gate eval --policy examples/policy/base_high_risk.yaml --intent examples/policy/intents/intent_delegated_egress_valid.json --delegation-token ./gait-out/delegation_token.json --delegation-public-key ./delegation_public.key --json
```

## Integrations

Blessed v2.3 lane:

- [`examples/integrations/openai_agents/`](examples/integrations/openai_agents/)
- [`.github/workflows/adoption-regress-template.yml`](.github/workflows/adoption-regress-template.yml)

Additional maintained references:

- [`examples/integrations/langchain/`](examples/integrations/langchain/)
- [`examples/integrations/autogen/`](examples/integrations/autogen/)
- [`examples/integrations/autogpt/`](examples/integrations/autogpt/)
- [`examples/integrations/openclaw/`](examples/integrations/openclaw/)
- [`examples/integrations/gastown/`](examples/integrations/gastown/)

Integration runbook: [`docs/integration_checklist.md`](docs/integration_checklist.md)

## Production Posture

High-risk profile (`oss-prod`) is fail-closed on policy/approval ambiguity:

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_delete.json \
  --profile oss-prod \
  --key-mode prod \
  --private-key ./gait-out/keys/prod_private.key \
  --credential-broker env \
  --json
```

Readiness command:

```bash
gait doctor --production-readiness --json
```

Hardening references: [`docs/hardening/v2_2_contract.md`](docs/hardening/v2_2_contract.md), [`docs/hardening/prime_time_runbook.md`](docs/hardening/prime_time_runbook.md)

## Command Surface

Most-used commands:

```text
gait demo
gait verify <run_id|path>
gait run inspect --from <run_id|path>
gait regress bootstrap --from <run_id|path>
gait gate eval --policy <policy.yaml> --intent <intent.json>
gait policy test <policy.yaml> <intent_fixture.json>
gait doctor --json
gait ui --open-browser=false
```

All commands support `--json`; most support `--explain`.

## Contract Commitments

- determinism: verify/diff/stub replay are deterministic on identical artifacts
- offline-first: core workflows do not require network
- fail-closed: high-risk paths block on policy/approval ambiguity
- schema stability: versioned artifacts with backward-compatible readers
- stable exit codes: `0` success, `2` verify failure, `3` policy block, `4` approval required, `5` regress failed, `6` invalid input

Normative contract: [`docs/contracts/primitive_contract.md`](docs/contracts/primitive_contract.md)

## Documentation Map

Start here:

1. [`docs/README.md`](docs/README.md)
2. [`docs/concepts/mental_model.md`](docs/concepts/mental_model.md)
3. [`docs/architecture.md`](docs/architecture.md)
4. [`docs/flows.md`](docs/flows.md)
5. [`docs/contracts/primitive_contract.md`](docs/contracts/primitive_contract.md)

## Developer Workflow

```bash
make fmt
make lint
make test
make test-e2e
make test-adoption
make test-contracts
make test-hardening-acceptance
make test-uat-local
```

Local push hooks:

```bash
make hooks
```

- default pre-push path: `make prepush` (fast)
- full local gate: `GAIT_PREPUSH_MODE=full git push`

Apply `main` guardrails (PR-only + required checks):

```bash
make github-guardrails
```

Contributor guide: [`CONTRIBUTING.md`](CONTRIBUTING.md)

## OSS And Enterprise Boundary

- OSS is the executable substrate: runpack, regress, gate, doctor, scout, guard, adapters
- Enterprise is a separate fleet governance layer that consumes OSS artifacts
- Enterprise packaging does not change OSS runtime contracts

Boundary details: [`docs/packaging.md`](docs/packaging.md)

## Feedback

- Issues: [https://github.com/davidahmann/gait/issues](https://github.com/davidahmann/gait/issues)
- Security reporting: [`SECURITY.md`](SECURITY.md)
- Contribution guide: [`CONTRIBUTING.md`](CONTRIBUTING.md)
- Community expectations: [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)
