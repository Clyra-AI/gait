# Evidence Templates (Epic A7.2)

Use these templates to standardize evidence handoff in incidents, pull requests, and postmortems.

Conventions:

- Always include `run_id`.
- Always include an executable verify command: `gait verify <run_id>`.
- Prefer immutable digest references from `ticket_footer`.
- Regenerate footer from artifact when needed: `gait run receipt --from <run_id|path>`.

## Incident Ticket Template

```text
Title: [Agent Incident] <short summary>

Impact:
- User/system impact:
- Start time (UTC):
- Detection source:

Gait Evidence:
- run_id: <run_id>
- verify: gait verify <run_id>
- ticket_footer: GAIT run_id=<run_id> manifest=sha256:<digest> verify="gait verify <run_id>"
- gate_trace: trace_<trace_id>.json
- approval_audit (if applicable): approval_audit_<trace_id>.json

Reproduction:
1) gait regress init --from <run_id> --json
2) gait regress run --json --junit=./gait-out/junit.xml

Current Status:
- [ ] mitigated
- [ ] root cause identified
- [ ] regression added
```

## PR Description Template

```text
## Why
- Brief issue summary:

## Evidence
- run_id: <run_id>
- verify: gait verify <run_id>
- regress_result: gait regress run --json
- policy_result (if policy changed): gait policy test <policy.yaml> <intent.json> --json

## Change Scope
- Components touched:
- Risk class:

## Validation
- [ ] make lint
- [ ] make test
- [ ] gait verify <run_id>
- [ ] regress fixture updated or confirmed unchanged
```

## Postmortem Section Template

```text
### Deterministic Evidence
- run_id: <run_id>
- verify command: gait verify <run_id>
- manifest digest: sha256:<digest>
- associated regress fixture: fixtures/<fixture_name>/

### Enforcement Outcome
- gate verdict:
- reason codes:
- approval required: yes/no
- approval ref/token id (if used):

### Preventive Action
- Regression command: gait regress run --json
- Policy hardening changes:
- Owner + due date:
```
