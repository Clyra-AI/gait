# Homebrew Publishing (Tap-First)

This doc defines the Homebrew strategy for Gait.

## Position

- GitHub Releases are the release source of truth.
- Homebrew is a distribution adapter, not the release system.
- Publish to a custom tap first (`homebrew/core` later, only after stability proof).
- Current tap repo: `davidahmann/homebrew-tap` (tap alias `davidahmann/tap`).

## Preconditions (Release Gate)

Before updating a tap formula:

- release tag is published and signed (`vX.Y.Z`)
- integrity assets are present in release:
  - `checksums.txt`
  - `checksums.txt.sig`
  - `checksums.txt.pem`
  - `checksums.txt.intoto.jsonl`
  - `sbom.spdx.json`
  - `provenance.json`
- install smoke jobs are green (macOS + Linux)
- no install-path or CLI contract churn in the release

Reference: `docs/hardening/release_checklist.md`

## Naming Check

Run these before deciding final formula naming:

```bash
brew search '^gait$'
brew info gait
```

If `gait` is taken in your target ecosystem, use a prefixed tap formula name (for example `gait-cli`) and keep docs explicit.

## Tap Update Workflow

1. Cut release in this repo (`vX.Y.Z`).
2. Download release `checksums.txt` or use local `dist/checksums.txt`.
3. Render formula deterministically:

```bash
bash scripts/render_homebrew_formula.sh \
  --repo davidahmann/gait \
  --version vX.Y.Z \
  --checksums dist/checksums.txt \
  --out Formula/gait.rb
```

4. Commit formula in tap repo (`davidahmann/homebrew-tap`, `Formula/gait.rb`).
5. Open PR (or direct push if you own the tap), require macOS verification green.
6. Merge and verify:

```bash
brew update
brew tap davidahmann/tap
brew reinstall davidahmann/tap/gait
brew test davidahmann/tap/gait
gait demo --json
```

## Rollback

If a formula release is bad:

1. Revert tap formula to prior known-good tag.
2. Merge rollback PR immediately.
3. Mark broken release as superseded in release notes.
4. Open follow-up issue for root cause + release gate hardening.
