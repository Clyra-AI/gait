# v2.6 Hero Demo Asset Review

Date: 2026-02-14

## Scope

Review whether README/docs-site hero terminal demo reflects current activation path:

- guided first-session flow (`gait tour`)
- durable branch (`gait demo --durable`)
- policy branch (`gait demo --policy`)

## Decision

Keep existing hero GIF for now and extend recording script support for activation-focused capture.

Rationale:

- existing asset still demonstrates offline proof (`demo -> verify -> replay -> regress`)
- v2.6 introduces additional branches; script should support these variants without forcing immediate asset churn

## Follow-up

- script now supports profile-driven capture (runpack-first vs activation-focused)
- next asset refresh should use activation profile after one iteration of copy/runtime validation
- replace README/docs-site hero asset when refreshed capture is reviewed and deterministic
