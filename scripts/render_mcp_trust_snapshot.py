#!/usr/bin/env python3
"""Render a deterministic local MCP trust snapshot from a local report file."""

from __future__ import annotations

import argparse
import json
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True, help="path to local trust input JSON")
    parser.add_argument("--output", required=True, help="path to rendered trust snapshot JSON")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    input_path = Path(args.input)
    output_path = Path(args.output)
    report = json.loads(input_path.read_text(encoding="utf-8"))

    snapshot = {
        "schema_id": "gait.mcp.trust_snapshot",
        "schema_version": "1.0.0",
        "created_at": report["updated_at"],
        "producer_version": "0.0.0-dev",
        "entries": [
            {
                "server_id": report["server_id"],
                "server_name": report.get("server_name", ""),
                "publisher": report.get("publisher", ""),
                "source": report.get("source", "external"),
                "status": report.get("status", "unknown"),
                "updated_at": report["updated_at"],
                "score": report.get("score", 0.0),
                "evidence_path": str(input_path),
            }
        ],
    }

    output_path.parent.mkdir(parents=True, exist_ok=True)
    output_path.write_text(json.dumps(snapshot, indent=2) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
