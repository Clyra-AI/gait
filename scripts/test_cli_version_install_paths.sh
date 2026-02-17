#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

SKIP_BREW="false"
BIN_INPUT=""

usage() {
  cat <<'EOF'
Validate CLI version behavior across install paths.

Usage:
  test_cli_version_install_paths.sh [--skip-brew] [path-to-gait-binary]

Options:
  --skip-brew  Skip Homebrew path verification.
  -h, --help   Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-brew)
      SKIP_BREW="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
    *)
      if [[ -n "${BIN_INPUT}" ]]; then
        echo "usage: $0 [--skip-brew] [path-to-gait-binary]" >&2
        exit 2
      fi
      BIN_INPUT="$1"
      shift
      ;;
  esac
done

if [[ -n "${BIN_INPUT}" ]]; then
  if [[ "${BIN_INPUT}" = /* ]]; then
    BIN_PATH="${BIN_INPUT}"
  else
    BIN_PATH="$(pwd)/${BIN_INPUT}"
  fi
else
  BIN_PATH="${REPO_ROOT}/gait"
  go build -o "${BIN_PATH}" ./cmd/gait
fi

if [[ ! -x "${BIN_PATH}" ]]; then
  echo "binary is not executable: ${BIN_PATH}" >&2
  exit 2
fi

if [[ "${SKIP_BREW}" != "true" ]] && ! command -v brew >/dev/null 2>&1; then
  echo "brew is required for install-path version smoke (or pass --skip-brew)" >&2
  exit 2
fi

detect_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)
      echo "unsupported OS for install-path version smoke: $(uname -s)" >&2
      exit 2
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "unsupported architecture for install-path version smoke: $(uname -m)" >&2
      exit 2
      ;;
  esac
}

sha256_file() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{print $1}'
    return
  fi
  shasum -a 256 "$path" | awk '{print $1}'
}

extract_version() {
  local bin="$1"
  "$bin" --version | awk 'NR==1{print $2}'
}

assert_version() {
  local label="$1"
  local bin="$2"
  local expected="$3"
  local got
  got="$(extract_version "$bin")"
  if [[ "$got" != "$expected" ]]; then
    echo "${label}: expected version ${expected}, got ${got}" >&2
    exit 1
  fi
  echo "${label}: version ${got}"
}

os="$(detect_os)"
arch="$(detect_arch)"
release_version_tag="v9.9.9-test"
release_version="${release_version_tag#v}"

work_dir="$(mktemp -d)"
tap_name=""
cleanup() {
  if [[ -n "${tap_name}" ]]; then
    brew uninstall --formula "${tap_name}/gait-local" >/dev/null 2>&1 || true
    brew untap "${tap_name}" >/dev/null 2>&1 || true
  fi
  rm -rf "${work_dir}"
}
trap cleanup EXIT

release_dir="${work_dir}/release"
install_dir="${work_dir}/install/bin"
mkdir -p "${release_dir}" "${install_dir}"

source_bin="${work_dir}/gait-source"
cp "${BIN_PATH}" "${source_bin}"
chmod 0755 "${source_bin}"
assert_version "source-build" "${source_bin}" "0.0.0-dev"

release_bin="${work_dir}/gait-release"
go build -ldflags "-X main.version=${release_version}" -o "${release_bin}" ./cmd/gait

asset_release="gait_${release_version}_${os}_${arch}.tar.gz"
project_asset_arm64="gait-local_${release_version}_darwin_arm64.tar.gz"
project_asset_amd64="gait-local_${release_version}_darwin_amd64.tar.gz"

tmp_extract="${work_dir}/extract"
mkdir -p "${tmp_extract}/gait" "${tmp_extract}/gait-local"

cp "${release_bin}" "${tmp_extract}/gait/gait"
cp "${release_bin}" "${tmp_extract}/gait-local/gait-local"

tar -czf "${release_dir}/${asset_release}" -C "${tmp_extract}/gait" gait
tar -czf "${release_dir}/${project_asset_arm64}" -C "${tmp_extract}/gait-local" gait-local
tar -czf "${release_dir}/${project_asset_amd64}" -C "${tmp_extract}/gait-local" gait-local

{
  printf '%s  %s\n' "$(sha256_file "${release_dir}/${asset_release}")" "${asset_release}"
  printf '%s  %s\n' "$(sha256_file "${release_dir}/${project_asset_amd64}")" "${project_asset_amd64}"
  printf '%s  %s\n' "$(sha256_file "${release_dir}/${project_asset_arm64}")" "${project_asset_arm64}"
} > "${release_dir}/checksums.txt"

echo "==> install.sh release path"
GAIT_RELEASE_BASE_URL="file://${release_dir}" \
  bash "${REPO_ROOT}/scripts/install.sh" \
    --version "${release_version_tag}" \
    --install-dir "${install_dir}"
assert_version "install-script" "${install_dir}/gait" "${release_version}"

echo "==> homebrew formula local path"
if [[ "${SKIP_BREW}" == "true" ]]; then
  echo "brew-install: skipped"
else
  tap_name="local/gait-version-smoke-$$"
  brew tap-new "${tap_name}" >/dev/null
  tap_repo="$(brew --repo "${tap_name}")"
  brew_formula_path="${tap_repo}/Formula/gait-local.rb"

  bash "${REPO_ROOT}/scripts/render_homebrew_formula.sh" \
    --version "${release_version_tag}" \
    --repo "Clyra-AI/gait" \
    --project "gait-local" \
    --checksums "${release_dir}/checksums.txt" \
    --release-base-url "file://${release_dir}" \
    --out "${brew_formula_path}"

  if ! grep -q "version \"${release_version}\"" "${brew_formula_path}"; then
    echo "homebrew formula missing explicit version ${release_version}" >&2
    exit 1
  fi

  HOMEBREW_NO_AUTO_UPDATE=1 brew install "${tap_name}/gait-local"
  brew_prefix="$(brew --prefix)"
  assert_version "brew-install" "${brew_prefix}/bin/gait-local" "${release_version}"
fi

echo "install-path version smoke: pass"
