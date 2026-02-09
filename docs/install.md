# Install Gait

Gait's default install path is the release installer script.

## Recommended Path (Binary + Checksum Verification)

```bash
curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/install.sh | bash
```

What the script does:

- resolves the latest release tag (or uses `--version`)
- downloads `checksums.txt` and your platform archive
- verifies SHA-256 checksum before install
- installs `gait` to `~/.local/bin` by default

Options:

```bash
bash scripts/install.sh --version v1.7.0 --install-dir ~/.local/bin
```

## Verify Installation

```bash
gait doctor --json
gait demo
gait verify run_demo
```

## One-Command Quickstart

After installation:

```bash
curl -fsSL https://raw.githubusercontent.com/davidahmann/gait/main/scripts/quickstart.sh | bash
```

## Advanced: Build From Source

Use this only when developing Gait itself.

```bash
git clone https://github.com/davidahmann/gait.git
cd gait
go build -o ./gait ./cmd/gait
```
