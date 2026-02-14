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
  BIN_PATH="$REPO_ROOT/gait"
  go build -o "$BIN_PATH" ./cmd/gait
fi

if [[ ! -x "$BIN_PATH" ]]; then
  echo "binary is not executable: $BIN_PATH" >&2
  exit 2
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

cp "$REPO_ROOT/scripts/testdata/packspec_tck/v1/run_record_input.json" "$WORK_DIR/run_record_input.json"

mkdir -p "$WORK_DIR/gait-out" "$WORK_DIR/jobs"

echo "==> tck: valid run pack"
"$BIN_PATH" run record --input "$WORK_DIR/run_record_input.json" --out-dir "$WORK_DIR/gait-out" --json > "$WORK_DIR/run_record.json"
"$BIN_PATH" pack build --type run --from "$WORK_DIR/gait-out/runpack_run_tck.zip" --out "$WORK_DIR/pack_run.zip" --json > "$WORK_DIR/pack_run_build.json"
"$BIN_PATH" pack verify "$WORK_DIR/pack_run.zip" --json > "$WORK_DIR/pack_run_verify.json"

echo "==> tck: valid job pack"
"$BIN_PATH" job submit --id job_tck --root "$WORK_DIR/jobs" --json > "$WORK_DIR/job_submit.json"
"$BIN_PATH" job checkpoint add --id job_tck --root "$WORK_DIR/jobs" --type decision-needed --summary "approval required" --required-action "approve" --json > "$WORK_DIR/job_checkpoint.json"
"$BIN_PATH" job approve --id job_tck --root "$WORK_DIR/jobs" --actor alice --reason "approved" --json > "$WORK_DIR/job_approve.json"
"$BIN_PATH" job resume --id job_tck --root "$WORK_DIR/jobs" --json > "$WORK_DIR/job_resume.json"
"$BIN_PATH" pack build --type job --from job_tck --job-root "$WORK_DIR/jobs" --out "$WORK_DIR/pack_job.zip" --json > "$WORK_DIR/pack_job_build.json"
"$BIN_PATH" pack verify "$WORK_DIR/pack_job.zip" --json > "$WORK_DIR/pack_job_verify.json"

echo "==> tck: legacy migration vectors"
"$BIN_PATH" pack verify "$WORK_DIR/gait-out/runpack_run_tck.zip" --json > "$WORK_DIR/legacy_run_verify.json"
"$BIN_PATH" guard pack --run "$WORK_DIR/gait-out/runpack_run_tck.zip" --out "$WORK_DIR/evidence_pack_legacy.zip" --json > "$WORK_DIR/legacy_guard_pack.json"
"$BIN_PATH" pack verify "$WORK_DIR/evidence_pack_legacy.zip" --json > "$WORK_DIR/legacy_guard_verify.json"

echo "==> tck: tampered hash vector"
python3 - "$WORK_DIR/pack_run.zip" "$WORK_DIR/pack_run_tampered.zip" <<'PY'
import sys
import zipfile
from pathlib import Path

src = Path(sys.argv[1])
out = Path(sys.argv[2])
with zipfile.ZipFile(src, "r") as zin, zipfile.ZipFile(out, "w", compression=zipfile.ZIP_DEFLATED) as zout:
    for info in zin.infolist():
        data = zin.read(info.filename)
        if info.filename == "run_payload.json":
            data = data + b"\n"
        clone = zipfile.ZipInfo(filename=info.filename, date_time=(1980, 1, 1, 0, 0, 0))
        clone.external_attr = info.external_attr
        clone.compress_type = zipfile.ZIP_DEFLATED
        zout.writestr(clone, data)
PY
set +e
"$BIN_PATH" pack verify "$WORK_DIR/pack_run_tampered.zip" --json > "$WORK_DIR/pack_run_tampered_verify.json"
TAMPER_CODE=$?
set -e
if [[ $TAMPER_CODE -eq 0 ]]; then
  echo "expected tampered pack verify to fail" >&2
  exit 1
fi

echo "==> tck: undeclared file vector"
python3 - "$WORK_DIR/pack_run.zip" "$WORK_DIR/pack_run_undeclared.zip" <<'PY'
import sys
import zipfile
from pathlib import Path

src = Path(sys.argv[1])
out = Path(sys.argv[2])
with zipfile.ZipFile(src, "r") as zin, zipfile.ZipFile(out, "w", compression=zipfile.ZIP_DEFLATED) as zout:
    for info in zin.infolist():
        data = zin.read(info.filename)
        clone = zipfile.ZipInfo(filename=info.filename, date_time=(1980, 1, 1, 0, 0, 0))
        clone.external_attr = info.external_attr
        clone.compress_type = zipfile.ZIP_DEFLATED
        zout.writestr(clone, data)
    extra = zipfile.ZipInfo(filename="extra.txt", date_time=(1980, 1, 1, 0, 0, 0))
    extra.external_attr = 0o644 << 16
    extra.compress_type = zipfile.ZIP_DEFLATED
    zout.writestr(extra, b"undeclared")
PY
set +e
"$BIN_PATH" pack verify "$WORK_DIR/pack_run_undeclared.zip" --json > "$WORK_DIR/pack_run_undeclared_verify.json"
UNDECLARED_CODE=$?
set -e
if [[ $UNDECLARED_CODE -eq 0 ]]; then
  echo "expected undeclared-file pack verify to fail" >&2
  exit 1
fi

echo "==> tck: schema-invalid vector"
python3 - "$WORK_DIR/pack_run.zip" "$WORK_DIR/pack_run_schema_invalid.zip" <<'PY'
import json
import sys
import zipfile
from pathlib import Path

src = Path(sys.argv[1])
out = Path(sys.argv[2])
with zipfile.ZipFile(src, "r") as zin, zipfile.ZipFile(out, "w", compression=zipfile.ZIP_DEFLATED) as zout:
    for info in zin.infolist():
        data = zin.read(info.filename)
        if info.filename == "pack_manifest.json":
            manifest = json.loads(data.decode("utf-8"))
            manifest["schema_id"] = "gait.pack.invalid"
            data = json.dumps(manifest, separators=(",", ":")).encode("utf-8")
        clone = zipfile.ZipInfo(filename=info.filename, date_time=(1980, 1, 1, 0, 0, 0))
        clone.external_attr = info.external_attr
        clone.compress_type = zipfile.ZIP_DEFLATED
        zout.writestr(clone, data)
PY
set +e
"$BIN_PATH" pack verify "$WORK_DIR/pack_run_schema_invalid.zip" --json > "$WORK_DIR/pack_run_schema_invalid_verify.json"
SCHEMA_CODE=$?
set -e
if [[ $SCHEMA_CODE -eq 0 ]]; then
  echo "expected schema-invalid pack verify to fail" >&2
  exit 1
fi

echo "==> tck: deterministic diff"
set +e
"$BIN_PATH" pack diff "$WORK_DIR/pack_run.zip" "$WORK_DIR/pack_job.zip" --json > "$WORK_DIR/diff_1.json"
DIFF_CODE_1=$?
"$BIN_PATH" pack diff "$WORK_DIR/pack_run.zip" "$WORK_DIR/pack_job.zip" --json > "$WORK_DIR/diff_2.json"
DIFF_CODE_2=$?
set -e
if [[ $DIFF_CODE_1 -ne $DIFF_CODE_2 ]]; then
  echo "diff exit codes are unstable: $DIFF_CODE_1 vs $DIFF_CODE_2" >&2
  exit 1
fi
python3 - "$WORK_DIR/diff_1.json" "$WORK_DIR/diff_2.json" <<'PY'
import hashlib
import json
import sys
from pathlib import Path

left = Path(sys.argv[1]).read_bytes()
right = Path(sys.argv[2]).read_bytes()
if hashlib.sha256(left).hexdigest() != hashlib.sha256(right).hexdigest():
    raise SystemExit("diff output is not deterministic")
obj = json.loads(left.decode("utf-8"))
if "diff" not in obj:
    raise SystemExit("pack diff json missing diff payload")
PY

python3 - "$WORK_DIR" <<'PY'
import json
import sys
from pathlib import Path

work = Path(sys.argv[1])
checks = {
    "pack_run_verify.json": True,
    "pack_job_verify.json": True,
    "legacy_run_verify.json": True,
    "legacy_guard_verify.json": True,
}
for name, expected_ok in checks.items():
    payload = json.loads((work / name).read_text(encoding="utf-8"))
    if bool(payload.get("ok")) != expected_ok:
        raise SystemExit(f"{name} expected ok={expected_ok} got {payload.get('ok')}")

legacy_run = json.loads((work / "legacy_run_verify.json").read_text(encoding="utf-8"))
legacy_guard = json.loads((work / "legacy_guard_verify.json").read_text(encoding="utf-8"))
if legacy_run.get("verify", {}).get("legacy_type") != "runpack":
    raise SystemExit("legacy run verify did not report legacy_type=runpack")
if legacy_guard.get("verify", {}).get("legacy_type") != "guard":
    raise SystemExit("legacy guard verify did not report legacy_type=guard")
PY

echo "packspec tck: pass"
