# Approval Runbook (Epic A4.3)

This runbook defines how to operate approval-gated tool execution in production.

## Scope

Use this for policies that return `require_approval` and for audit workflows tied to signed gate traces.

Runtime performance and reliability expectations for this path are defined in:

- `docs/slo/runtime_slo.md`

## Roles

- Requester: service or operator submitting an approval-required intent.
- Approver: authorized human/system identity issuing approval token(s).
- Security owner: key custody, rotation, and incident review.

## Prerequisites

- Policy and intent digests are available from `gait gate eval --json` output.
- Signing keys are provisioned in a secure keystore or HSM-backed secret manager.
- Public verification keys are distributed to runtime and audit systems.

## Step 1: Evaluate Intent

```bash
gait gate eval --policy <policy.yaml> --intent <intent.json> --trace-out ./gait-out/trace_pre_approval.json --json
```

Expected:

- Exit `4`
- verdict `require_approval`
- stable `intent_digest` and `policy_digest`
- for context-required rules, non-compliant requests may return block with:
  - `context_evidence_missing`
  - `context_set_digest_missing`
  - `context_evidence_mode_mismatch`
  - `context_freshness_exceeded`

## Step 1A: Context-Required Re-evaluation (When Policy Demands Context Evidence)

Before token minting for context-required rules, validate intent context linkage:

```bash
gait gate eval --policy <policy.yaml> --intent <intent.json> --json
```

Intent context requirements:

- `context.context_set_digest` present
- `context.context_evidence_mode=required` when policy requires required mode
- optional freshness bound satisfied (`context.auth_context.context_age_seconds <= max_context_age_seconds`)

If any requirement fails, do not mint approval token. Fix intent context and re-evaluate.

## Step 2: Mint Approval Token

Create one token per approver:

```bash
gait approve \
  --intent-digest <intent_digest> \
  --policy-digest <policy_digest> \
  --ttl 1h \
  --scope tool.write \
  --approver approver@company \
  --reason-code change_ticket_123 \
  --json > token_a.json
```

For multi-party requirements, mint additional tokens (`token_b.json`, ...).

## Step 3: Re-evaluate With Approval Token Chain

```bash
gait gate eval \
  --policy <policy.yaml> \
  --intent <intent.json> \
  --approval-token token_a.json \
  --approval-token-chain token_b.json \
  --trace-out ./gait-out/trace_post_approval.json \
  --json
```

Expected:

- Exit `0` and verdict `allow`, or
- Exit `4` if approvals are still insufficient
- Exit `3` and verdict `block` for context-required violations until context proof is corrected

## Step 4: Execute Via Wrapped Tool Path

- Execute side effects only after approved `allow`.
- Keep wrappers fail-closed on non-`allow` decisions.

## Token TTL And Scope Policy

- Default TTL: `1h`
- High-risk operations: `15m` to `30m`
- Scope must be minimal and tool-specific (for example `tool.write`, not wildcard scope).
- Tokens are single-intent by digest; do not reuse across different intents.
- Do not store tokens in source control or long-lived shared volumes.

## Key Handling Policy

- Keep private signing keys outside repos and developer workstations when possible.
- Prefer environment or secret manager injection over plaintext files.
- Rotate approval signing keys on a fixed schedule and immediately after suspected compromise.
- Version and publish active public keys to verification consumers before key cutover.
- Enforce key separation:
  - approval signing key
  - gate trace signing key
  - release signing keys

## Incident Audit Workflow

1. Collect artifacts:
   - `trace_*.json`
   - `approval_audit_*.json`
   - `credential_evidence_*.json` (if broker gating is enabled)
2. Verify trace signatures:

```bash
gait trace verify ./gait-out/trace_post_approval.json --json --public-key ./public.key
```

3. Build evidence pack:

```bash
gait guard pack --run <run_id_or_path> --template incident_response --json
```

4. Attach evidence pack and trace verification output to incident ticket.

Optional deterministic triage item creation:

```bash
bash scripts/bridge_trace_to_beads.sh --trace ./gait-out/trace_post_approval.json --dry-run --json
```

## Operational Guardrails

- If policy evaluation fails (`exit 6`) or trace verification fails, block execution and open incident.
- If approval tokens are expired or scope-mismatched, deny execution and re-issue tokens.
- If approver identity is not authorized, deny token issuance and notify security owner.
- Validate runtime SLO posture before release with:

```bash
make bench-budgets
```
