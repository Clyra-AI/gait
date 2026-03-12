# Hardening Risk Register (Epic H0.3)

This register tracks the highest-risk runtime and operational failure modes for Gait.

| ID | Failure Mode | Severity | Likelihood | Mitigation Epic | Owner | Target Milestone |
| --- | --- | --- | --- | --- | --- | --- |
| HR-01 | Operational failures reported as `invalid_input` | High | High | `H1` | CLI maintainer | v1.6 |
| HR-02 | Partial write corruption or silent divergence on critical job state files | High | Medium | `H2`, `W2` | Core runtime maintainer | v1.6 |
| HR-03 | Lock contention causes nondeterministic behavior | High | Medium | `H3` | Gate maintainer | v1.6 |
| HR-04 | Remote transient failures fail immediately without retry | Medium | Medium | `H4` | Registry maintainer | v1.7 |
| HR-05 | Stale lock files degrade operator confidence | Medium | Medium | `H3`, `H5` | Gate maintainer | v1.7 |
| HR-06 | Hook policy not enabled on contributor machines | Medium | High | `H6` | Repo stewardship | v1.6 |
| HR-07 | Error envelope drift breaks automation parsers | High | Medium | `H1`, `H6` | CLI maintainer | v1.6 |
| HR-08 | Integrity checks skipped in release pipeline edge cases | High | Low | `H7`, `H12` | Release owner | v1.7 |
| HR-09 | Misconfigured key sources produce ambiguous runtime behavior | High | Medium | `H8` | Security maintainer | v1.7 |
| HR-10 | Resource pressure causes degraded reliability | Medium | Medium | `H11` | Performance owner | v1.8 |

## Register Policy

- Update this register whenever a hardening story changes risk posture.
- Keep one accountable owner per risk.
- Do not close a risk without test evidence or explicit exception rationale.
