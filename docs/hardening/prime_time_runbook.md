# Prime-Time Hardening Runbook (OSS)

Use this runbook to deploy Gait as a hardened execution boundary for production OSS usage.

## 1) Configure Hardened Defaults

Create project config from template:

```bash
mkdir -p .gait
cp examples/config/oss_prod_template.yaml .gait/config.yaml
```

Set secrets in environment:

- `GAIT_PRIVATE_KEY`
- `GAIT_MCP_TOKEN`
- broker token variables (`GAIT_BROKER_TOKEN_*`) when broker mode is enabled

## 2) Validate Production Readiness

```bash
gait doctor --production-readiness --json
```

Must return:

- `ok=true`
- `status=pass`
- `checks[].name=job_runtime_durability` with `status=pass`

## 3) Start Hardened MCP Service

```bash
gait mcp serve \
  --policy examples/policy/base_high_risk.yaml \
  --profile oss-prod \
  --listen 127.0.0.1:8787 \
  --auth-mode token \
  --auth-token-env GAIT_MCP_TOKEN \
  --max-request-bytes 1048576 \
  --http-verdict-status strict \
  --trace-dir ./gait-out/mcp-serve/traces \
  --runpack-dir ./gait-out/mcp-serve/runpacks \
  --session-dir ./gait-out/mcp-serve/sessions \
  --trace-max-age 168h \
  --trace-max-count 50000 \
  --runpack-max-age 336h \
  --runpack-max-count 20000 \
  --session-max-age 336h \
  --session-max-count 20000
```

## 4) Must-Pass Hardening Gates

```bash
make test-hardening-acceptance
make test-chaos
bash scripts/test_session_soak.sh
make bench-budgets
```

## 5) Operational Guardrails

- Keep `--http-verdict-status strict` for service callers that can handle non-2xx on non-allow.
- Keep `allow_client_artifact_paths=false` in production.
- Set `GAIT_TELEMETRY_HEALTH_PATH` to monitor telemetry write degradation.
- Keep session lock profile on `standard` unless operating swarms; use `swarm` only with contention evidence.

## 6) Incident Escalation Flow

1. Capture and verify artifacts:
   - `gait verify <run|path> --json`
   - `gait trace verify <trace.json> --json`
2. Check durable job health before manual cleanup:
   - `gait doctor --json`
   - `gait job inspect --id <job_id> --root ./gait-out/jobs --json`
3. Package evidence:
   - `gait guard pack --run <run_id> --json`
   - `gait incident pack --from <run_id> --window 24h --json`
4. Convert to regression:
   - `gait regress bootstrap --from <run_id> --json`
