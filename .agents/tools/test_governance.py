"""Regression tests for governance validation, using synthetic temp files."""
import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("governance", Path(__file__).with_name("check_governance.py"))
assert spec is not None and spec.loader is not None
governance = importlib.util.module_from_spec(spec)
spec.loader.exec_module(governance)


class LinkChecks(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.page = self.root / "README.md"

    def errors(self, text):
        self.page.write_text(text)
        return governance.link_errors(self.page, self.root)

    def test_valid_link(self):
        (self.root / "AGENTS.md").write_text("# Rules\n")
        self.assertEqual(self.errors("[Rules](AGENTS.md#rules)"), [])

    def test_missing_file(self):
        self.assertTrue(self.errors("[Missing](missing.md)"))

    def test_missing_heading(self):
        (self.root / "AGENTS.md").write_text("# Rules\n")
        self.assertTrue(self.errors("[Rules](AGENTS.md#absent)"))

    def test_private_docs_dependency(self):
        (self.root / "docs").mkdir()
        (self.root / "docs/context.md").write_text("private")
        self.assertTrue(self.errors("[Private](docs/context.md)"))

    def test_external_and_code_are_not_local_links(self):
        self.assertEqual(self.errors("[Web](https://example.org) `x [sample](absent.md)`"), [])

    def test_repository_escape(self):
        self.assertTrue(self.errors("[Outside](../outside.md)"))


class SkillChecks(unittest.TestCase):
    def test_valid_skill_and_invalid_name(self):
        with tempfile.TemporaryDirectory() as directory:
            folder = Path(directory) / "example"
            folder.mkdir()
            path = folder / "SKILL.md"
            text = "---\nname: example\ndescription: Use when checking examples.\nversion: 0.1.0\n---\n## Quando usar\n## Procedimento\n## Armadilhas\n## Verificação\n"
            path.write_text(text)
            self.assertEqual(governance.skill_errors(path), [])
            path.write_text(text.replace("name: example", "name: wrong"))
            self.assertTrue(governance.skill_errors(path))

    def test_missing_frontmatter(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "SKILL.md"
            path.write_text("body without metadata")
            self.assertTrue(governance.skill_errors(path))


if __name__ == "__main__":
    unittest.main()
