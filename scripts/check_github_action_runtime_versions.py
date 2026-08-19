#!/usr/bin/env python3
from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable


@dataclass(frozen=True)
class Rule:
    minimum_major: int
    immutable_sha: bool = False


@dataclass(frozen=True)
class Violation:
    path: str
    line: int
    reference: str
    message: str


RULES: dict[str, Rule] = {
    "actions/checkout": Rule(minimum_major=5),
    "actions/setup-go": Rule(minimum_major=6),
    "actions/setup-python": Rule(minimum_major=6),
    "actions/setup-node": Rule(minimum_major=5),
    "github/codeql-action/init": Rule(minimum_major=4),
    "github/codeql-action/analyze": Rule(minimum_major=4),
    "actions/download-artifact": Rule(minimum_major=4, immutable_sha=True),
}

USES_PATTERN = re.compile(
    r"uses:\s*['\"]?([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)?@[^ \t\r\n#'\"]+)"
)
MAJOR_PATTERN = re.compile(r"^v(?P<major>[0-9]+)(?:[._-].*)?$")
SHA_PATTERN = re.compile(r"^[0-9a-f]{40}$")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Fail deterministically when monitored GitHub Actions references use deprecated "
            "runtime-backed major versions."
        )
    )
    parser.add_argument(
        "paths",
        nargs="+",
        help="Workflow directories or files to scan for monitored GitHub Actions references.",
    )
    return parser.parse_args()


def iter_files(paths: Iterable[str]) -> list[Path]:
    expanded: set[Path] = set()
    for raw_path in paths:
        path = Path(raw_path)
        if not path.exists():
            raise SystemExit(f"workflow runtime guard: path does not exist: {raw_path}")
        if path.is_dir():
            for candidate in sorted(child for child in path.rglob("*") if child.is_file()):
                expanded.add(candidate)
            continue
        expanded.add(path)
    return sorted(expanded)


def parse_major(reference: str) -> int | None:
    match = MAJOR_PATTERN.match(reference)
    if match is None:
        return None
    return int(match.group("major"))


def scan_file(path: Path) -> list[Violation]:
    violations: list[Violation] = []
    for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
        for match in USES_PATTERN.finditer(line):
            reference = match.group(1)
            action_name, version_ref = reference.split("@", 1)
            rule = RULES.get(action_name)
            if rule is None:
                continue
            if rule.immutable_sha and SHA_PATTERN.fullmatch(version_ref):
                continue
            major = parse_major(version_ref)
            if major is None:
                violations.append(
                    Violation(
                        path=str(path),
                        line=line_number,
                        reference=reference,
                        message=(
                            f"unsupported monitored ref format; require {action_name}@v"
                            f"{rule.minimum_major}+"
                            + (" or a 40-character commit SHA" if rule.immutable_sha else "")
                        ),
                    )
                )
                continue
            if major < rule.minimum_major:
                violations.append(
                    Violation(
                        path=str(path),
                        line=line_number,
                        reference=reference,
                        message=(
                            f"deprecated major v{major}; require {action_name}@v"
                            f"{rule.minimum_major}+"
                        ),
                    )
                )
            elif rule.immutable_sha:
                violations.append(
                    Violation(
                        path=str(path),
                        line=line_number,
                        reference=reference,
                        message=(
                            f"mutable ref; require {action_name}@<40-character commit SHA>"
                        ),
                    )
                )
    return violations


def main() -> int:
    args = parse_args()
    files = iter_files(args.paths)
    violations: list[Violation] = []
    for path in files:
        violations.extend(scan_file(path))

    if violations:
        print("workflow runtime guard: fail")
        for violation in sorted(violations, key=lambda item: (item.path, item.line, item.reference)):
            print(
                f"{violation.path}:{violation.line}: {violation.reference}: {violation.message}"
            )
        return 1

    print(f"workflow runtime guard: pass ({len(files)} files scanned)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
