# Hardening Release Checklist

Use this checklist before creating a release tag. Items marked "MANDATORY" are release-blocking.

## 1) Baseline Validation (MANDATORY)

- [ ] `make lint` passes on a clean checkout.
- [ ] `make test` passes with coverage gates:
  - Go coverage >= 85%
  - Python coverage >= 85%
- [ ] `make test-hardening-acceptance` passes.
- [ ] `make test-install-path-versions` passes.
- [ ] Docs and release-story gates pass:
  - `make test-docs-consistency`
  - `make docs-site-check`
  - `make test-release-smoke`
- [ ] Versioned acceptance/context gates pass:
  - `make test-v2-3-acceptance`
  - `make test-v2-4-acceptance`
  - `make test-v2-5-acceptance`
  - `make test-v2-6-acceptance`
  - `make test-context-conformance`
  - `make test-context-chaos`
- [ ] Boundary contract repro guards pass:
  - `go test ./cmd/gait -count=1 -run 'TestRunGateEvalRequiresVerifiedContextEnvelopeForContextPolicies|TestPolicyTestEqualPriorityRenamesDoNotChangeVerdict'`
  - `go test ./core/gate -count=1 -run 'TestEvaluatePolicyDetailedEqualPriorityRenameDoesNotChangeVerdict'`
  - `go test ./core/jobruntime -count=1 -run 'TestSubmitAppendFailureRollsBackNewJob|TestMutationAppendFailureRollsBackStateAndRetrySucceeds|TestMutationAppendFailureWithDurableEventPreservesPendingMarker'`
  - `cd sdk/python && PYTHONPATH=. uv run --python 3.13 --extra dev pytest tests/test_client.py -q -k 'capture_demo_runpack_uses_json_cli_contract or capture_demo_runpack_malformed_json_raises_command_error'`
- [ ] Full local UAT passes: `bash scripts/test_uat_local.sh`
  - verify `.uat_local/summary.txt` contains `UAT COMPLETE: PASS`
- [ ] CI `hardening` job is green on the release commit.

## 2) Contract Integrity (MANDATORY)

- [ ] Public CLI exit-code behavior is unchanged or intentionally documented.
- [ ] `--json` error envelope remains stable (`error_code`, `error_category`, `retryable`, `hint`).
- [ ] Schema changes are additive and versioned; no unplanned breaking changes in v1 artifacts.
- [ ] Golden tests for error envelopes and critical outputs are green.

## 3) Security and Privacy (MANDATORY)

- [ ] `gosec` and `govulncheck` pass with no unresolved critical findings.
- [ ] Credential broker safety controls verified:
  - command allowlist behavior
  - timeout/output-size bounds
  - no secret leakage in default CLI outputs
- [ ] Key source configuration checks pass (`doctor` and command-level validation).
- [ ] Unsafe operations retain explicit interlocks and fail-closed defaults.

## 4) Determinism and Artifact Safety (MANDATORY)

- [ ] Deterministic zip generation tests pass.
- [ ] Atomic write and lock contention tests pass.
- [ ] Registry retry/fallback behavior remains deterministic and trust-preserving.
- [ ] Trace/runpack verification passes on regenerated artifacts.

## 5) Supply Chain Integrity (MANDATORY)

- [ ] Release workflow tool versions are pinned.
- [ ] Release workflow gate jobs are green and release depends on all version gates:
  - `v2_3_gate`
  - `v2_4_gate`
  - `v2_5_gate`
  - `v2_6_gate`
  - `boundary_repro_gate`
  - `hardening_gate`
- [ ] Checksums generated and verified.
- [ ] Signatures/provenance artifacts generated and verifiable.
- [ ] Homebrew formula asset rendered from release checksums (`dist/gait.rb`).
- [ ] Release-owner Action Contract evidence bundle generated from the peeled
  release commit with a stable release-time Ed25519 key supplied through the
  protected `GAIT_RELEASE_SIGNING_SEED` secret (never a checked-in key); verify its manifest,
  lifecycle/typed-evidence signatures, schema digests, and authoritative
  (`authoritative=true`, `fixture_only=false`, `development_signing=false`,
  `quarantine=false`) markers before upload.
- [ ] The authoritative bundle zip, manifest, public key, and every referenced
  schema are uploaded as release assets.
- [ ] `publish-homebrew-tap` workflow job is green (or intentionally skipped with documented reason).
- [ ] Release workflow integrity verification steps complete successfully.

## 6) Operational Readiness (RECOMMENDED)

- [ ] `gait doctor --json` includes green checks for hooks, cache, lock staleness, temp writeability, and key-source ambiguity.
- [ ] `gait doctor --json` reports the invoked binary truthfully and only surfaces `path_binary_path` when a different PATH `gait` is present.
- [ ] `python3 scripts/check_github_action_runtime_versions.py .github/workflows docs/adopt_in_one_pr.md` passes, including `actions/setup-node`.
- [ ] Install and release docs point operators to `examples/config/oss_prod_template.yaml` and require `gait doctor --production-readiness --json` before claiming `oss-prod` readiness.
- [ ] README, launch docs, and docs-site public LLM mirrors all keep the same staged launch story:
  - fast proof
  - strict inline enforcement only at a real interception seam
  - hardened `oss-prod` readiness as a separate explicit gate
- [ ] Installer/Homebrew/manual verification uses `gait version --json` as the machine-readable binary probe.
- [ ] Correlation IDs and operational events are emitted in opt-in logs where enabled.
- [ ] Homebrew tap install/test smoke passes for the release:
  - `brew reinstall Clyra-AI/tap/gait`
  - `brew test Clyra-AI/tap/gait`
- [ ] Relevant hardening docs updated:
  - `docs/hardening/contracts.md`
  - `docs/hardening/risk_register.md`
  - framework alignment matrices

## 7) Release Decision

- [ ] Release manager sign-off (engineering owner)
- [ ] Security sign-off (if security-sensitive changes included)
- [ ] Go/No-Go recorded in release notes with:
  - decision date
  - evidence command list
  - residual non-blocking risks or `none`
