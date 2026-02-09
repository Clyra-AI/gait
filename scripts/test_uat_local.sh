#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

OUTPUT_DIR="${REPO_ROOT}/gait-out/uat_local"
RELEASE_VERSION="${GAIT_UAT_RELEASE_VERSION:-v1.0.0}"
SKIP_BREW="false"

usage() {
  cat <<'EOF'
Run local end-to-end UAT across source, release-installer, and Homebrew install paths.

Usage:
  test_uat_local.sh [--output-dir <path>] [--release-version <tag>] [--skip-brew]

Options:
  --output-dir <path>      UAT artifacts directory (default: gait-out/uat_local)
  --release-version <tag>  GitHub release tag for installer path (default: v1.0.0)
  --skip-brew              Skip Homebrew install path checks
  -h, --help               Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --output-dir)
      [[ $# -ge 2 ]] || { echo "error: --output-dir requires a value" >&2; exit 2; }
      OUTPUT_DIR="$2"
      shift 2
      ;;
    --release-version)
      [[ $# -ge 2 ]] || { echo "error: --release-version requires a value" >&2; exit 2; }
      RELEASE_VERSION="$2"
      shift 2
      ;;
    --skip-brew)
      SKIP_BREW="true"
      shift
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

mkdir -p "${OUTPUT_DIR}/logs"
SUMMARY_PATH="${OUTPUT_DIR}/summary.txt"
: > "${SUMMARY_PATH}"

log() {
  printf '%s\n' "$*" | tee -a "${SUMMARY_PATH}"
}

require_cmd() {
  local name="$1"
  if ! command -v "${name}" >/dev/null 2>&1; then
    log "FAIL missing command: ${name}"
    exit 1
  fi
}

run_step() {
  local name="$1"
  shift
  local log_path="${OUTPUT_DIR}/logs/${name}.log"
  log "==> ${name}"
  if "$@" >"${log_path}" 2>&1; then
    log "PASS ${name}"
  else
    log "FAIL ${name} (see ${log_path})"
    tail -n 80 "${log_path}" || true
    exit 1
  fi
}

run_binary_contract_suite() {
  local label="$1"
  local bin_path="$2"
  if [[ ! -x "${bin_path}" ]]; then
    log "FAIL ${label}: binary not executable at ${bin_path}"
    exit 1
  fi

  run_step "${label}_v1_acceptance" bash "${REPO_ROOT}/scripts/test_v1_acceptance.sh" "${bin_path}"
  run_step "${label}_v1_6_acceptance" bash "${REPO_ROOT}/scripts/test_v1_6_acceptance.sh" "${bin_path}"
  run_step "${label}_v1_7_acceptance" bash "${REPO_ROOT}/scripts/test_v1_7_acceptance.sh" "${bin_path}"
  run_step "${label}_release_smoke" bash "${REPO_ROOT}/scripts/test_release_smoke.sh" "${bin_path}"
}

log "UAT output dir: ${OUTPUT_DIR}"
log "Release version: ${RELEASE_VERSION}"

require_cmd go
require_cmd python3
require_cmd uv
require_cmd gh

if [[ "${SKIP_BREW}" != "true" ]]; then
  require_cmd brew
fi

run_step "quality_lint" make -C "${REPO_ROOT}" lint
run_step "quality_test" make -C "${REPO_ROOT}" test
run_step "quality_e2e" make -C "${REPO_ROOT}" test-e2e
run_step "quality_adoption" make -C "${REPO_ROOT}" test-adoption
run_step "quality_contracts" make -C "${REPO_ROOT}" test-contracts
run_step "quality_hardening_acceptance" make -C "${REPO_ROOT}" test-hardening-acceptance

SOURCE_BIN="${REPO_ROOT}/gait"
run_step "build_source_binary" go build -o "${SOURCE_BIN}" "${REPO_ROOT}/cmd/gait"
run_binary_contract_suite "source" "${SOURCE_BIN}"

RELEASE_INSTALL_DIR="${OUTPUT_DIR}/release_install/bin"
mkdir -p "${RELEASE_INSTALL_DIR}"
run_step "install_release_binary" bash "${REPO_ROOT}/scripts/install.sh" --version "${RELEASE_VERSION}" --install-dir "${RELEASE_INSTALL_DIR}"
run_binary_contract_suite "release_install" "${RELEASE_INSTALL_DIR}/gait"

if [[ "${SKIP_BREW}" == "true" ]]; then
  log "SKIP brew_path (requested)"
else
  run_step "brew_tap" brew tap davidahmann/tap
  run_step "brew_reinstall" brew reinstall davidahmann/tap/gait
  run_step "brew_test_formula" brew test davidahmann/tap/gait

  BREW_PREFIX="$(brew --prefix)"
  BREW_BIN="${BREW_PREFIX}/bin/gait"
  run_binary_contract_suite "brew" "${BREW_BIN}"
fi

log "UAT COMPLETE: PASS"
