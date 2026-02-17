#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

usage() {
  cat <<'EOF'
Publish Homebrew formula updates into a tap repository.

Usage:
  publish_homebrew_tap.sh --version <tag> [options]

Options:
  --version <tag>         Release tag to publish (required, e.g. v1.0.0)
  --source-repo <owner/repo>
                          Source release repository (default: Clyra-AI/gait)
  --tap-repo <owner/repo> Target tap repository (default: Clyra-AI/homebrew-tap)
  --formula <name>        Formula file name without .rb (default: gait)
  --branch <name>         Target branch in tap repo (default: main)
  --checksums <path>      Use local checksums file instead of downloading release asset
  --dry-run               Render and diff but do not commit/push
  -h, --help              Show this help

Environment:
  HOMEBREW_TAP_TOKEN      Required for push unless --dry-run is set
  GH_TOKEN                Optional token for gh release download (recommended in CI)
EOF
}

version=""
source_repo="Clyra-AI/gait"
tap_repo="Clyra-AI/homebrew-tap"
formula_name="gait"
target_branch="main"
checksums_override=""
dry_run="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || { echo "error: --version requires a value" >&2; exit 2; }
      version="$2"
      shift 2
      ;;
    --source-repo)
      [[ $# -ge 2 ]] || { echo "error: --source-repo requires a value" >&2; exit 2; }
      source_repo="$2"
      shift 2
      ;;
    --tap-repo)
      [[ $# -ge 2 ]] || { echo "error: --tap-repo requires a value" >&2; exit 2; }
      tap_repo="$2"
      shift 2
      ;;
    --formula)
      [[ $# -ge 2 ]] || { echo "error: --formula requires a value" >&2; exit 2; }
      formula_name="$2"
      shift 2
      ;;
    --branch)
      [[ $# -ge 2 ]] || { echo "error: --branch requires a value" >&2; exit 2; }
      target_branch="$2"
      shift 2
      ;;
    --checksums)
      [[ $# -ge 2 ]] || { echo "error: --checksums requires a value" >&2; exit 2; }
      checksums_override="$2"
      shift 2
      ;;
    --dry-run)
      dry_run="true"
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

if [[ -z "$version" ]]; then
  echo "error: --version is required" >&2
  exit 2
fi

if [[ "$dry_run" != "true" && -z "${HOMEBREW_TAP_TOKEN:-}" ]]; then
  echo "error: HOMEBREW_TAP_TOKEN is required (or use --dry-run)" >&2
  exit 2
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "error: gh CLI is required" >&2
  exit 2
fi

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

checksums_path="${checksums_override}"
if [[ -z "$checksums_path" ]]; then
  attempts=4
  for attempt in $(seq 1 "${attempts}"); do
    set +e
    download_out="$(
      gh release download "$version" \
        --repo "$source_repo" \
        --pattern "checksums.txt" \
        --dir "$workdir" 2>&1
    )"
    code=$?
    set -e
    if [[ $code -eq 0 ]]; then
      checksums_path="${workdir}/checksums.txt"
      break
    fi
    if printf '%s' "$download_out" | rg -q "429|throttl" && [[ $attempt -lt $attempts ]]; then
      sleep_seconds=$((attempt * 30))
      echo "gh release download throttled (attempt ${attempt}/${attempts}); retrying in ${sleep_seconds}s..."
      sleep "${sleep_seconds}"
      continue
    fi
    echo "$download_out" >&2
    exit "$code"
  done
fi

if [[ ! -f "$checksums_path" ]]; then
  echo "error: checksums file not found: $checksums_path" >&2
  exit 2
fi

rendered_formula="${workdir}/${formula_name}.rb"
bash "${REPO_ROOT}/scripts/render_homebrew_formula.sh" \
  --repo "$source_repo" \
  --version "$version" \
  --checksums "$checksums_path" \
  --out "$rendered_formula"

tap_checkout="${workdir}/tap"
if [[ "$dry_run" == "true" ]]; then
  git clone "https://github.com/${tap_repo}.git" "$tap_checkout"
else
  git clone "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/${tap_repo}.git" "$tap_checkout"
fi

cd "$tap_checkout"
git checkout "$target_branch"

mkdir -p Formula
target_formula="Formula/${formula_name}.rb"
if [[ -f "$target_formula" ]] && cmp -s "$rendered_formula" "$target_formula"; then
  echo "tap formula already up to date: ${tap_repo}/${target_formula}"
  exit 0
fi

cp "$rendered_formula" "$target_formula"
git add "$target_formula"

if [[ "$dry_run" == "true" ]]; then
  echo "dry-run: would publish formula changes for ${version} to ${tap_repo}:${target_branch}"
  git --no-pager diff --cached
  exit 0
fi

git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git commit -m "homebrew: update ${formula_name} for ${version}"

attempts=4
for attempt in $(seq 1 "${attempts}"); do
  set +e
  push_out="$(git push origin "HEAD:${target_branch}" 2>&1)"
  code=$?
  set -e
  if [[ $code -eq 0 ]]; then
    echo "published ${tap_repo}/${target_formula} for ${version}"
    exit 0
  fi
  if printf '%s' "$push_out" | rg -q "429|throttl" && [[ $attempt -lt $attempts ]]; then
    sleep_seconds=$((attempt * 30))
    echo "git push throttled (attempt ${attempt}/${attempts}); retrying in ${sleep_seconds}s..."
    sleep "${sleep_seconds}"
    continue
  fi
  echo "$push_out" >&2
  exit "$code"
done

exit 1
