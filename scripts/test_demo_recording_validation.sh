#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_NAME="${BASE_NAME:-gait_demo_simple_e2e_60s}"
MAX_SECONDS="${MAX_SECONDS:-60}"
MIN_SECONDS="${MIN_SECONDS:-8}"

CAST_DOCS="${REPO_ROOT}/docs/assets/${BASE_NAME}.cast"
GIF_DOCS="${REPO_ROOT}/docs/assets/${BASE_NAME}.gif"
MP4_DOCS="${REPO_ROOT}/docs/assets/${BASE_NAME}.mp4"
CAST_SITE="${REPO_ROOT}/docs-site/public/assets/${BASE_NAME}.cast"
GIF_SITE="${REPO_ROOT}/docs-site/public/assets/${BASE_NAME}.gif"
MP4_SITE="${REPO_ROOT}/docs-site/public/assets/${BASE_NAME}.mp4"
README_PATH="${REPO_ROOT}/README.md"

for required in "${CAST_DOCS}" "${GIF_DOCS}" "${MP4_DOCS}" "${CAST_SITE}" "${GIF_SITE}" "${MP4_SITE}"; do
  if [[ ! -f "${required}" ]]; then
    echo "missing expected demo asset: ${required}" >&2
    exit 2
  fi
done

python3 - "${CAST_DOCS}" "${MAX_SECONDS}" "${MIN_SECONDS}" <<'PY'
import json
import sys
from pathlib import Path

cast_path = Path(sys.argv[1])
max_seconds = float(sys.argv[2])
min_seconds = float(sys.argv[3])
lines = cast_path.read_text(encoding="utf-8").splitlines()
if not lines:
    raise SystemExit(f"empty cast file: {cast_path}")

# asciinema stores one JSON record per line; header line precedes events.
header = json.loads(lines[0])
version = int(header.get("version", 2)) if isinstance(header, dict) else 2
max_time = 0.0
sum_time = 0.0
output_chunks: list[str] = []
for idx, line in enumerate(lines[1:], start=2):
    line = line.strip()
    if not line:
        continue
    event = json.loads(line)
    if not isinstance(event, list) or len(event) < 3:
        raise SystemExit(f"invalid event at line {idx}")
    t, kind, data = event[0], event[1], event[2]
    if isinstance(t, (int, float)):
        numeric_t = float(t)
        max_time = max(max_time, numeric_t)
        sum_time += numeric_t
    if kind == "o" and isinstance(data, str):
        output_chunks.append(data)

duration = sum_time if version >= 3 else max_time
if duration > max_seconds:
    raise SystemExit(f"cast duration exceeded: {duration:.2f}s > {max_seconds:.2f}s")
if duration < min_seconds:
    raise SystemExit(f"cast duration too short for readability: {duration:.2f}s < {min_seconds:.2f}s")

text = "".join(output_chunks)
required_markers = [
    "Simple end-to-end: intent -> gate -> allow/block -> trace -> runpack -> regress",
    "allow_verdict=allow",
    "block_verdict=block",
    "run_id=run_demo",
    "verify_ok=True",
    "regress_status=pass",
]
missing = [marker for marker in required_markers if marker not in text]
if missing:
    raise SystemExit("missing cast markers: " + ", ".join(missing))

print(f"cast_ok duration={duration:.2f}s")
PY

if ! rg -q "gait_demo_simple_e2e_60s.gif" "${README_PATH}"; then
  echo "README missing new simple_e2e hero gif reference" >&2
  exit 2
fi

if ! rg -q "gait_demo_20s.gif" "${README_PATH}"; then
  echo "README missing legacy 20s gif reference" >&2
  exit 2
fi

if ! rg -q "Fast 20-Second Proof" "${README_PATH}"; then
  echo "README missing Fast 20-Second Proof section" >&2
  exit 2
fi

echo "demo recording validation: pass"
