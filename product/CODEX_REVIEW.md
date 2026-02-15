# Codex-Style Repo Review Findings

Date: 2026-02-15  
Scope: full repository (`main`)  
Mode: read-only investigation (no functional code changes)

## Findings

### [P1] `gait verify` accepts runpacks that omit required artifact files

- Files:
  - `core/runpack/verify.go:75`
  - `core/runpack/verify.go:83`
  - `cmd/gait/verify.go:392`
- Problem:
  - `VerifyZip` only validates files declared in `manifest.files`.
  - It never enforces that runpack-required files (`run.json`, `intents.jsonl`, `results.jsonl`, `refs.json`) are declared/present.
  - `verifyRunpackArtifact` marks verification `ok=true` when `missing_files` and `hash_mismatches` are empty, so an artifact with `files: []` is accepted as valid.
- Why this matters:
  - It breaks the runpack contract and allows structurally incomplete artifacts to pass integrity verification.
  - Downstream systems can treat non-replayable/non-auditable packs as trusted.
- Repro:
  1. Create zip containing only `manifest.json` with `run_id`, `files: []`.
  2. Run `./gait verify <zip> --json`.
  3. Current result returns `"ok": true`.

### [P1] `gait verify` does not validate `manifest_digest` correctness

- Files:
  - `core/runpack/verify.go:77`
  - `core/runpack/verify.go:150`
  - `cmd/gait/verify.go:83`
- Problem:
  - `manifest_digest` is surfaced in output but never recomputed and compared against the manifest content.
  - Any arbitrary digest string is accepted.
- Why this matters:
  - The command explicitly claims manifest-digest verification, but it currently does not provide that guarantee.
  - This weakens integrity signaling in security/compliance workflows.
- Repro:
  1. Set `manifest_digest` to `"deadbeef"` in `manifest.json`.
  2. Run `./gait verify <zip> --json`.
  3. Current result can still return `"ok": true` with the bogus digest echoed back.

### [P2] `gait verify` does not enforce runpack manifest schema identity/version

- Files:
  - `core/runpack/verify.go:67`
  - `core/runpack/verify.go:71`
- Problem:
  - Verification checks only that `manifest.json` parses and has non-empty `run_id`.
  - It does not validate `schema_id == gait.runpack.manifest` and `schema_version == 1.0.0`.
- Why this matters:
  - A non-runpack manifest shape can be accepted as a valid runpack verification target.
  - This undermines schema-stability guarantees and contract enforcement.
- Repro:
  1. Create `manifest.json` with `schema_id: "not.gait"` and `schema_version: "999"`, plus `run_id`.
  2. Run `./gait verify <zip> --json`.
  3. Current result can still return `"ok": true`.

## What I Ran

- `go test ./...` (pass)
- `make lint-fast` (pass)
- `python3 scripts/check_docs_site_validation.py` (pass)
- Direct CLI repros of malformed runpack manifests (showing false-positive `ok: true` in `gait verify`)
