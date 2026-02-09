#!/usr/bin/env python3
"""Validate docs/ecosystem/community_index.json against repository contract."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import Any

DEFAULT_PATH = Path("docs/ecosystem/community_index.json")
ENTRY_ID_PATTERN = re.compile(r"^[a-z0-9][a-z0-9-]{1,63}$")
REPO_PATTERN = re.compile(r"^https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$")
KIND_VALUES = {"skill", "adapter", "policy_pack", "tooling"}
SOURCE_VALUES = {"official", "community"}
STATUS_VALUES = {"experimental", "stable", "deprecated"}


def fail(message: str) -> None:
    print(f"community index validation failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def expect_type(value: Any, expected_type: type, context: str) -> None:
    if not isinstance(value, expected_type):
        fail(f"{context} must be {expected_type.__name__}")


def validate_entry(entry: dict[str, Any], index: int) -> None:
    context = f"entries[{index}]"
    required = {"id", "kind", "name", "summary", "repo", "source", "status"}
    missing = sorted(required.difference(entry.keys()))
    if missing:
        fail(f"{context} missing required keys: {missing}")

    unknown = sorted(set(entry.keys()).difference(required | {"integration", "maintainers"}))
    if unknown:
        fail(f"{context} contains unknown keys: {unknown}")

    entry_id = entry["id"]
    expect_type(entry_id, str, f"{context}.id")
    if ENTRY_ID_PATTERN.match(entry_id) is None:
        fail(f"{context}.id must match {ENTRY_ID_PATTERN.pattern}")

    kind = entry["kind"]
    expect_type(kind, str, f"{context}.kind")
    if kind not in KIND_VALUES:
        fail(f"{context}.kind must be one of {sorted(KIND_VALUES)}")

    for key in ("name", "summary", "repo", "source", "status"):
        expect_type(entry[key], str, f"{context}.{key}")

    if not (3 <= len(entry["name"]) <= 80):
        fail(f"{context}.name length must be between 3 and 80")
    if not (10 <= len(entry["summary"]) <= 280):
        fail(f"{context}.summary length must be between 10 and 280")
    if REPO_PATTERN.match(entry["repo"]) is None:
        fail(f"{context}.repo must match {REPO_PATTERN.pattern}")
    if entry["source"] not in SOURCE_VALUES:
        fail(f"{context}.source must be one of {sorted(SOURCE_VALUES)}")
    if entry["status"] not in STATUS_VALUES:
        fail(f"{context}.status must be one of {sorted(STATUS_VALUES)}")

    if "integration" in entry:
        expect_type(entry["integration"], str, f"{context}.integration")
        if not entry["integration"]:
            fail(f"{context}.integration must be non-empty when present")

    if "maintainers" in entry:
        maintainers = entry["maintainers"]
        expect_type(maintainers, list, f"{context}.maintainers")
        if len(maintainers) > 5:
            fail(f"{context}.maintainers cannot exceed 5 entries")
        for maintainer_index, maintainer in enumerate(maintainers):
            expect_type(maintainer, str, f"{context}.maintainers[{maintainer_index}]")
            if len(maintainer) < 3:
                fail(f"{context}.maintainers[{maintainer_index}] must be at least 3 chars")


def validate(path: Path) -> None:
    if not path.exists():
        fail(f"index file does not exist: {path}")

    payload = json.loads(path.read_text(encoding="utf-8"))
    expect_type(payload, dict, "root")

    required_top_level = {"schema_id", "schema_version", "updated_at", "entries"}
    missing_top_level = sorted(required_top_level.difference(payload.keys()))
    if missing_top_level:
        fail(f"root missing required keys: {missing_top_level}")

    unknown_top_level = sorted(set(payload.keys()).difference(required_top_level))
    if unknown_top_level:
        fail(f"root contains unknown keys: {unknown_top_level}")

    if payload["schema_id"] != "gait.registry.ecosystem_index":
        fail("schema_id must equal gait.registry.ecosystem_index")
    if payload["schema_version"] != "1.0.0":
        fail("schema_version must equal 1.0.0")

    expect_type(payload["updated_at"], str, "updated_at")
    if "T" not in payload["updated_at"] or not payload["updated_at"].endswith("Z"):
        fail("updated_at must be RFC3339 UTC (ends with Z)")

    entries = payload["entries"]
    expect_type(entries, list, "entries")
    if not entries:
        fail("entries must not be empty")

    ids: list[str] = []
    for index, raw_entry in enumerate(entries):
        expect_type(raw_entry, dict, f"entries[{index}]")
        validate_entry(raw_entry, index)
        ids.append(raw_entry["id"])

    if len(ids) != len(set(ids)):
        fail("entry ids must be unique")
    if ids != sorted(ids):
        fail("entries must be sorted by id for deterministic diffs")

    print(f"community index validation passed: {path} ({len(entries)} entries)")


def main() -> int:
    if len(sys.argv) > 2:
        print("usage: validate_community_index.py [path]", file=sys.stderr)
        return 2
    path = Path(sys.argv[1]) if len(sys.argv) == 2 else DEFAULT_PATH
    validate(path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
