# Install Gait

Gait's default install path is the release installer script.

## Platform Support

- `scripts/install.sh` currently supports Linux and macOS.
- Windows is supported through release assets (manual download + PATH setup), not the installer script.
- Homebrew publishing is tap-first and release-gated; see `docs/homebrew.md`.

## Recommended Path (Binary + Checksum Verification)

```bash
curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/install.sh | bash
```

What the script does:

- resolves the latest release tag (or uses `--version`)
- downloads `checksums.txt` and your platform archive
- verifies SHA-256 checksum before install
- installs `gait` to `~/.local/bin` by default

## Alternate Path: Homebrew (Tap)

```bash
brew tap davidahmann/tap
brew install gait
```

Validate install:

```bash
brew test davidahmann/tap/gait
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

If PATH is still not updated, run directly once:

```bash
~/.local/bin/gait doctor --json
```

## One-Command Quickstart

After installation:

```bash
curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/quickstart.sh | bash
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
git clone https://github.com/davidahmann/gait.git
cd gait
go build -o ./gait ./cmd/gait
```
