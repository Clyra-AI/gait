#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <path-to-gait-binary> <work-dir>" >&2
  exit 2
fi

BIN_PATH="$1"
WORK_DIR="$2"

if [[ ! -x "$BIN_PATH" ]]; then
  echo "binary is not executable: $BIN_PATH" >&2
  exit 2
fi

mkdir -p "$WORK_DIR"
export PATH="$(dirname "$BIN_PATH"):$PATH"
cd "$WORK_DIR"

echo "==> installed-binary doctor"
"$BIN_PATH" doctor --json > doctor.json

python3 - <<'PY'
import json
from pathlib import Path

payload = json.loads(Path("doctor.json").read_text(encoding="utf-8"))
if payload.get("ok") is not True:
    raise SystemExit(f"expected doctor ok=true, got {payload}")
if payload.get("non_fixable"):
    raise SystemExit(f"expected doctor non_fixable=false, got {payload}")
if payload.get("status") not in {"pass", "warn"}:
    raise SystemExit(f"expected pass|warn doctor status, got {payload.get('status')}")
if payload.get("onboarding_mode") != "installed_binary":
    raise SystemExit(f"expected installed_binary onboarding mode, got {payload.get('onboarding_mode')}")
checks = {check.get("name"): check for check in payload.get("checks", [])}
for repo_only in ("schema_files", "hooks_path", "onboarding_assets"):
    if repo_only in checks:
        raise SystemExit(f"repo-only check unexpectedly present in installed-binary doctor output: {repo_only}")
PY

echo "==> installed-binary doctor production readiness"
set +e
"$BIN_PATH" doctor --production-readiness --json > doctor_production.json
production_exit="$?"
set -e
if [[ "$production_exit" -ne 2 ]]; then
  echo "expected doctor --production-readiness to exit 2, got $production_exit" >&2
  exit 1
fi

python3 - <<'PY'
import json
from pathlib import Path

payload = json.loads(Path("doctor_production.json").read_text(encoding="utf-8"))
if payload.get("ok") is not False:
    raise SystemExit(f"expected production-readiness ok=false, got {payload}")
if payload.get("status") != "fail":
    raise SystemExit(f"expected production-readiness fail status, got {payload.get('status')}")
PY

echo "==> installed-binary init"
"$BIN_PATH" init --json > init.json
init_next_command="$(python3 - <<'PY'
import json
from pathlib import Path

payload = json.loads(Path("init.json").read_text(encoding="utf-8"))
commands = payload.get("next_commands") or []
if payload.get("ok") is not True:
    raise SystemExit(f"expected init ok=true, got {payload}")
if payload.get("policy_path") != ".gait.yaml":
    raise SystemExit(f"expected init policy_path=.gait.yaml, got {payload.get('policy_path')}")
if not commands:
    raise SystemExit(f"expected init next_commands, got {payload}")
for command in commands:
    if "examples/policy/intents/" in command:
        raise SystemExit(f"unexpected repo-only next command in init output: {command}")
print(commands[0])
PY
)"
sh -c "$init_next_command" > init_next.json

echo "==> installed-binary check"
"$BIN_PATH" check --json > check.json
check_next_command="$(python3 - <<'PY'
import json
from pathlib import Path

payload = json.loads(Path("check.json").read_text(encoding="utf-8"))
commands = payload.get("next_commands") or []
if payload.get("ok") is not True:
    raise SystemExit(f"expected check ok=true, got {payload}")
if not commands:
    raise SystemExit(f"expected check next_commands, got {payload}")
for command in commands:
    if "examples/policy/intents/" in command:
        raise SystemExit(f"unexpected repo-only next command in check output: {command}")
print(commands[0])
PY
)"
sh -c "$check_next_command" > check_next.json

echo "onboarding contract smoke: pass"
