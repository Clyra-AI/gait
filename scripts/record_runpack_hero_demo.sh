#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GAIT_BIN="${GAIT_BIN:-}"
OUTPUT_DIR="${OUTPUT_DIR:-${REPO_ROOT}/docs/assets}"
WORKSPACE="${WORKSPACE:-${REPO_ROOT}/gait-out/hero_demo/workspace}"
DEMO_PROFILE="${DEMO_PROFILE:-runpack}"

case "${DEMO_PROFILE}" in
  runpack)
    DEFAULT_BASE_NAME="gait_demo_20s"
    ;;
  activation)
    DEFAULT_BASE_NAME="gait_demo_activation_60s"
    ;;
  simple_e2e_60s)
    DEFAULT_BASE_NAME="gait_demo_simple_e2e_60s"
    ;;
  *)
    echo "unsupported DEMO_PROFILE: ${DEMO_PROFILE}" >&2
    exit 2
    ;;
esac

CAST_PATH="${CAST_PATH:-${OUTPUT_DIR}/${DEFAULT_BASE_NAME}.cast}"
GIF_PATH="${GIF_PATH:-${OUTPUT_DIR}/${DEFAULT_BASE_NAME}.gif}"
MP4_PATH="${MP4_PATH:-${OUTPUT_DIR}/${DEFAULT_BASE_NAME}.mp4}"

if [[ -z "${GAIT_BIN}" ]]; then
  if command -v gait >/dev/null 2>&1; then
    GAIT_BIN="$(command -v gait)"
  elif [[ -x "${REPO_ROOT}/gait" ]]; then
    GAIT_BIN="${REPO_ROOT}/gait"
  else
    (cd "${REPO_ROOT}" && go build -o ./gait ./cmd/gait)
    GAIT_BIN="${REPO_ROOT}/gait"
  fi
fi

for required in asciinema agg python3; do
  if ! command -v "${required}" >/dev/null 2>&1; then
    echo "missing required dependency: ${required}" >&2
    exit 2
  fi
done

mkdir -p "${OUTPUT_DIR}" "${WORKSPACE}"

DRIVER_SCRIPT="$(mktemp)"
cat > "${DRIVER_SCRIPT}" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

GAIT_BIN="$1"
WORKSPACE="$2"
PROFILE="${3:-runpack}"
REPO_ROOT="$4"

mkdir -p "${WORKSPACE}"
cd "${WORKSPACE}"

narrate() {
  echo
  echo "# $1"
  sleep 1
}

run_runpack_profile() {
  echo '$ gait demo --json'
  "${GAIT_BIN}" demo --json > demo.json
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("demo.json").read_text(encoding="utf-8"))
print(f"run_id={payload.get('run_id')}")
print(f"bundle={payload.get('bundle')}")
PY
  sleep 2

  echo
  echo '$ gait verify run_demo --json'
  "${GAIT_BIN}" verify run_demo --json > verify.json
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("verify.json").read_text(encoding="utf-8"))
print(f"verified={payload.get('ok')}")
print(f"manifest_digest={payload.get('manifest_digest')}")
PY
  sleep 2

  echo
  echo '$ gait run replay run_demo --json'
  "${GAIT_BIN}" run replay run_demo --json > replay.json
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("replay.json").read_text(encoding="utf-8"))
print(f"mode={payload.get('mode')}")
print(f"steps={len(payload.get('steps') or [])}")
PY
  sleep 2

  echo
  echo '$ gait regress bootstrap --from run_demo --json'
  set +e
  "${GAIT_BIN}" regress bootstrap --from run_demo --json > regress.json
  status=$?
  set -e
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("regress.json").read_text(encoding="utf-8"))
print(f"regress_status={payload.get('status')}")
print(f"failed_graders={payload.get('failed')}")
PY
  printf 'regress_exit_code=%s\n' "$status"
  sleep 2
}

run_activation_profile() {
  echo '$ gait tour --json'
  "${GAIT_BIN}" tour --json > tour.json
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("tour.json").read_text(encoding="utf-8"))
print(f"tour_ok={payload.get('ok')}")
print(f"regress_status={payload.get('regress_status')}")
print(f"next={payload.get('next_commands')}")
PY
  sleep 2

  echo
  echo '$ gait demo --durable --json'
  "${GAIT_BIN}" demo --durable --json > durable.json
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("durable.json").read_text(encoding="utf-8"))
print(f"job_id={payload.get('job_id')}")
print(f"job_status={payload.get('job_status')}")
print(f"pack_path={payload.get('pack_path')}")
PY
  sleep 2

  echo
  echo '$ gait demo --policy --json'
  "${GAIT_BIN}" demo --policy --json > policy.json
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("policy.json").read_text(encoding="utf-8"))
print(f"policy_verdict={payload.get('policy_verdict')}")
print(f"matched_rule={payload.get('matched_rule')}")
print(f"reasons={payload.get('reason_codes')}")
PY
  sleep 2
}

run_simple_e2e_profile() {
  narrate "Simple end-to-end: intent -> gate -> allow/block -> trace -> runpack -> regress"

  narrate "Step 1: evaluate an allow path at the tool boundary"
  echo '$ gait gate eval --policy examples/policy/base_low_risk.yaml --intent examples/policy/intents/intent_read.json --trace-out ./trace_allow.json --json'
  "${GAIT_BIN}" gate eval \
    --policy "${REPO_ROOT}/examples/policy/base_low_risk.yaml" \
    --intent "${REPO_ROOT}/examples/policy/intents/intent_read.json" \
    --trace-out ./trace_allow.json \
    --json > gate_allow.json
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("gate_allow.json").read_text(encoding="utf-8"))
print(f"allow_verdict={payload.get('verdict')}")
print(f"allow_trace={payload.get('trace_path')}")
PY
  sleep 1

  narrate "Step 2: evaluate a block path before side effects"
  echo '$ gait gate eval --policy examples/policy/base_high_risk.yaml --intent examples/policy/intents/intent_delete.json --trace-out ./trace_block.json --json'
  set +e
  "${GAIT_BIN}" gate eval \
    --policy "${REPO_ROOT}/examples/policy/base_high_risk.yaml" \
    --intent "${REPO_ROOT}/examples/policy/intents/intent_delete.json" \
    --trace-out ./trace_block.json \
    --json > gate_block.json
  block_exit=$?
  set -e
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("gate_block.json").read_text(encoding="utf-8"))
print(f"block_verdict={payload.get('verdict')}")
print(f"block_trace={payload.get('trace_path')}")
PY
  printf 'block_exit_code=%s\n' "$block_exit"
  sleep 1

  narrate "Step 3: capture and verify deterministic runpack evidence"
  echo '$ gait demo --json'
  "${GAIT_BIN}" demo --json > demo.json
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("demo.json").read_text(encoding="utf-8"))
print(f"run_id={payload.get('run_id')}")
print(f"bundle={payload.get('bundle')}")
PY
  echo
  echo '$ gait verify run_demo --json'
  "${GAIT_BIN}" verify run_demo --json > verify.json
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("verify.json").read_text(encoding="utf-8"))
print(f"verify_ok={payload.get('ok')}")
print(f"verify_path={payload.get('path')}")
PY
  sleep 1

  narrate "Step 4: convert evidence to CI regression gate"
  echo '$ gait regress bootstrap --from run_demo --json --junit ./junit.xml'
  "${GAIT_BIN}" regress bootstrap --from run_demo --json --junit ./junit.xml > regress.json
  python3 - <<'PY'
import json
from pathlib import Path
payload = json.loads(Path("regress.json").read_text(encoding="utf-8"))
print(f"regress_status={payload.get('status')}")
print(f"fixture={payload.get('fixture')}")
print(f"junit=./junit.xml")
PY
  sleep 1
}

case "${PROFILE}" in
  activation)
    run_activation_profile
    ;;
  runpack)
    run_runpack_profile
    ;;
  simple_e2e_60s)
    run_simple_e2e_profile
    ;;
  *)
    echo "unsupported DEMO_PROFILE: ${PROFILE}" >&2
    exit 2
    ;;
esac
SH
chmod +x "${DRIVER_SCRIPT}"

asciinema rec \
  --overwrite \
  --idle-time-limit 5 \
  --quiet \
  --command "bash ${DRIVER_SCRIPT} $(printf '%q' "${GAIT_BIN}") $(printf '%q' "${WORKSPACE}") $(printf '%q' "${DEMO_PROFILE}") $(printf '%q' "${REPO_ROOT}")" \
  "${CAST_PATH}"

agg \
  --theme github-dark \
  --speed 1.0 \
  --idle-time-limit 5 \
  --font-size 16 \
  "${CAST_PATH}" \
  "${GIF_PATH}"

if command -v ffmpeg >/dev/null 2>&1; then
  ffmpeg -y -loglevel error -i "${GIF_PATH}" -movflags faststart "${MP4_PATH}"
fi

mkdir -p "${REPO_ROOT}/docs-site/public/assets"
cp "${CAST_PATH}" "${REPO_ROOT}/docs-site/public/assets/$(basename "${CAST_PATH}")"
cp "${GIF_PATH}" "${REPO_ROOT}/docs-site/public/assets/$(basename "${GIF_PATH}")"
if [[ -f "${MP4_PATH}" ]]; then
  cp "${MP4_PATH}" "${REPO_ROOT}/docs-site/public/assets/$(basename "${MP4_PATH}")"
fi

rm -f "${DRIVER_SCRIPT}"

echo "wrote cast: ${CAST_PATH}"
echo "wrote gif: ${GIF_PATH}"
if [[ -f "${MP4_PATH}" ]]; then
  echo "wrote mp4: ${MP4_PATH}"
fi
echo "profile: ${DEMO_PROFILE}"
