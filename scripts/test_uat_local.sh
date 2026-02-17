#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

OUTPUT_DIR="${REPO_ROOT}/.uat_local"
RELEASE_VERSION="${GAIT_UAT_RELEASE_VERSION:-}"
SKIP_BREW="false"
SKIP_DOCS_SITE="false"
FULL_CONTRACTS_ALL_PATHS="true"
PRIMARY_INSTALL_PATH="release-installer"

usage() {
  cat <<'EOF'
Run local end-to-end UAT across source, release-installer, and Homebrew install paths.

Usage:
  test_uat_local.sh [--output-dir <path>] [--release-version <tag>] [--skip-brew] [--skip-docs-site] [--baseline-install-paths]

Options:
  --output-dir <path>      UAT artifacts directory (default: .uat_local)
  --release-version <tag>  GitHub release tag for installer path (default: latest published release)
  --skip-brew              Skip Homebrew install path checks
  --skip-docs-site         Skip docs-site lint/build checks in local quality gate
  --baseline-install-paths  Run baseline suites on release/brew install paths (legacy releases only)
  --full-contracts-all-paths
                            Deprecated alias for full coverage on all install paths (default behavior)
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
    --skip-docs-site)
      SKIP_DOCS_SITE="true"
      shift
      ;;
    --full-contracts-all-paths)
      FULL_CONTRACTS_ALL_PATHS="true"
      shift
      ;;
    --baseline-install-paths)
      FULL_CONTRACTS_ALL_PATHS="false"
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

resolve_release_version() {
  if [[ -n "${RELEASE_VERSION}" ]]; then
    return 0
  fi

  RELEASE_VERSION="$(gh release view --repo Clyra-AI/gait --json tagName --jq '.tagName' 2>/dev/null || true)"
  if [[ -z "${RELEASE_VERSION}" ]]; then
    echo "error: unable to resolve latest release tag; pass --release-version explicitly" >&2
    exit 2
  fi
}

binary_supports_job_and_pack() {
  local bin_path="$1"
  local usage_output
  usage_output="$("${bin_path}" 2>&1 || true)"
  [[ "${usage_output}" == *"gait job submit"* ]] && [[ "${usage_output}" == *"gait pack build"* ]]
}

resolve_install_path_mode() {
  local requested_mode="$1"
  local bin_path="$2"
  if [[ "${requested_mode}" == "extended" || "${requested_mode}" == "baseline" ]]; then
    if ! binary_supports_job_and_pack "${bin_path}"; then
      printf '%s' "compat_v16"
      return 0
    fi
  fi
  printf '%s' "${requested_mode}"
  return 0
}

mkdir -p "${OUTPUT_DIR}/logs"
SUMMARY_PATH="${OUTPUT_DIR}/summary.txt"
: > "${SUMMARY_PATH}"

log() {
  mkdir -p "$(dirname "${SUMMARY_PATH}")"
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
  mkdir -p "${OUTPUT_DIR}/logs"
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
  local mode="${3:-extended}"
  if [[ ! -x "${bin_path}" ]]; then
    log "FAIL ${label}: binary not executable at ${bin_path}"
    exit 1
  fi

  run_step "${label}_release_smoke" bash "${REPO_ROOT}/scripts/test_release_smoke.sh" "${bin_path}"
  run_step "${label}_v1_acceptance" bash "${REPO_ROOT}/scripts/test_v1_acceptance.sh" "${bin_path}"
  if [[ "${mode}" == "compat_v1" ]]; then
    return 0
  fi

  run_step "${label}_v1_6_acceptance" bash "${REPO_ROOT}/scripts/test_v1_6_acceptance.sh" "${bin_path}"
  if [[ "${mode}" == "compat_v16" ]]; then
    return 0
  fi

  run_step "${label}_v1_7_acceptance" bash "${REPO_ROOT}/scripts/test_v1_7_acceptance.sh" "${bin_path}"
  if [[ "${mode}" == "extended" ]]; then
    run_step "${label}_v1_8_acceptance" bash "${REPO_ROOT}/scripts/test_v1_8_acceptance.sh" "${bin_path}"
    run_step "${label}_openclaw_skill_install" bash "${REPO_ROOT}/scripts/test_openclaw_skill_install.sh"
    run_step "${label}_beads_bridge" bash "${REPO_ROOT}/scripts/test_beads_bridge.sh"
  fi
}

require_cmd go
require_cmd python3
require_cmd uv
require_cmd gh
require_cmd npm

if [[ "${SKIP_BREW}" != "true" ]]; then
  require_cmd brew
fi

resolve_release_version

log "UAT output dir: ${OUTPUT_DIR}"
log "Release version: ${RELEASE_VERSION}"
log "Primary install path: ${PRIMARY_INSTALL_PATH}"
if [[ "${FULL_CONTRACTS_ALL_PATHS}" == "true" ]]; then
  log "Install-path capability mode: extended (source + release-install + brew)"
else
  log "Install-path capability mode: baseline for release-install + brew (legacy override)"
fi
if [[ "${FULL_CONTRACTS_ALL_PATHS}" == "true" ]]; then
  INSTALL_PATH_MODE_REQUESTED="extended"
else
  INSTALL_PATH_MODE_REQUESTED="baseline"
fi
log "Requested install-path suite mode for release/brew: ${INSTALL_PATH_MODE_REQUESTED}"

run_step "quality_lint" make -C "${REPO_ROOT}" lint
run_step "quality_test" make -C "${REPO_ROOT}" test
run_step "quality_e2e" make -C "${REPO_ROOT}" test-e2e
run_step "quality_integration" go test "${REPO_ROOT}/internal/integration" -count=1
run_step "quality_adoption" make -C "${REPO_ROOT}" test-adoption
run_step "quality_adapter_parity" make -C "${REPO_ROOT}" test-adapter-parity
run_step "quality_policy_compliance" bash "${REPO_ROOT}/scripts/policy_compliance_ci.sh"
run_step "quality_contracts" make -C "${REPO_ROOT}" test-contracts
run_step "quality_v2_3_acceptance" make -C "${REPO_ROOT}" test-v2-3-acceptance
run_step "quality_v2_4_acceptance" make -C "${REPO_ROOT}" test-v2-4-acceptance
run_step "quality_v2_5_acceptance" make -C "${REPO_ROOT}" test-v2-5-acceptance
run_step "quality_v2_6_acceptance" make -C "${REPO_ROOT}" test-v2-6-acceptance
run_step "quality_voice_acceptance" make -C "${REPO_ROOT}" test-voice-acceptance
run_step "quality_context_conformance" make -C "${REPO_ROOT}" test-context-conformance
run_step "quality_context_chaos" make -C "${REPO_ROOT}" test-context-chaos
run_step "quality_ui_acceptance" make -C "${REPO_ROOT}" test-ui-acceptance
run_step "quality_ui_unit" make -C "${REPO_ROOT}" test-ui-unit
run_step "quality_ui_e2e_smoke" make -C "${REPO_ROOT}" test-ui-e2e-smoke
run_step "quality_ui_perf" make -C "${REPO_ROOT}" test-ui-perf
run_step "quality_packspec_tck" make -C "${REPO_ROOT}" test-packspec-tck
run_step "quality_hardening_acceptance" make -C "${REPO_ROOT}" test-hardening-acceptance
run_step "quality_chaos" make -C "${REPO_ROOT}" test-chaos
run_step "quality_session_soak" bash "${REPO_ROOT}/scripts/test_session_soak.sh"
run_step "quality_runtime_slo" make -C "${REPO_ROOT}" test-runtime-slo
run_step "quality_perf_bench_check" make -C "${REPO_ROOT}" bench-check
if [[ "${SKIP_BREW}" == "true" ]]; then
  run_step "quality_install_path_versions" bash "${REPO_ROOT}/scripts/test_cli_version_install_paths.sh" --skip-brew
else
  run_step "quality_install_path_versions" make -C "${REPO_ROOT}" test-install-path-versions
fi

if [[ "${SKIP_DOCS_SITE}" == "true" ]]; then
  log "SKIP quality_docs_site (requested)"
elif command -v npm >/dev/null 2>&1; then
  run_step "quality_docs_site" make -C "${REPO_ROOT}" docs-site-lint docs-site-build docs-site-check
else
  log "SKIP quality_docs_site (npm missing)"
fi

SOURCE_BIN="${REPO_ROOT}/gait"
run_step "build_source_binary" go build -o "${SOURCE_BIN}" "${REPO_ROOT}/cmd/gait"
run_binary_contract_suite "source" "${SOURCE_BIN}" "extended"

RELEASE_INSTALL_DIR="${OUTPUT_DIR}/release_install/bin"
mkdir -p "${RELEASE_INSTALL_DIR}"
run_step "install_release_binary" bash "${REPO_ROOT}/scripts/install.sh" --version "${RELEASE_VERSION}" --install-dir "${RELEASE_INSTALL_DIR}"
if [[ "${FULL_CONTRACTS_ALL_PATHS}" == "true" ]]; then
  RELEASE_INSTALL_MODE="$(resolve_install_path_mode "${INSTALL_PATH_MODE_REQUESTED}" "${RELEASE_INSTALL_DIR}/gait")"
  log "Resolved release_install suite mode: ${RELEASE_INSTALL_MODE}"
  run_binary_contract_suite "release_install" "${RELEASE_INSTALL_DIR}/gait" "${RELEASE_INSTALL_MODE}"
else
  RELEASE_INSTALL_MODE="$(resolve_install_path_mode "${INSTALL_PATH_MODE_REQUESTED}" "${RELEASE_INSTALL_DIR}/gait")"
  log "Resolved release_install suite mode: ${RELEASE_INSTALL_MODE}"
  run_binary_contract_suite "release_install" "${RELEASE_INSTALL_DIR}/gait" "${RELEASE_INSTALL_MODE}"
fi
log "PRIMARY_INSTALL_PATH_STATUS release-installer PASS"

if [[ "${SKIP_BREW}" == "true" ]]; then
  log "SKIP brew_path (requested)"
else
  run_step "brew_tap" brew tap Clyra-AI/tap
  run_step "brew_update" brew update
  run_step "brew_reinstall" brew reinstall Clyra-AI/tap/gait
  run_step "brew_test_formula" brew test Clyra-AI/tap/gait

  BREW_PREFIX="$(brew --prefix)"
  BREW_BIN="${BREW_PREFIX}/bin/gait"
  if [[ "${FULL_CONTRACTS_ALL_PATHS}" == "true" ]]; then
    BREW_MODE="$(resolve_install_path_mode "${INSTALL_PATH_MODE_REQUESTED}" "${BREW_BIN}")"
    log "Resolved brew suite mode: ${BREW_MODE}"
    run_binary_contract_suite "brew" "${BREW_BIN}" "${BREW_MODE}"
  else
    BREW_MODE="$(resolve_install_path_mode "${INSTALL_PATH_MODE_REQUESTED}" "${BREW_BIN}")"
    log "Resolved brew suite mode: ${BREW_MODE}"
    run_binary_contract_suite "brew" "${BREW_BIN}" "${BREW_MODE}"
  fi
fi

log "UAT COMPLETE: PASS"
