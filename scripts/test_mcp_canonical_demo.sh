#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

"$REPO_ROOT/scripts/demo_mcp_canonical.sh" "$WORK_DIR"

python3 - "$WORK_DIR/mcp_canonical_summary.json" <<'PY'
import json
import pathlib
import sys

payload = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
if payload.get("schema_id") != "gait.mcp.canonical.summary":
    raise SystemExit("summary schema_id mismatch")
cases = payload.get("cases", {})
expected = {
    "allow": ("allow", 0),
    "block": ("block", 3),
    "require_approval": ("require_approval", 4),
}
for key, (verdict, exit_code) in expected.items():
    case = cases.get(key, {})
    if case.get("verdict") != verdict:
        raise SystemExit(f"case {key} verdict mismatch: {case}")
    if case.get("exit_code") != exit_code:
        raise SystemExit(f"case {key} exit_code mismatch: {case}")
    if not pathlib.Path(case.get("trace_path", "")).exists():
        raise SystemExit(f"case {key} missing trace artifact")
    if not pathlib.Path(case.get("runpack_path", "")).exists():
        raise SystemExit(f"case {key} missing runpack artifact")
    if not pathlib.Path(case.get("pack_path", "")).exists():
        raise SystemExit(f"case {key} missing pack artifact")
print("mcp canonical demo: pass")
PY
