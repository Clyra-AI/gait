# Sandbox Policy Contract

Gate can now validate structured sandbox posture for high-risk `proc.exec` and
generated-code execution paths without treating wrapper-local prose claims as
authoritative.

Intent context shape:

```json
{
  "context": {
    "sandbox": {
      "network_mode": "egress_allowlist",
      "writable_paths": ["/tmp/work"],
      "read_only_roots": ["/repo", "/usr/share"],
      "env_exposure_mode": "allowlist",
      "timeout_seconds": 30,
      "filesystem_isolation": "container",
      "user_mode": "unprivileged",
      "evidence_ref": "sandbox:receipt:v1",
      "evidence_digest": "<sha256 hex>"
    }
  }
}
```

Policy rule shape:

```yaml
sandbox:
  allowed_network_modes: [disabled, egress_allowlist]
  allowed_writable_path_prefixes: [/tmp/work]
  required_read_only_roots: [/repo, /usr/share]
  allowed_env_exposure_modes: [none, allowlist]
  max_timeout_seconds: 60
  allowed_filesystem_isolations: [workspace, container]
  allowed_user_modes: [unprivileged]
```

Contract details:

- Gait validates metadata and evidence references; it does not implement the OS
  sandbox itself.
- Raw environment assignments and secret-like sandbox evidence refs are rejected
  during intent normalization.
- Signed traces and `gait gate eval --json` expose sandbox decision state and
  evidence digest/ref, not raw environment contents.

Reason-code contract:

- `sandbox_metadata_missing`
- `sandbox_evidence_missing`
- `sandbox_network_mode_missing`
- `sandbox_network_mode_disallowed`
- `sandbox_env_exposure_mode_missing`
- `sandbox_env_exposure_mode_disallowed`
- `sandbox_timeout_missing`
- `sandbox_timeout_exceeded`
- `sandbox_filesystem_isolation_missing`
- `sandbox_filesystem_isolation_disallowed`
- `sandbox_user_mode_missing`
- `sandbox_user_mode_disallowed`
- `sandbox_writable_path_disallowed`
- `sandbox_read_only_root_missing`

Examples:

- `examples/policy/sandbox/allow_sandboxed_exec.yaml`
- `examples/policy/sandbox/intent_proc_exec_valid.json`
- `examples/policy/sandbox/intent_proc_exec_missing_sandbox.json`
- `examples/policy/sandbox/intent_proc_exec_permissive_network.json`
