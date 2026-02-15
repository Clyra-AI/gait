#!/usr/bin/env python3
"""Generate provider-specific submission copies from core repo skills."""

from __future__ import annotations

import shutil
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
SKILLS_ROOT = REPO_ROOT / ".agents" / "skills"
SUBMISSIONS_ROOT = REPO_ROOT / ".agents" / "submissions"
LICENSE_SOURCE = REPO_ROOT / "LICENSE"

SKILL_NAMES = [
    "incident-to-regression",
    "ci-failure-triage",
    "evidence-receipt-generation",
]


def parse_skill(path: Path) -> tuple[dict[str, str], str]:
    text = path.read_text(encoding="utf-8")
    lines = text.splitlines()
    if len(lines) < 3 or lines[0].strip() != "---":
        raise ValueError(f"{path}: missing frontmatter start")
    end = None
    for index in range(1, len(lines)):
        if lines[index].strip() == "---":
            end = index
            break
    if end is None:
        raise ValueError(f"{path}: missing frontmatter end")

    frontmatter: dict[str, str] = {}
    for line in lines[1:end]:
        if not line.strip():
            continue
        if ":" not in line:
            raise ValueError(f"{path}: invalid frontmatter line '{line}'")
        key, value = line.split(":", 1)
        frontmatter[key.strip()] = value.strip()

    body = "\n".join(lines[end + 1 :]).strip() + "\n"
    return frontmatter, body


def render_frontmatter(frontmatter: dict[str, str], provider: str) -> str:
    name = frontmatter["name"]
    description = frontmatter["description"]
    output = ["---", f"name: {name}", f"description: {description}"]
    if provider == "anthropic":
        output.append("license: Apache-2.0")
    output.append("---")
    return "\n".join(output) + "\n\n"


def provider_note(name: str, provider: str) -> str:
    if provider == "openai":
        return (
            "## Provider Notes (OpenAI Codex)\n\n"
            f"- Invoke explicitly as `${name}` when asking Codex to run this workflow.\n"
            "- Keep outputs grounded in command results and `--json` payload fields.\n"
        )

    return (
        "## Provider Notes (Anthropic Claude)\n\n"
        f"- Ask Claude to use the `{name}` skill by name when this workflow applies.\n"
        "- Keep outputs grounded in command results and `--json` payload fields.\n"
    )


def emit_variant(name: str, provider: str) -> None:
    skill_path = SKILLS_ROOT / name / "SKILL.md"
    frontmatter, body = parse_skill(skill_path)

    out_dir = SUBMISSIONS_ROOT / provider / name
    out_dir.mkdir(parents=True, exist_ok=True)

    rendered = render_frontmatter(frontmatter, provider)
    rendered += body.rstrip() + "\n\n"
    rendered += provider_note(name, provider)

    (out_dir / "SKILL.md").write_text(rendered, encoding="utf-8")
    shutil.copy2(LICENSE_SOURCE, out_dir / "LICENSE.txt")


def main() -> int:
    for provider in ("openai", "anthropic"):
        provider_dir = SUBMISSIONS_ROOT / provider
        provider_dir.mkdir(parents=True, exist_ok=True)
        for name in SKILL_NAMES:
            emit_variant(name, provider)

    print(
        f"generated {len(SKILL_NAMES)} skill variants for openai and anthropic under {SUBMISSIONS_ROOT}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
