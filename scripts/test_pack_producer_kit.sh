#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

BIN_PATH="${1:-$REPO_ROOT/gait}"
if [[ ! -x "$BIN_PATH" ]]; then
  (cd "$REPO_ROOT" && go build -o ./gait ./cmd/gait)
  BIN_PATH="$REPO_ROOT/gait"
fi

PACK_A="$TMP_DIR/pack_a.zip"
PACK_B="$TMP_DIR/pack_b.zip"

sha256_file() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{print $1}'
    return 0
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$path" | awk '{print $1}'
    return 0
  fi
  python3 - "$path" <<'PY'
import hashlib
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
print(hashlib.sha256(path.read_bytes()).hexdigest())
PY
}

python3 "$REPO_ROOT/scripts/pack_producer_kit.py" \
  --out "$PACK_A" \
  --run-id "run_producer_kit_fixture" \
  --created-at "2026-01-01T00:00:00Z" > "$TMP_DIR/out_a.json"
python3 "$REPO_ROOT/scripts/pack_producer_kit.py" \
  --out "$PACK_B" \
  --run-id "run_producer_kit_fixture" \
  --created-at "2026-01-01T00:00:00Z" > "$TMP_DIR/out_b.json"

SHA_A="$(sha256_file "$PACK_A")"
SHA_B="$(sha256_file "$PACK_B")"
if [[ "$SHA_A" != "$SHA_B" ]]; then
  echo "producer kit determinism failure: $SHA_A != $SHA_B" >&2
  exit 1
fi

"$BIN_PATH" pack verify "$PACK_A" --json > "$TMP_DIR/verify.json"
python3 - "$TMP_DIR/verify.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if payload.get("ok") is not True:
    raise SystemExit(f"expected ok=true, got: {payload}")
if payload.get("pack_type") != "run":
    raise SystemExit(f"expected pack_type=run, got: {payload.get('pack_type')}")
print("pack producer kit: pass")
PY
