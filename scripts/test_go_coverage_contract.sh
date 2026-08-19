#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PACKAGE_THRESHOLD="${1:-75}"
AGGREGATE_THRESHOLD="${2:-85}"
PYTHON="${PYTHON:-python3}"

cd "$REPO_ROOT"

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

go_test_packages=()
while IFS= read -r package; do
  package=${package%$'\r'}
  [[ -n "$package" ]] || continue
  go_test_packages+=("$package")
done < <("$PYTHON" scripts/list_go_test_packages.py)

if [[ "${#go_test_packages[@]}" -eq 0 ]]; then
  echo "no Go test packages discovered" >&2
  exit 1
fi

set +e
go test "${go_test_packages[@]}" -cover > "$WORK_DIR/coverage-go-packages.out" 2>&1
package_test_status=$?
set -e
cat "$WORK_DIR/coverage-go-packages.out"
if [[ "$package_test_status" -ne 0 ]]; then
  echo "go test filtered packages with coverage failed" >&2
  exit "$package_test_status"
fi
"$PYTHON" scripts/check_go_package_coverage.py "$WORK_DIR/coverage-go-packages.out" "$PACKAGE_THRESHOLD"

go test ./core/... ./cmd/gait -coverprofile="$WORK_DIR/coverage-go.out"
if [[ "${RUNNER_OS:-Linux}" == "Linux" ]]; then
  "$PYTHON" scripts/check_go_coverage.py "$WORK_DIR/coverage-go.out" "$AGGREGATE_THRESHOLD"
else
  go tool cover -func="$WORK_DIR/coverage-go.out" | tail -n 1
fi
