# Project Defaults (Lightweight Config)

Use a local project config to avoid repeating the same `gait gate eval` flags.

Default lookup path:

- `.gait/config.yaml`

Command flags:

- `--config <path>` to load a custom config file
- `--no-config` to disable config lookup for one command

## Minimal Example

```yaml
gate:
  policy: examples/policy/base_high_risk.yaml
  profile: oss-prod
  key_mode: prod
  private_key: examples/scenarios/keys/approval_private.key
  wrkr_inventory_path: ./.gait/wrkr_inventory.json
  credential_broker: stub
```

With this file present, the repeated command becomes:

```bash
gait gate eval --intent examples/policy/intents/intent_delete.json --json
```

## Supported `gate` Defaults

- `policy`
- `profile`
- `key_mode`
- `private_key`
- `private_key_env`
- `approval_public_key`
- `approval_public_key_env`
- `approval_private_key`
- `approval_private_key_env`
- `rate_limit_state`
- `credential_broker`
- `credential_env_prefix`
- `credential_ref`
- `credential_scopes`
- `credential_command`
- `credential_command_args`
- `credential_evidence_path`
- `trace_path`
- `wrkr_inventory_path`

## Guardrails

- CLI flags always override config values.
- Missing default `.gait/config.yaml` is ignored.
- Missing explicit `--config <path>` returns an input error.
- Keep sensitive key files out of git and prefer `*_env` options in shared repos.

## Hardened OSS Template

Use the hardened template for production starts:

```bash
mkdir -p .gait
cp examples/config/oss_prod_template.yaml .gait/config.yaml
```

Template path:

- `examples/config/oss_prod_template.yaml`

## Migration Notes (Permissive -> Strict)

1. Set `gate.profile: oss-prod`.
2. Set `gate.key_mode: prod` and move key material to env-backed sources.
3. Configure `mcp_serve` with token auth and strict verdict status.
4. Add retention TTL values (`trace_ttl`, `session_ttl`, `export_ttl`).
5. Validate with `gait doctor --production-readiness --json`.
