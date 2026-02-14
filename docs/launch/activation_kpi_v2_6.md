# v2.6 Activation KPI Definition

Date: 2026-02-14

## Goal

Measure whether v2.6 onboarding changes reduce time-to-value from first command to first CI-grade regression.

## Data Source

- local adoption events (`GAIT_ADOPTION_LOG`)
- report command:

```bash
gait doctor adoption --from ./gait-out/adoption.jsonl --json
```

## Milestones

- `A1`: first successful `gait demo` or `gait tour`
- `A2`: first successful `gait verify run_demo`
- `A3`: first successful `gait regress init --from run_demo`
- `A4`: first successful `gait regress run`

Durable and policy branch milestones:

- `A3D`: first successful `gait demo --durable`
- `A3P`: first successful `gait demo --policy`

## Primary Metrics

- median A1->A4 elapsed time (`activation_timing_ms`)
- completion rate of A1->A4 within first session
- completion rate of A3D and A3P within first 24h of installation

## Comparison Window

- pre-v2.6 baseline: logs collected before 2026-02-14
- post-v2.6 window: logs collected on/after 2026-02-14
- evaluation horizon: first 28 days after rollout

## Interpretation Rules

- success if median A1->A4 <= 30 minutes
- success if A3D completion increases vs baseline
- success if A3P completion increases vs baseline
- no metric is considered valid unless sample size >= 30 sessions
