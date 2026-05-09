# Starter Policy Templates (Epic A4.1)

These templates provide baseline policy packs by risk tier:

- `base_low_risk.yaml`
- `base_medium_risk.yaml`
- `base_high_risk.yaml`
- `baseline-highrisk.yaml` (hyphenated alias)
- `knowledge_worker_safe.yaml` (reversible-first profile)

Scaffold a baseline file directly from the CLI:

```bash
gait policy init baseline-highrisk --out ./gait.policy.yaml --json
```

Each template includes explicit examples for all three verdicts:

- `allow`
- `require_approval`
- `block`
- delegated egress allow/block controls for high-risk write paths
- JIT-only credential posture for covered high-risk write and deploy paths
- deterministic freeze-window examples for production deploy holds
- sandbox posture examples for high-risk `proc.exec`
- generalized kill-switch examples for additive emergency stops
- trust-graduation stage examples from observe through brokered write
- tainted-data egress blocking for prompt-injection-style payloads

Run from repo root:

```bash
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_write.json --json
gait policy test examples/policy/base_low_risk.yaml examples/policy/intents/intent_delete.json --json

gait policy test examples/policy/base_medium_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_medium_risk.yaml examples/policy/intents/intent_write.json --json
gait policy test examples/policy/base_medium_risk.yaml examples/policy/intents/intent_delete.json --json

gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_write.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delete.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_tainted_egress.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delegated_egress_valid.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/intents/intent_delegated_egress_invalid.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/credentials/intent_aws_sts_allow.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/credentials/intent_github_oidc_allow.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/credentials/intent_vault_dynamic_allow.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/credentials/intent_github_pat_static_block.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/credentials/intent_aws_iam_user_cloud_admin_block.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/credentials/intent_env_inherited_block.json --json
gait policy test examples/policy/base_high_risk.yaml examples/policy/credentials/intent_unknown_provenance_block.json --json
gait gate eval --policy examples/policy/freeze_windows/production_block.yaml --intent examples/policy/freeze_windows/intent_prod_deploy.json --evaluation-time 2026-03-10T14:30:00Z --json
gait gate eval --policy examples/policy/freeze_windows/production_require_approval.yaml --intent examples/policy/freeze_windows/intent_prod_deploy.json --evaluation-time 2026-03-10T14:30:00Z --json
gait gate eval --policy examples/policy/sandbox/allow_sandboxed_exec.yaml --intent examples/policy/sandbox/intent_proc_exec_valid.json --json
gait gate eval --policy examples/policy/sandbox/allow_sandboxed_exec.yaml --intent examples/policy/sandbox/intent_proc_exec_missing_sandbox.json --json
gait gate eval --policy examples/policy/kill_switch/allow_exec.yaml --intent examples/policy/kill_switch/intent_exec.json --kill-switch-state examples/policy/kill_switch/kill_switch_state_identity.json --json
gait policy test examples/policy/trust_graduation/read_only_allow.yaml examples/policy/intents/intent_read.json --json
gait policy test examples/policy/trust_graduation/approval_gated_write.yaml examples/policy/intents/intent_write.json --json
gait policy test examples/policy/trust_graduation/blocked_destructive.yaml examples/policy/intents/intent_delete.json --json
```

Expected verdict matrix:

- `intent_read.json` => `allow` (exit `0`)
- `intent_write.json` => `require_approval` (exit `4`)
- `intent_delete.json` => `block` (exit `3`)
- `intent_tainted_egress.json` => `block` (exit `3`)
- `intent_delegated_egress_valid.json` => `allow` (exit `0`)
- `intent_delegated_egress_invalid.json` => `block` (exit `3`)
- `credentials/intent_aws_sts_allow.json` => `allow` (exit `0`)
- `credentials/intent_github_oidc_allow.json` => `allow` (exit `0`)
- `credentials/intent_vault_dynamic_allow.json` => `allow` (exit `0`)
- `credentials/intent_github_pat_static_block.json` => `block` (exit `3`)
- `credentials/intent_aws_iam_user_cloud_admin_block.json` => `block` (exit `3`)
- `credentials/intent_env_inherited_block.json` => `block` (exit `3`)
- `credentials/intent_unknown_provenance_block.json` => `block` (exit `3`)
- `freeze_windows/production_block.yaml` + `intent_prod_deploy.json` => `block` (exit `3`)
- `freeze_windows/production_require_approval.yaml` + `intent_prod_deploy.json` => `require_approval` (exit `4`)
- `sandbox/allow_sandboxed_exec.yaml` + `intent_proc_exec_valid.json` => `allow` (exit `0`)
- `sandbox/allow_sandboxed_exec.yaml` + `intent_proc_exec_missing_sandbox.json` => `block` (exit `3`)
- `kill_switch/allow_exec.yaml` + `intent_exec.json` + `kill_switch_state_identity.json` => `block` (exit `3`)
- `trust_graduation/read_only_allow.yaml` + `intent_read.json` => `allow` (exit `0`)
- `trust_graduation/approval_gated_write.yaml` + `intent_write.json` => `require_approval` (exit `4`)
- `trust_graduation/blocked_destructive.yaml` + `intent_delete.json` => `block` (exit `3`)

High-risk note:

- `base_high_risk.yaml` marks write actions with `require_broker_credential: true` for least-privilege brokering.
- `base_high_risk.yaml` and `baseline-highrisk.yaml` accept scoped, time-bounded
  AWS STS, GitHub OIDC, and Vault-style dynamic credentials for covered
  high-risk write and deploy paths by default.
- `base_high_risk.yaml` requires explicit delegation metadata for high-risk egress writes and blocks tainted external payload flow to network destinations.
- `base_high_risk.yaml` and `baseline-highrisk.yaml` include `destructive_budget` defaults to fail-closed once destructive threshold windows are exceeded.
- `knowledge_worker_safe.yaml` defaults unknown tools to block, prefers archive/trash actions, and requires explicit break-glass approval for permanent delete paths.
- For runtime checks in hardened mode, evaluate with `--profile oss-prod` and an explicit broker, for example:

```bash
gait gate eval \
  --policy examples/policy/base_high_risk.yaml \
  --intent examples/policy/intents/intent_write.json \
  --profile oss-prod \
  --key-mode prod \
  --private-key ./prod_signing.key \
  --credential-broker stub \
  --json
```
