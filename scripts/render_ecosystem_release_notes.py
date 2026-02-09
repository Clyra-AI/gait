#!/usr/bin/env python3
"""Render deterministic ecosystem release notes from community index."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

DEFAULT_INDEX_PATH = Path("docs/ecosystem/community_index.json")
DEFAULT_OUTPUT_PATH = Path("gait-out/ecosystem_release_notes.md")


def fail(message: str) -> None:
    print(f"ecosystem release render failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def require_str(value: Any, field: str) -> str:
    if not isinstance(value, str) or not value.strip():
        fail(f"{field} must be a non-empty string")
    return value.strip()


def load_index(path: Path) -> dict[str, Any]:
    if not path.exists():
        fail(f"index file not found: {path}")
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError as err:
        fail(f"invalid json: {err}")
    if not isinstance(payload, dict):
        fail("root payload must be a JSON object")
    return payload


def render_markdown(index_payload: dict[str, Any]) -> str:
    schema_id = require_str(index_payload.get("schema_id"), "schema_id")
    schema_version = require_str(index_payload.get("schema_version"), "schema_version")
    updated_at = require_str(index_payload.get("updated_at"), "updated_at")
    raw_entries = index_payload.get("entries")
    if not isinstance(raw_entries, list):
        fail("entries must be a JSON array")
    if not raw_entries:
        fail("entries must not be empty")

    entries: list[dict[str, str]] = []
    for index, raw_entry in enumerate(raw_entries):
        if not isinstance(raw_entry, dict):
            fail(f"entries[{index}] must be a JSON object")
        entries.append(
            {
                "id": require_str(raw_entry.get("id"), f"entries[{index}].id"),
                "kind": require_str(raw_entry.get("kind"), f"entries[{index}].kind"),
                "name": require_str(raw_entry.get("name"), f"entries[{index}].name"),
                "summary": require_str(raw_entry.get("summary"), f"entries[{index}].summary"),
                "repo": require_str(raw_entry.get("repo"), f"entries[{index}].repo"),
                "source": require_str(raw_entry.get("source"), f"entries[{index}].source"),
                "status": require_str(raw_entry.get("status"), f"entries[{index}].status"),
                "integration": str(raw_entry.get("integration", "")).strip(),
            }
        )

    entries.sort(key=lambda item: item["id"])

    kinds = ["adapter", "skill", "policy_pack", "tooling"]
    kind_counts = {kind: sum(1 for entry in entries if entry["kind"] == kind) for kind in kinds}
    source_counts = {
        "official": sum(1 for entry in entries if entry["source"] == "official"),
        "community": sum(1 for entry in entries if entry["source"] == "community"),
    }
    status_values = sorted({entry["status"] for entry in entries})
    status_counts = {
        status: sum(1 for entry in entries if entry["status"] == status) for status in status_values
    }

    lines: list[str] = []
    lines.append("# Ecosystem Release Notes")
    lines.append("")
    lines.append(f"- source index: `{DEFAULT_INDEX_PATH}`")
    lines.append(f"- schema: `{schema_id}` `{schema_version}`")
    lines.append(f"- index updated_at: `{updated_at}`")
    lines.append(f"- total entries: `{len(entries)}`")
    lines.append("")
    lines.append("## Summary")
    lines.append("")
    lines.append(f"- adapters: `{kind_counts['adapter']}`")
    lines.append(f"- skills: `{kind_counts['skill']}`")
    lines.append(f"- policy packs: `{kind_counts['policy_pack']}`")
    lines.append(f"- tooling: `{kind_counts['tooling']}`")
    lines.append(f"- official entries: `{source_counts['official']}`")
    lines.append(f"- community entries: `{source_counts['community']}`")
    for status in status_values:
        lines.append(f"- status `{status}`: `{status_counts[status]}`")
    lines.append("")
    lines.append("## Entries")
    lines.append("")

    for kind in kinds:
        kind_entries = [entry for entry in entries if entry["kind"] == kind]
        if not kind_entries:
            continue
        lines.append(f"### {kind}")
        lines.append("")
        for entry in kind_entries:
            integration = f" integration={entry['integration']}" if entry["integration"] else ""
            lines.append(
                f"- `{entry['id']}` ({entry['status']}, {entry['source']}{integration}) "
                f"[{entry['name']}]({entry['repo']}): {entry['summary']}"
            )
        lines.append("")

    return "\n".join(lines).rstrip() + "\n"


def main() -> int:
    if len(sys.argv) > 3:
        print(
            "usage: render_ecosystem_release_notes.py [community_index.json] [output.md]",
            file=sys.stderr,
        )
        return 2

    index_path = Path(sys.argv[1]) if len(sys.argv) >= 2 else DEFAULT_INDEX_PATH
    output_path = Path(sys.argv[2]) if len(sys.argv) == 3 else DEFAULT_OUTPUT_PATH

    payload = load_index(index_path)
    rendered = render_markdown(payload)

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(rendered, encoding="utf-8")
    print(f"ecosystem release notes written: {output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
