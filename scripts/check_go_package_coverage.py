#!/usr/bin/env python3
from __future__ import annotations

import re
import sys
from pathlib import Path

LINE_RE = re.compile(
    r"^ok\s+(\S+)\s+.*coverage:\s+([0-9]+(?:\.[0-9]+)?)% of statements$"
)


def main() -> int:
    if len(sys.argv) not in (3, 4):
        print(
            "usage: check_go_package_coverage.py <go_test_cover_output.txt> <min_percent> [allowlist_csv]",
            file=sys.stderr,
        )
        return 2

    output_path = Path(sys.argv[1])
    if not output_path.exists():
        print(f"go test coverage output file not found: {output_path}", file=sys.stderr)
        return 2

    try:
        minimum = float(sys.argv[2])
    except ValueError:
        print(f"invalid minimum coverage value: {sys.argv[2]}", file=sys.stderr)
        return 2

    allowlist: set[str] = set()
    if len(sys.argv) == 4 and sys.argv[3].strip():
        allowlist = {item.strip() for item in sys.argv[3].split(",") if item.strip()}

    package_coverage: dict[str, float] = {}
    for raw_line in output_path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        match = LINE_RE.match(line)
        if not match:
            continue
        package, coverage_text = match.groups()
        if package in allowlist:
            continue
        if not package.startswith("github.com/Clyra-AI/gait/"):
            continue
        package_coverage[package] = float(coverage_text)

    if not package_coverage:
        print("no package coverage rows found in go test output", file=sys.stderr)
        return 1

    failures = [(pkg, cov) for pkg, cov in sorted(package_coverage.items()) if cov < minimum]
    if failures:
        print(
            f"Go package coverage check failed (minimum {minimum:.1f}%):",
            file=sys.stderr,
        )
        for pkg, cov in failures:
            print(f"- {pkg}: {cov:.1f}%", file=sys.stderr)
        return 1

    print(
        f"Go package coverage OK: {len(package_coverage)} packages >= {minimum:.1f}%"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
