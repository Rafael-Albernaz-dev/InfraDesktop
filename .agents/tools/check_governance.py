"""Offline checks for published governance, independent of private docs/."""
from __future__ import annotations

import json
from pathlib import Path
import re
import subprocess
import sys
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[2]
LINK = re.compile(r"\[[^\]\n]+\]\(([^)\s]+)\)")


def visible_markdown(text: str) -> str:
    text = re.sub(r"(?ms)^```.*?^```[^\n]*", "", text)
    return re.sub(r"`[^`\n]+`", "", text)


def link_errors(path: Path, root: Path) -> list[str]:
    errors = []
    for raw in LINK.findall(visible_markdown(path.read_text(encoding="utf-8"))):
        parsed = urlsplit(raw)
        if parsed.scheme or parsed.netloc:
            continue  # Offline check; external availability is not asserted.
        target = (path.parent / unquote(parsed.path)).resolve() if parsed.path else path
        if not target.is_relative_to(root.resolve()):
            errors.append(f"{path.name}: link outside repository: {raw}")
        elif target.relative_to(root.resolve()).parts[:1] == ("docs",):
            errors.append(f"{path.name}: published link depends on private docs/: {raw}")
        elif not target.exists():
            errors.append(f"{path.name}: missing target: {raw}")
        elif parsed.fragment and target.suffix == ".md":
            headings = re.findall(r"(?m)^#{1,6}\s+(.+?)\s*$", target.read_text(encoding="utf-8"))
            anchors = {re.sub(r"[^\w\- ]", "", h.lower()).replace(" ", "-") for h in headings}
            if unquote(parsed.fragment) not in anchors:
                errors.append(f"{path.name}: missing heading: {raw}")
    return errors


def skill_errors(path: Path) -> list[str]:
    text = path.read_text(encoding="utf-8")
    if not text.startswith("---\n") or "\n---\n" not in text[4:]:
        return [f"{path.parent.name}: missing frontmatter"]
    front, body = text[4:].split("\n---\n", 1)
    # Deliberately supports the scalar frontmatter used here, not arbitrary YAML.
    fields = dict(re.findall(r"(?m)^([a-z_]+):\s*(.+)$", front))
    errors = []
    if fields.get("name") != path.parent.name:
        errors.append(f"{path.parent.name}: name/directory mismatch")
    description = fields.get("description", "").strip('"\'')
    if not description or len(description) > 60 or not description.endswith("."):
        errors.append(f"{path.parent.name}: invalid description")
    if not re.fullmatch(r"\d+\.\d+\.\d+", fields.get("version", "")):
        errors.append(f"{path.parent.name}: missing semantic version")
    for section in ["## Quando usar", "## Procedimento", "## Armadilhas", "## Verificação"]:
        if section not in body:
            errors.append(f"{path.parent.name}: missing {section}")
    return errors


def check(root: Path) -> tuple[list[str], int]:
    errors = []
    for name in ["README.md", "AGENTS.md", ".gitignore", ".agents/SKILLS.md"]:
        if not (root / name).is_file():
            errors.append(f"missing required file: {name}")
    public = [root / "README.md", root / "AGENTS.md"] + sorted((root / ".agents").rglob("*.md"))
    for path in public:
        if path.is_file():
            errors.extend(link_errors(path, root))
    skills = sorted((root / ".agents/skills").glob("*/SKILL.md"))
    if not skills:
        errors.append("no project skills found")
    catalog = (root / ".agents/SKILLS.md").read_text(encoding="utf-8") if (root / ".agents/SKILLS.md").exists() else ""
    for path in skills:
        errors.extend(skill_errors(path))
        if f"skills/{path.parent.name}/SKILL.md" not in catalog:
            errors.append(f"uncatalogued skill: {path.parent.name}")
    if (root / ".gitignore").is_file() and "/docs/" not in (root / ".gitignore").read_text().splitlines():
        errors.append("private docs/ ignore rule missing")
    agents = (root / "AGENTS.md").read_text() if (root / "AGENTS.md").exists() else ""
    if "InfraDesktop — guardrail obrigatório ARQ-001:" not in agents:
        errors.append("portable ARQ-001 prompt block missing")
    tracked = subprocess.run(["git", "ls-files", "-z", "--", "docs"], cwd=root, capture_output=True)
    if tracked.returncode:
        errors.append("cannot inspect Git index")
    elif tracked.stdout:
        errors.append("private docs/ files present in Git index")
    workflow = root / ".github/workflows/ci.yml"
    if not workflow.is_file():
        errors.append("CI workflow missing")
    else:
        uses = re.findall(r"uses:\s*(\S+)", workflow.read_text())
        if not uses or any(not re.fullmatch(r"[\w-]+/[\w-]+@[0-9a-f]{40}", use) for use in uses):
            errors.append("CI actions must use full commit SHA pins")
    return errors, len(public)


def main() -> int:
    errors, count = check(ROOT)
    print(json.dumps({"markdown_files": count, "errors": errors, "scope": "published governance only; docs/ not required"}, ensure_ascii=False, indent=2))
    return bool(errors)


if __name__ == "__main__":
    sys.exit(main())
