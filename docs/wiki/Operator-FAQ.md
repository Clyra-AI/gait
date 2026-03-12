# Operator FAQ

## Why Gate exists

To enforce action boundaries at execution time, not after the fact.
Gate evaluates intent, policy, approvals, and provenance before side effects execute.

## Is Gait offline-first?

Yes for core workflows. Policy bootstrap, recording, verifying, diffing, replay, policy test, and regression run locally.

## Is fail-open supported?

Default is fail-closed for high-risk paths. Simulation mode exists for staged rollout.

## How do we prove what happened?

Use traces, runpacks, verification output, and ticket footer artifacts. These are deterministic and schema-validated.

## How do we prevent framework lock-in?

Use one contract across adapters. No framework receives privileged bypass behavior.
Gait is not a framework replacement; it owns the execution verdict and evidence contract while the framework keeps orchestration.

## How do we keep reports deterministic?

- Use `--json` outputs.
- Preserve canonical artifacts under `gait-out/`.
- Run regress fixtures in CI.

## Where are enterprise features?

OSS focuses on execution substrate and local control loops.
Fleet control-plane concerns stay in enterprise planning docs, not OSS runtime path.
