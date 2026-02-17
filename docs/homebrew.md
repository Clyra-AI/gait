# Homebrew Publishing (Tap-First)

This doc defines the Homebrew strategy for Gait.

## Position

- GitHub Releases are the release source of truth.
- Homebrew is a distribution adapter, not the release system.
- Publish to a custom tap first (`homebrew/core` later, only after stability proof).
- Current tap repo: `Clyra-AI/homebrew-tap` (tap alias `Clyra-AI/tap`).

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
  --repo Clyra-AI/gait \
  --version vX.Y.Z \
  --checksums dist/checksums.txt \
  --out Formula/gait.rb
```

4. Commit formula in tap repo (`Clyra-AI/homebrew-tap`, `Formula/gait.rb`).
5. Open PR (or direct push if you own the tap), require macOS verification green.
6. Merge and verify:

```bash
brew update
brew tap Clyra-AI/tap
brew reinstall Clyra-AI/tap/gait
brew test Clyra-AI/tap/gait
gait demo --json
```

## Tag-Driven Automation (Current Path)

`release.yml` includes a `publish-homebrew-tap` job that runs on version tags after release artifacts are published.

Required repository secret in `Clyra-AI/gait`:

- `HOMEBREW_TAP_TOKEN`: fine-grained token with `contents: write` on `Clyra-AI/homebrew-tap`

Behavior:

- Downloads `checksums.txt` from the tagged release
- Renders `Formula/gait.rb` deterministically
- Commits and pushes only when formula content changes
- Retries on transient GitHub API throttling

Manual fallback is still supported via `scripts/publish_homebrew_tap.sh`.

## Rollback

If a formula release is bad:

1. Revert tap formula to prior known-good tag.
2. Merge rollback PR immediately.
3. Mark broken release as superseded in release notes.
4. Open follow-up issue for root cause + release gate hardening.
