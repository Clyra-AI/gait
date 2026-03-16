#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ $# -gt 1 ]]; then
  echo "usage: $0 [path-to-gait-binary]" >&2
  exit 2
fi

if [[ $# -eq 1 ]]; then
  if [[ "$1" = /* ]]; then
    BIN_PATH="$1"
  else
    BIN_PATH="$(pwd)/$1"
  fi
else
  BIN_PATH="${REPO_ROOT}/gait"
  go build -o "${BIN_PATH}" ./cmd/gait
fi

if [[ ! -x "${BIN_PATH}" ]]; then
  echo "binary is not executable: ${BIN_PATH}" >&2
  exit 2
fi

detect_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)
      echo "unsupported OS for install smoke: $(uname -s)" >&2
      exit 2
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "unsupported architecture for install smoke: $(uname -m)" >&2
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

os="$(detect_os)"
arch="$(detect_arch)"
version="v0.0.0-ci"

work_dir="$(mktemp -d)"
trap 'rm -rf "${work_dir}"' EXIT

release_dir="${work_dir}/release"
install_dir="${work_dir}/bin"
extract_dir="${work_dir}/extract"
mkdir -p "${release_dir}" "${install_dir}" "${extract_dir}"

asset_name="gait_${version}_${os}_${arch}.tar.gz"
cp "${BIN_PATH}" "${extract_dir}/gait"
tar -czf "${release_dir}/${asset_name}" -C "${extract_dir}" gait
checksum="$(sha256_file "${release_dir}/${asset_name}")"
printf '%s  %s\n' "${checksum}" "${asset_name}" > "${release_dir}/checksums.txt"

echo "==> install script smoke"
GAIT_RELEASE_BASE_URL="file://${release_dir}" \
  bash "${REPO_ROOT}/scripts/install.sh" \
    --version "${version}" \
    --install-dir "${install_dir}"

if [[ ! -x "${install_dir}/gait" ]]; then
  echo "installed binary missing: ${install_dir}/gait" >&2
  exit 1
fi

export PATH="${install_dir}:$PATH"

bash "${REPO_ROOT}/scripts/test_onboarding_contract.sh" "${install_dir}/gait" "${work_dir}/onboarding"

"${install_dir}/gait" demo > "${work_dir}/demo.txt"
if ! grep -q '^run_id=run_demo$' "${work_dir}/demo.txt"; then
  echo "installed binary demo output missing run_id" >&2
  cat "${work_dir}/demo.txt" >&2
  exit 1
fi

echo "install smoke: pass"
