#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any


MERMAID_KEYWORDS = (
    "flowchart",
    "graph",
    "sequencediagram",
    "classdiagram",
    "statediagram",
    "erdiagram",
    "journey",
    "gantt",
    "pie",
    "mindmap",
    "timeline",
    "gitgraph",
)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Validate docs-site link and mermaid readiness checks.")
    parser.add_argument(
        "--report",
        default="gait-out/docs_site_validation_report.json",
        help="output report path",
    )
    return parser.parse_args()


def collect_markdown_sources(repo_root: Path) -> list[Path]:
    docs_root = repo_root / "docs"
    sources = sorted(path for path in docs_root.rglob("*.md") if path.is_file())
    for root_doc in ("README.md", "SECURITY.md", "CONTRIBUTING.md"):
        path = repo_root / root_doc
        if path.exists():
            sources.append(path)
    return sources


def slug_for_doc(repo_root: Path, path: Path) -> str:
    docs_root = repo_root / "docs"
    if path.parent == repo_root:
        if path.name == "README.md":
            return "start-here"
        if path.name == "SECURITY.md":
            return "security"
        if path.name == "CONTRIBUTING.md":
            return "contributing"
    relative = path.relative_to(docs_root).as_posix()
    slug = relative[:-3].lower()
    if slug == "readme":
        return "start-here"
    return slug


def collect_known_slugs(repo_root: Path, sources: list[Path]) -> set[str]:
    slugs = {slug_for_doc(repo_root, path) for path in sources}
    slugs.add("")
    return slugs


def normalize_link_target(raw: str) -> str:
    target = raw.strip()
    if target.startswith("<") and target.endswith(">"):
        target = target[1:-1].strip()
    if " " in target and not target.startswith("http"):
        target = target.split(" ", 1)[0]
    return target


def strip_fragment_and_query(target: str) -> str:
    no_fragment = target.split("#", 1)[0]
    no_query = no_fragment.split("?", 1)[0]
    return no_query


def validate_navigation_routes(
    repo_root: Path,
    known_slugs: set[str],
    failures: list[str],
    checked_routes: list[str],
) -> None:
    nav_path = repo_root / "docs-site" / "src" / "lib" / "navigation.ts"
    pattern = re.compile(r"href:\s*'(/docs[^']*)'")
    for route in pattern.findall(nav_path.read_text(encoding="utf-8")):
        checked_routes.append(route)
        if route == "/docs":
            continue
        slug = route.removeprefix("/docs/").strip("/").lower()
        if slug not in known_slugs:
            failures.append(f"navigation route missing doc source: {route}")


def has_balanced_pairs(text: str, open_char: str, close_char: str) -> bool:
    depth = 0
    for char in text:
        if char == open_char:
            depth += 1
        elif char == close_char:
            depth -= 1
            if depth < 0:
                return False
    return depth == 0


def validate_mermaid(file_path: Path, content: str, failures: list[str]) -> int:
    blocks = re.findall(r"```mermaid\s*\n(.*?)\n```", content, flags=re.IGNORECASE | re.DOTALL)
    for index, block in enumerate(blocks, start=1):
        trimmed = block.strip()
        lowered = trimmed.lower()
        if not trimmed:
            failures.append(f"{file_path}: mermaid block #{index} is empty")
            continue
        if not any(keyword in lowered for keyword in MERMAID_KEYWORDS):
            failures.append(f"{file_path}: mermaid block #{index} missing diagram keyword")
        if not has_balanced_pairs(trimmed, "(", ")"):
            failures.append(f"{file_path}: mermaid block #{index} has unbalanced parentheses")
        if not has_balanced_pairs(trimmed, "[", "]"):
            failures.append(f"{file_path}: mermaid block #{index} has unbalanced brackets")
        if not has_balanced_pairs(trimmed, "{", "}"):
            failures.append(f"{file_path}: mermaid block #{index} has unbalanced braces")
    return len(blocks)


def validate_markdown_links(
    repo_root: Path,
    source: Path,
    content: str,
    known_slugs: set[str],
    failures: list[str],
) -> int:
    link_count = 0
    for raw_target in re.findall(r"\[[^\]]+\]\(([^)]+)\)", content):
        target = normalize_link_target(raw_target)
        if not target:
            continue
        if target.startswith(("#", "http://", "https://", "mailto:", "tel:")):
            continue
        if target.startswith("javascript:"):
            failures.append(f"{source}: javascript link target is not allowed: {target}")
            continue

        target_no_fragment = strip_fragment_and_query(target)
        if not target_no_fragment:
            continue

        link_count += 1
        if target_no_fragment.startswith("/docs/"):
            route_slug = target_no_fragment.removeprefix("/docs/").strip("/").lower()
            if route_slug not in known_slugs:
                failures.append(f"{source}: broken docs route target {target}")
            continue

        if ".md" in target_no_fragment.lower():
            if target_no_fragment.startswith("/"):
                resolved = (repo_root / target_no_fragment.lstrip("/")).resolve()
            else:
                resolved = (source.parent / target_no_fragment).resolve()
            if not resolved.exists():
                failures.append(f"{source}: missing markdown target {target}")
            continue

        if target_no_fragment.startswith("/"):
            candidate = (repo_root / target_no_fragment.lstrip("/")).resolve()
            if not candidate.exists():
                failures.append(f"{source}: missing repo target {target}")
    return link_count


def main() -> int:
    args = parse_args()
    repo_root = Path(__file__).resolve().parents[1]
    sources = collect_markdown_sources(repo_root)
    known_slugs = collect_known_slugs(repo_root, sources)

    failures: list[str] = []
    checked_routes: list[str] = []
    validate_navigation_routes(repo_root, known_slugs, failures, checked_routes)

    total_links = 0
    total_mermaid_blocks = 0
    for source in sources:
        content = source.read_text(encoding="utf-8")
        total_links += validate_markdown_links(repo_root, source, content, known_slugs, failures)
        total_mermaid_blocks += validate_mermaid(source, content, failures)

    report: dict[str, Any] = {
        "schema_id": "gait.docs.site.validation_report",
        "schema_version": "1.0.0",
        "sources_checked": len(sources),
        "navigation_routes_checked": len(checked_routes),
        "links_checked": total_links,
        "mermaid_blocks_checked": total_mermaid_blocks,
        "failures": failures,
        "status": "pass" if not failures else "fail",
    }

    report_path = Path(args.report)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")

    if failures:
        print("docs-site validation failed:", file=sys.stderr)
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        return 1

    print(
        "docs-site validation passed "
        f"(sources={len(sources)} links={total_links} mermaid_blocks={total_mermaid_blocks})"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
