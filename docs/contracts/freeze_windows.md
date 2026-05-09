# Freeze Window Contract

Gate freeze windows let a policy apply a deterministic `block` or
`require_approval` outcome to production-impacting actions during operator-set
change freezes.

Rule shape:

```yaml
freeze_window:
  timezone: America/Toronto
  effect: block
  reason: quarter-end freeze
  environments: [prod]
  risk_classes: [high, critical]
  windows:
    - name: quarter-end
      start: "2026-03-10T09:00:00"
      end: "2026-03-10T12:00:00"
```

Contract details:

- `timezone` is an IANA timezone from policy config, not a caller-local guess.
- `windows[].start` and `windows[].end` use local wall-clock timestamps in
  `YYYY-MM-DDTHH:MM:SS` form and are interpreted in the configured timezone.
- `effect` must be `block` or `require_approval`.
- `environments` and `risk_classes` optionally narrow when the freeze applies.
- `gait gate eval --evaluation-time <rfc3339>` provides deterministic replay and
  fixture coverage without relying on wall-clock time.

Reason-code contract:

- `freeze_window_active_block`
- `freeze_window_active_require_approval`
- `freeze_window_invalid_timezone`
- `freeze_window_invalid_window`

Proof surfaces:

- `gait gate eval --json` returns `freeze_window` details when a rule carries
  freeze-window state.
- signed gate traces record the selected freeze-window decision under the
  `freeze_window` field.

Examples:

- `examples/policy/freeze_windows/production_block.yaml`
- `examples/policy/freeze_windows/production_require_approval.yaml`
- `examples/policy/freeze_windows/intent_prod_deploy.json`
