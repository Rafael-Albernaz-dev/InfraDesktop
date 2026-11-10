"""Aceite L-HIGIENE: a versão do produto vem de VERSION e todo espelho concorda.

Escrito por Claude. Copiar para quality/checks/test-version.py sem editar.
Rodar: python3 quality/checks/test-version.py
"""
from pathlib import Path
import json
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
SEMVER = re.compile(r'^\d+\.\d+\.\d+$')


def version():
    return (ROOT / 'VERSION').read_text().strip()


class VersionTests(unittest.TestCase):
    def test_version_file_is_single_semver_line(self):
        raw = (ROOT / 'VERSION').read_text()
        self.assertRegex(raw.strip(), SEMVER)
        self.assertEqual(raw, raw.strip() + '\n', 'VERSION: uma linha, terminada em \\n')

    def test_wails_matches(self):
        info = json.loads((ROOT / 'wails.json').read_text())['info']
        self.assertEqual(info['productVersion'], version())

    def test_frontend_package_and_lock_match(self):
        pkg = json.loads((ROOT / 'frontend/package.json').read_text())
        lock = json.loads((ROOT / 'frontend/package-lock.json').read_text())
        self.assertEqual(pkg['version'], version())
        self.assertEqual(lock['version'], version())
        self.assertEqual(lock['packages']['']['version'], version())

    def test_ipc_product_version_matches(self):
        text = (ROOT / 'internal/adapters/ipc/protocol.go').read_text()
        found = re.findall(r'^const ProductVersion = "([^"]+)"$', text, re.M)
        self.assertEqual(found, [version()])

    def test_package_reads_version_file(self):
        text = (ROOT / 'scripts/build/package.py').read_text()
        self.assertIn("'VERSION'", text, 'package.py precisa ler o arquivo VERSION')
        self.assertIsNone(re.search(r'(?<!Standards-)Version: \d', text), 'versão literal no control')
        self.assertIsNone(re.search(r'_\d+\.\d+\.\d+_amd64', text), 'versão literal no nome do .deb')


if __name__ == '__main__':
    unittest.main()
