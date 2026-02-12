# UI Contract (Localhost)

Status: informative for v2.3 UI surface.

The localhost UI is an orchestration layer over CLI commands. It must remain contract-compatible with existing CLI and artifact schemas.

## API Endpoints

- `GET /api/health`
- `GET /api/state`
- `POST /api/exec`

## `POST /api/exec` request

```json
{
  "command": "demo",
  "args": {}
}
```

Allowed `command` values:

- `demo`
- `verify_demo`
- `receipt_demo`
- `regress_init`
- `regress_run`
- `policy_block_test`

Arbitrary command execution is not allowed.

## `POST /api/exec` response

```json
{
  "ok": true,
  "command": "demo",
  "argv": ["/path/to/gait", "demo", "--json"],
  "exit_code": 0,
  "duration_ms": 123,
  "stdout": "{...}",
  "stderr": "",
  "json": {
    "ok": true,
    "run_id": "run_demo"
  }
}
```

## Invariants

- UI must call local `gait` binary, never reimplement policy/verify logic.
- UI must preserve exit code and reason visibility.
- UI must not imply execution success on non-`allow` policy outcomes.
- Core artifact contracts remain those defined in `docs/contracts/primitive_contract.md`.
