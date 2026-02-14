# v2.6 Content Cadence Plan

Date: 2026-02-14
Horizon: one quarter

## Cadence

- 2 technical posts per month
- 1 integration case study per month
- 1 short "incident->regress" demo asset refresh per month

## Themes

1. Incident to deterministic CI gate
- topic: `gait regress bootstrap`
- artifact: reproducible fixture + JUnit
2. Durable jobs lifecycle
- topic: checkpoint, approval, resume, pack verify
- artifact: `gait demo --durable` walkthrough
3. Policy rollout without outage
- topic: observe (`--simulate`) to enforce
- artifact: `gait demo --policy` walkthrough
4. Context evidence and trust chain
- topic: context-proof and conformance gates
- artifact: trace/pack verification examples

## Distribution

- README changelog highlights
- docs-site updates
- blog posts in `docs/blog/`
- integration examples in `examples/integrations/*`

## Exit Criteria

- each monthly cycle ships:
- at least one reproducible command sequence
- one CI-ready example
- one updated docs page linked from `docs/README.md`
