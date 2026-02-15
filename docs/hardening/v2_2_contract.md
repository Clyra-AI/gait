# Hardening Contract (OSS)

This contract defines hardening behavior required for OSS production posture.

Version note: this page is evergreen. The hardening baseline was introduced in `v2.2`, while release-by-release rollout details belong in release plans/changelog docs.

## Scope

Applies to OSS runtime boundary paths only:

- `gait gate eval`
- `gait mcp proxy`
- `gait mcp serve`
- session capture (`gait run session *`)
- operational/adoption telemetry writes

## Production vs Development Defaults

Development convenience:

- `gate.profile=standard`
- `mcp_serve.auth_mode=off` on loopback only
- no retention caps

Production posture:

- `gate.profile=oss-prod`
- `gate.key_mode=prod`
- `mcp_serve.auth_mode=token`
- `mcp_serve.http_verdict_status=strict`
- `mcp_serve.allow_client_artifact_paths=false`
- bounded request size and retention policies configured

Use `gait doctor --production-readiness --json` as the gate.

## Runtime Boundary Requirements

`mcp serve` boundary hardening requirements:

- non-loopback listen requires token auth
- request bodies are bounded by `max_request_bytes`
- non-allow verdicts can map to non-2xx with `--http-verdict-status strict`
- caller-controlled artifact output paths are disabled by default
- optional retention rotation for trace/runpack/session artifacts

## Session Durability Requirements

Session append behavior must be crash-tolerant and contention-safe:

- append path uses lock-protected state index (`*.state.json`)
- sequence and checkpoint linkage remain deterministic
- compaction can prune checkpointed events without breaking chain verification
- lock contention diagnostics are structured and tunable by env:
  - `GAIT_SESSION_LOCK_PROFILE`
  - `GAIT_SESSION_LOCK_TIMEOUT`
  - `GAIT_SESSION_LOCK_RETRY`
  - `GAIT_SESSION_LOCK_STALE_AFTER`

## Compatibility Contract

v2.2 changes are additive in `v1.x`:

- `TraceRecord` additive fields: `event_id`, `observed_at`
- service responses keep compat mode, strict mode is opt-in
- existing parsers must ignore unknown fields

## Required Validation Before Release

- `make test`
- `make test-hardening-acceptance`
- `bash scripts/test_session_soak.sh`
- `make test-chaos`
- `make bench-budgets`
