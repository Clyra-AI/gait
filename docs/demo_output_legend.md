# Demo Output Legend

This page explains the key fields emitted by `gait demo` and `gait tour`.

It also covers the observe-only `gait trace` wrapper output because that command is part of the same first-run onboarding surface.

## `gait demo` (Standard)

Example fields:

- `mode=standard`: demo branch used.
- `run_id=run_demo`: deterministic run identifier.
- `bundle=./gait-out/runpack_run_demo.zip`: generated runpack artifact path.
- `ticket_footer=...`: copyable audit footer for PRs/incidents.
- `verify=ok`: immediate artifact verification succeeded.
- `next=...`: deterministic follow-up commands.
- `metrics_opt_in=...`: optional local adoption telemetry env var.

## `gait demo --durable`

Additional fields:

- `job_id=...`: durable job id.
- `job_status=completed`: lifecycle ended successfully.
- `pack_path=...`: generated job pack path.
- `simulated_failure=resume blocked until approval`: shows approval gate behavior was exercised.

## `gait demo --policy`

Additional fields:

- `policy_verdict=block|allow|require_approval`: evaluation result for demo intent.
- `matched_rule=...`: policy rule that produced the decision.
- `reason_codes=...`: deterministic explanation codes.

## `gait tour`

Phase markers:

- `a1_demo=ok`: runpack demo step complete.
- `a2_verify=ok`: verification step complete.
- `a3_regress_init=ok`: regress fixture initialization complete.
- `a4_regress_run=pass`: regress execution complete.
- `branch_hints=...`: next branches to explore (`--durable`, `--policy`).

## `gait trace`

Example fields:

- `mode=trace`: observe-only wrapper mode.
- `child_exit_code=...`: child process exit code preserved by the wrapper.
- `trace_paths=[...]`: emitted trace references discovered from child output.
- `verdict_counts=[...]`: summary of emitted trace verdicts without changing child execution.

## How To Read Outcomes Safely

- `allow` -> execution may proceed.
- `block`, `require_approval`, `dry_run`, or eval error -> do not execute side effects.

## Related Docs

- `docs/scenarios/simple_agent_tool_boundary.md`
- `docs/flows.md`
- `README.md`
