# Gait

Your AI agent broke prod. Gait gives you the signed artifact to prove what happened, the regression to make sure it never happens again, and the policy gate to block it at the boundary.

Public docs + marketing site: `https://davidahmann.github.io/gait/`

## Install And First Win (60 Seconds)

```bash
curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/install.sh | bash
```

Linux and macOS. Windows: see `docs/install.md`.
Homebrew (tap) alternative:

```bash
brew tap davidahmann/tap
brew install gait
```

Tap-first release details: `docs/homebrew.md`.

```bash
gait demo
```

```text
run_id=run_demo
bundle=./gait-out/runpack_run_demo.zip
ticket_footer=GAIT run_id=run_demo manifest=sha256:88913ed... verify="gait verify run_demo"
verify=ok
```

You now have a signed, portable execution artifact. Verify it:

```bash
gait verify run_demo
```

Paste the `ticket_footer` line into any incident ticket or PR. Anyone with the artifact can verify it offline.

## What Just Happened

- **runpack** = a signed ZIP of exactly what the agent did (intents, results, receipts, manifest with SHA-256 hashes)
- **verify** = offline integrity proof anyone can run, no network needed
- **ticket_footer** = the one line you paste into tickets so incidents are traceable

Full mental model: `docs/concepts/mental_model.md`

## Turn That Into A CI Regression (2 Minutes)

```bash
gait regress init --from run_demo --json
gait regress run --json --junit ./gait-out/junit.xml
```

This incident is now a permanent test. If agent behavior drifts, CI fails.

What you get:

- `gait.yaml` and `fixtures/run_demo/runpack.zip`
- `regress_result.json`
- `junit.xml` for CI test reporting

Exit codes are stable and CI-friendly:

- `0` success
- `5` regression failed

When regress fails, JSON output includes `top_failure_reason`, `next_command`, and `artifact_paths` so you can act without parsing large files.

Canonical CI template: `.github/workflows/adoption-regress-template.yml`

## Block Dangerous Tool Calls (5 Minutes)

Gate examples use fixture files from this repository:

```bash
git clone https://github.com/davidahmann/gait.git && cd gait
```

Test policies deterministically (no side effects, no keys needed):

```bash
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delete.json --json
```

- low risk read -> `allow`
- high risk destructive call -> `block`

Evaluate a real intent through Gate:

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_delete.json \
  --trace-out ./gait-out/trace_delete.json \
  --json
```

Every gate decision produces a signed `trace_<id>.json`. Every block is auditable.

Block a prompt-injection-style tool call:

```bash
gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json
```

Result: `verdict: block`, `reason_codes: ["blocked_prompt_injection"]`

Gate evaluates structured tool-call intent, not prompt text. If verdict is not `allow`, execution does not run.

## Why This Matters

**What Gait is:**

- an execution-boundary guard for production agent tool calls
- a verifiable artifact standard (runpack + trace) for incidents, CI, and audits
- a vendor-neutral CLI that works across frameworks and model providers

**What Gait is not:**

- not a hosted dashboard
- not prompt-only filtering
- not a replacement for your identity provider, SIEM, or ticketing system

**Why tool-call boundary, not prompt layer:**

Tool calls are where authority is exercised. Portable artifacts are the durable evidence contract. Deterministic regressions turn one incident into a permanent safety test.

**OSS and Enterprise:**

- OSS v1 is the free execution substrate: runpack, regress, gate, doctor, scout, and adapter kits.
- Enterprise is a separate control-plane layer for fleet governance, consuming OSS artifacts.
- Artifact contracts remain stable regardless of enterprise adoption.

Details: `docs/packaging.md`

## Demo To Production (30 to 120 Minutes)

1. Walk through `docs/integration_checklist.md` once.
2. Pick the framework adapter closest to your stack:
   - `examples/integrations/openai_agents`
   - `examples/integrations/langchain`
   - `examples/integrations/autogen`
   - `examples/integrations/openclaw`
   - `examples/integrations/autogpt`
3. Wire the boundary: wrapper or sidecar -> `gait gate eval` -> execute only on `allow`.
4. Add a regress fixture and JUnit output in CI before enabling privileged tools.

Reduce repeated flags with a project config: `docs/project_defaults.md`

## Production Posture (oss-prod Profile)

For fail-closed production enforcement, use `--profile oss-prod` with explicit keys and a credential broker:

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_delete.json \
  --profile oss-prod \
  --key-mode prod \
  --private-key examples/scenarios/keys/approval_private.key \
  --credential-broker stub \
  --trace-out ./gait-out/trace_delete_prod.json \
  --json
```

The demo key in `examples/scenarios/keys/` is for walkthroughs only. In production, provision your own key and replace `stub` with `env` or `command` broker.

For staged rollout, simulate mode reports what *would have* happened without enforcing:

```bash
gait gate eval \
  --policy examples/policy/base_medium_risk.yaml \
  --intent examples/policy/intents/intent_write.json \
  --simulate --json
```

References: `docs/approval_runbook.md`, `docs/policy_rollout.md`, `docs/project_defaults.md`

## Local Signal Engine

Cluster incident families and rank top issues offline:

```bash
gait scout signal --runs ./gait-out/runpack_run_demo.zip --regress ./gait-out/regress_result.json --json
```

Output includes deterministic fingerprints, family grouping, ranked top issues with driver attribution (`policy_change`, `tool_result_shape_change`, `reference_set_change`, `configuration_change`), and bounded fix suggestions.

## Ecosystem

- Discovery: `docs/ecosystem/awesome.md`
- Contribute: `docs/ecosystem/contribute.md`
- Machine-readable index: `docs/ecosystem/community_index.json`
- Adapter proposal: `.github/ISSUE_TEMPLATE/adapter.yml`
- Skill proposal: `.github/ISSUE_TEMPLATE/skill.yml`

## Commands

```text
gait demo                                          # offline first win
gait verify <run_id|path>                          # offline integrity proof
gait run replay <run_id|path>                      # deterministic stub replay
gait run diff <left> <right>                       # artifact diff
gait regress init --from <run_id|path>             # incident fixture bootstrap
gait regress run [--junit junit.xml]               # run regressions
gait policy test <policy.yaml> <fixture.json>      # test policy offline
gait gate eval --policy <p> --intent <i>           # evaluate tool intent
gait approve --intent-digest ... --ttl ...         # mint approval token
gait scout signal --runs <csv>                     # cluster incidents
gait guard pack --run <id> --out <path>            # evidence bundle
gait incident pack --from <id> --window <dur>      # incident bundle
gait trace verify <trace.json>                     # verify signed trace
gait doctor --json                                 # environment diagnostics
```

All commands support `--json`. Most support `--explain`.

## Contracts

- Primitive contract: `docs/contracts/primitive_contract.md`
- Determinism: verify, diff, and stub replay are deterministic for identical artifacts
- Offline-first: core workflows require no network
- Fail-closed: high-risk paths block on policy or approval ambiguity
- Schema stability: versioned artifacts, backward-compatible readers
- Exit codes: `0` success, `2` verification failed, `3` policy block, `4` approval required, `5` regress failed, `6` invalid input
- Runtime SLOs: `docs/slo/runtime_slo.md`

## Docs

1. `docs/README.md`
2. `docs/concepts/mental_model.md`
3. `docs/architecture.md`
4. `docs/flows.md`
5. `docs/integration_checklist.md`
6. `docs/project_defaults.md`
7. `docs/policy_rollout.md`
8. `docs/approval_runbook.md`
9. `docs/ci_regress_kit.md`
10. `docs/contracts/primitive_contract.md`
11. `docs/evidence_templates.md`
12. `docs/positioning.md`
13. `docs/packaging.md`
14. `docs/install.md`
15. `docs/ecosystem/awesome.md`
16. `docs/launch/README.md`
17. `docs/homebrew.md`

## Development

```bash
make fmt && make lint && make test
make test-e2e
make test-adoption
make test-contracts
make test-hardening-acceptance
make test-release-smoke
make test-uat-local
make docs-site-install
make docs-site-build
```

90-second terminal demo: `bash scripts/demo_90s.sh`

Enable hooks: `pre-commit install --hook-type pre-commit --hook-type pre-push`

## Links

- `SECURITY.md`
- `CONTRIBUTING.md`
- `CODE_OF_CONDUCT.md`
- `docs/hardening/contracts.md`
- `docs/hardening/release_checklist.md`
- `docs/uat_functional_plan.md`
- `product/PRD.md`
- `product/ROADMAP.md`
