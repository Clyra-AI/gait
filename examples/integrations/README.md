# Integration Adapter Parity

This directory contains framework adapters that demonstrate the same Gait execution contract across runtimes.

## Contract (Must Stay Identical)

Every adapter must implement:

1. Framework tool-call payload -> normalized intent payload.
2. `gait gate eval` using local policy + intent files.
3. Execute tool exactly once only on `allow`.
4. Persist deterministic trace/evidence paths for local debugging and CI.

Any verdict other than `allow` is fail-closed (`executed=false`).

## Adapter Acceptance Checklist

Each `examples/integrations/<adapter>/` folder must include:

- `README.md` with copy/paste allow/block commands and expected output fields.
- Runnable quickstart script.
- Deterministic artifact output paths under `gait-out/integrations/<adapter>/`.
- No raw side-effecting tool path exposed without Gate evaluation.

## Parity Rules

- No adapter-specific policy language, no policy forks.
- No framework gets a bypass path around Gate.
- No adapter-specific semantics for approval or block outcomes.
- New features must be added in a framework-neutral way first, then adopted by adapters.

## Validation

Run before opening a PR:

```bash
go build -o ./gait ./cmd/gait
make lint
make test
make test-adoption
```
