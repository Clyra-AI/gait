# Gait

Gait is an offline-first CLI that makes production AI agent runs controllable and debuggable by default:

- **Runpack**: record, verify, diff, replay (stub by default)
- **Regress**: turn runpacks into deterministic CI regressions
- **Gate**: evaluate tool-call intent against policy with signed traces
- **Doctor**: first-5-minutes diagnostics with stable JSON output

The product contract is artifacts + schemas, not a hosted UI.

## Start Here

Use one install path: download a release binary from GitHub Releases:

- <https://github.com/davidahmann/gait/releases>

Then run:

```bash
gait demo
gait verify run_demo
```

Expected `gait demo` output:

```text
run_id=run_demo
bundle=./gait-out/runpack_run_demo.zip
ticket_footer=GAIT run_id=run_demo manifest=sha256:<digest> verify="gait verify run_demo"
verify=ok
```

## Ticket Footer Semantics

`ticket_footer` is a copy/paste-ready receipt for incident tickets:

- `run_id`: stable handle for this execution artifact set
- `manifest=sha256:<digest>`: immutable manifest digest
- `verify="gait verify <run_id>"`: one-command integrity check

Use it to move from vague incident reports to reproducible artifacts.

## Incident To Regress

When an incident happens, convert it into a deterministic regression:

```bash
gait demo
gait regress init --from run_demo --json
gait regress run --json
```

Expected outcomes:

- `gait regress init` creates `gait.yaml` and `fixtures/<name>/runpack.zip`
- `gait regress run` returns `status=pass` or `status=fail` with stable exit codes

## Gate High-Risk Tools

Use Gate to enforce policy at the tool boundary before side effects.

Allow/block/approval flow (offline):

```bash
gait policy test examples/policy-test/allow.yaml examples/policy-test/intent.json --json
gait policy test examples/policy-test/block.yaml examples/policy-test/intent.json --json
gait policy test examples/policy-test/require_approval.yaml examples/policy-test/intent.json --json
```

Expected exit codes:

- `0`: allow
- `3`: block
- `4`: require approval

## Why Gate Exists (Enterprise Context)

In agent systems, **instructions** and **data** often collide:

- User or external content can smuggle tool-like instructions into prompts.
- If policy is not enforced at the execution boundary, privileged tools can run from untrusted context.

Gate addresses this by making tool execution depend on deterministic policy evaluation over typed intent.

Concrete blocked example:

```bash
gait policy test examples/prompt-injection/policy.yaml examples/prompt-injection/intent_injected.json --json
```

Expected result: verdict `block` with reason code `blocked_prompt_injection`.

## Offline Examples

See `/examples` for offline-safe, no-secrets examples:

- `/examples/stub-replay`
- `/examples/policy-test`
- `/examples/regress-run`
- `/examples/prompt-injection`
- `/examples/python` (thin adapter SDK path)

## Scope (v1)

Runpack, Regress, Gate, Doctor, and minimal adapter surfaces only.
