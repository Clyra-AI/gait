---
title: "Install Gait"
description: "Step-by-step installation for Linux, macOS, and Windows with checksum verification and PATH setup."
---

# Install Gait

Gait's default install path is the release installer script.

## Platform Support

- `scripts/install.sh` currently supports Linux and macOS.
- Windows is supported through release assets (manual download + PATH setup), not the installer script.
- Homebrew publishing is tap-first and release-gated; see `docs/homebrew.md`.

## Recommended Path (Binary + Checksum Verification)

```bash
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/install.sh | bash
```

What the script does:

- resolves the latest release tag (or uses `--version`)
- downloads `checksums.txt` and your platform archive
- verifies SHA-256 checksum before install
- installs `gait` to `~/.local/bin` by default

## Alternate Path: Homebrew (Tap)

```bash
brew tap Clyra-AI/tap
brew install gait
```

Validate install:

```bash
brew test Clyra-AI/tap/gait
gait demo --json
```

## PATH Setup (Common First-Run Fix)

If `gait` is not found after install, add `~/.local/bin` to your shell PATH:

For `zsh`:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

For `bash`:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

For `fish`:

```fish
set -U fish_user_paths $HOME/.local/bin $fish_user_paths
```

Options:

```bash
bash scripts/install.sh --version vX.Y.Z --install-dir ~/.local/bin
```

## Verify Installation

```bash
gait doctor --json
gait demo
gait verify run_demo
```

## Dev vs Prod Mode (Important)

Use development mode for first-run validation:

```bash
gait demo
gait verify run_demo
```

Before production use, apply hardened defaults and validate readiness:

```bash
mkdir -p .gait
gait policy init baseline-highrisk --out .gait/policy.yaml
cat > .gait/config.yaml <<'YAML'
gate:
  policy: .gait/policy.yaml
  profile: oss-prod
  key_mode: prod
  private_key_env: GAIT_PRIVATE_KEY
  credential_broker: env
  credential_env_prefix: GAIT_BROKER_TOKEN_
  rate_limit_state: ./gait-out/gate_rate_limits.json

mcp_serve:
  enabled: true
  listen: 127.0.0.1:8787
  auth_mode: token
  auth_token_env: GAIT_MCP_TOKEN
  max_request_bytes: 1048576
  http_verdict_status: strict
  allow_client_artifact_paths: false

retention:
  trace_ttl: 168h
  session_ttl: 336h
  export_ttl: 168h
YAML
gait doctor --production-readiness --json
```

Production readiness must report `ok=true` before enforcing high-risk runtime boundaries.

If PATH is still not updated, run directly once:

```bash
~/.local/bin/gait doctor --json
```

## One-Command Quickstart

After installation:

```bash
curl -fsSL https://raw.githubusercontent.com/Clyra-AI/gait/main/scripts/quickstart.sh | bash
```

## Windows Install (Manual Path)

1. Download the latest Windows release asset from GitHub Releases.
2. Place `gait.exe` in a local tools directory (for example `%USERPROFILE%\\bin`).
3. Add that directory to your user PATH.
4. Open a new shell and run:

```powershell
gait.exe doctor --json
gait.exe demo
gait.exe verify run_demo
```

## Alternate Path: Build From Source

Use this only when developing Gait itself.

```bash
git clone https://github.com/Clyra-AI/gait.git
cd gait
go build -o ./gait ./cmd/gait
```

## Frequently Asked Questions

### Does Gait require Go to be installed?

No. The install script downloads a prebuilt binary. Go is only needed if building from source.

### Does Gait work on Windows?

Yes. Download the Windows binary from the GitHub release and add it to your PATH.

### How do I verify the install worked?

Run `gait doctor --json`. All checks should pass. Then run `gait demo` to create your first signed artifact.

### Can I install via Homebrew?

Yes. See `docs/homebrew.md` for tap-based installation as an alternate path.

### How large is the Gait binary?

The compiled binary is a single static Go executable, typically under 30 MB with zero runtime dependencies.
