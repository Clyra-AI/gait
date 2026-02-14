# Launch Kit

This folder is the repeatable distribution package for OSS launch cycles.

Use it when shipping a major release, announcing a wedge milestone, or running a re-launch.

## Contents

- `narrative_one_liner.md`: positioning statements by audience
- `hn_post.md`: Hacker News launch post + first comment template
- `github_release_template.md`: release page structure and copy
- `faq_objections.md`: objection handling
- `kpi_scorecard.md`: launch metrics and thresholds (`M*`, `C*`, `D*`)
- `activation_kpi_v2_6.md`: activation milestone metrics (`A1..A4`, durable/policy branches)
- `content_cadence_v2_6.md`: quarterly content plan for activation themes
- `hero_demo_asset_review_v2_6.md`: hero GIF/script alignment decision log
- framework RFC/runbook docs under `docs/launch/`

## Standard Adoption Proof Bundle (v2.3)

Bundle contents:

- sample runpack
- verify output
- regress result
- CI JUnit
- intent+receipt conformance report
- integration lane scorecard
- v2.3 metrics snapshot

Canonical generator:

```bash
go build -o ./gait ./cmd/gait
bash scripts/test_v2_3_acceptance.sh ./gait
```

Generated artifacts:

- `gait-out/runpack_run_demo.zip` (or quickstart workspace equivalent)
- `gait-out/v2_3_metrics_snapshot.json`
- `gait-out/integration_lane_scorecard.json`
- `gait-out/adoption_regress/regress_result.json` (CI template path)

## Suggested Launch Sequence

1. run `scripts/demo_90s.sh`
2. run `scripts/test_v2_3_acceptance.sh ./gait` and confirm `release_gate_passed=true`
3. generate ecosystem release notes:

```bash
python3 scripts/render_ecosystem_release_notes.py \
  docs/ecosystem/community_index.json \
  gait-out/ecosystem_release_notes.md \
  gait-out/v2_3_metrics_snapshot.json
```

4. publish release using `github_release_template.md`
5. post distribution threads and monitor `kpi_scorecard.md`
