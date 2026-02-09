# Launch FAQ: Common Objections

## "Why not just use built-in controls from one framework?"

Framework-native controls help, but most production stacks are multi-framework and multi-model.  
Gait keeps one execution-boundary and artifact contract across all of them.

## "Is this another prompt-injection scanner?"

No. Gait enforces policy at tool-call execution boundary.  
Prompt text can be hostile; execution decisions must be structured and deterministic.

## "Will this add too much latency?"

Runtime budgets are measured and enforced (`make bench-budgets`).  
Gate p95/p99 thresholds are tracked in CI and fail if they regress.

## "Do we need a hosted service?"

No for core workflows.  
Runpack, Gate, Regress, and Doctor all run offline-first with local artifacts.

## "Can we fail-open for velocity?"

Default posture for high-risk paths is fail-closed.  
Staged rollout exists via `--simulate`; enforcement modes should not silently bypass policy.

## "Are we locked into Gait-specific infra?"

No. Artifacts are files, schemas are versioned, and integration surface is CLI + JSON.  
Outputs are designed to move through existing CI, ticketing, and compliance workflows.
