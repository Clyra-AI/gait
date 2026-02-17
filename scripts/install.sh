#!/usr/bin/env bash
set -euo pipefail

REPO_DEFAULT="Clyra-AI/gait"
INSTALL_DIR_DEFAULT="${HOME}/.local/bin"
VERSION_DEFAULT="latest"

usage() {
  cat <<'EOF'
Install Gait from signed GitHub release artifacts.

Usage:
  install.sh [--version <tag>] [--repo <owner/name>] [--install-dir <path>]

Options:
  --version <tag>      Release tag (default: latest)
  --repo <owner/name>  GitHub repository (default: Clyra-AI/gait)
  --install-dir <path> Binary install directory (default: ~/.local/bin)
  -h, --help           Show this help

Environment overrides:
  GAIT_RELEASE_BASE_URL  Override release base URL (advanced/testing)

Examples:
  ./scripts/install.sh
  ./scripts/install.sh --version vX.Y.Z
  ./scripts/install.sh --install-dir /usr/local/bin
EOF
}

download() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO- "$url"
    return
  fi
  echo "error: curl or wget is required" >&2
  exit 2
}

download_file() {
  local url="$1"
  local out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$out"
    return
  fi
  echo "error: curl or wget is required" >&2
  exit 2
}

sha256_file() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$path" | awk '{print $1}'
    return
  fi
  echo "error: sha256sum or shasum is required" >&2
  exit 2
}

detect_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)
      echo "error: unsupported operating system: $(uname -s)" >&2
      echo "supported: linux, darwin" >&2
      exit 2
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "error: unsupported architecture: $(uname -m)" >&2
      echo "supported: amd64, arm64" >&2
      exit 2
      ;;
  esac
}

path_contains_dir() {
  local dir="$1"
  case ":$PATH:" in
    *":${dir}:"*) return 0 ;;
    *) return 1 ;;
  esac
}

print_path_hint() {
  local dir="$1"
  local shell_name="${SHELL##*/}"
  case "$shell_name" in
    zsh)
      echo "add PATH for zsh:"
      echo "  echo 'export PATH=\"${dir}:\$PATH\"' >> ~/.zshrc"
      echo "  source ~/.zshrc"
      ;;
    bash)
      local rc_file="$HOME/.bashrc"
      if [[ -f "$HOME/.bash_profile" ]]; then
        rc_file="$HOME/.bash_profile"
      fi
      echo "add PATH for bash:"
      echo "  echo 'export PATH=\"${dir}:\$PATH\"' >> ${rc_file}"
      echo "  source ${rc_file}"
      ;;
    fish)
      echo "add PATH for fish:"
      echo "  set -U fish_user_paths ${dir} \$fish_user_paths"
      ;;
    *)
      echo "add PATH for your shell:"
      echo "  export PATH=\"${dir}:\$PATH\""
      ;;
  esac
}

repo="$REPO_DEFAULT"
version="$VERSION_DEFAULT"
install_dir="$INSTALL_DIR_DEFAULT"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || { echo "error: --version requires a value" >&2; exit 2; }
      version="$2"
      shift 2
      ;;
    --repo)
      [[ $# -ge 2 ]] || { echo "error: --repo requires a value" >&2; exit 2; }
      repo="$2"
      shift 2
      ;;
    --install-dir)
      [[ $# -ge 2 ]] || { echo "error: --install-dir requires a value" >&2; exit 2; }
      install_dir="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "$version" == "latest" ]]; then
  latest_api="https://api.github.com/repos/${repo}/releases/latest"
  version="$(download "$latest_api" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  if [[ -z "$version" ]]; then
    echo "error: could not resolve latest release tag from ${latest_api}" >&2
    exit 2
  fi
fi

os="$(detect_os)"
arch="$(detect_arch)"

version_normalized="${version#v}"
asset_candidates=(
  "gait_${version}_${os}_${arch}.tar.gz"
)
if [[ "${version_normalized}" != "${version}" ]]; then
  asset_candidates+=("gait_${version_normalized}_${os}_${arch}.tar.gz")
fi
release_base_url="${GAIT_RELEASE_BASE_URL:-https://github.com/${repo}/releases/download/${version}}"

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

checksums_path="${work_dir}/checksums.txt"

echo "==> Downloading checksums: ${release_base_url}/checksums.txt"
download_file "${release_base_url}/checksums.txt" "$checksums_path"

asset_name=""
for candidate in "${asset_candidates[@]}"; do
  if awk -v file="$candidate" '$2 == file {found=1} END {exit found ? 0 : 1}' "$checksums_path"; then
    asset_name="$candidate"
    break
  fi
done

if [[ -z "$asset_name" ]]; then
  echo "error: checksum entry not found for any candidate asset name" >&2
  echo "tried:" >&2
  for candidate in "${asset_candidates[@]}"; do
    echo "  - ${candidate}" >&2
  done
  exit 2
fi

asset_path="${work_dir}/${asset_name}"

echo "==> Downloading asset: ${release_base_url}/${asset_name}"
download_file "${release_base_url}/${asset_name}" "$asset_path"

expected="$(awk -v file="$asset_name" '$2 == file {print $1}' "$checksums_path")"

actual="$(sha256_file "$asset_path")"
if [[ "$actual" != "$expected" ]]; then
  echo "error: checksum mismatch for ${asset_name}" >&2
  echo "expected: ${expected}" >&2
  echo "actual:   ${actual}" >&2
  exit 2
fi

extract_dir="${work_dir}/extract"
mkdir -p "$extract_dir"
tar -xzf "$asset_path" -C "$extract_dir"

bin_src="${extract_dir}/gait"
if [[ ! -f "$bin_src" ]]; then
  echo "error: extracted binary not found: ${bin_src}" >&2
  exit 2
fi

mkdir -p "$install_dir"
bin_dst="${install_dir}/gait"
cp "$bin_src" "$bin_dst"
chmod 0755 "$bin_dst"

echo "==> Installed: ${bin_dst}"
if command -v gait >/dev/null 2>&1; then
  echo "==> gait on PATH: $(command -v gait)"
elif path_contains_dir "$install_dir"; then
  echo "==> ${install_dir} is on PATH. Open a new shell and run: gait --version"
else
  echo "note: ${install_dir} is not on PATH for this shell session."
  print_path_hint "$install_dir"
fi

echo "==> Next steps"
echo "gait doctor --json"
echo "gait demo"
echo "gait verify run_demo"
