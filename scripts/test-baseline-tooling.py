#!/usr/bin/env python3
"""Public, isolated regression checks for evidence wrappers and visual geometry."""
from pathlib import Path
import hashlib
import json
import os
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]


class EvidenceWrappers(unittest.TestCase):
    def test_clean_directory_and_exit_propagation(self):
        for script, default in [('part1-gate.py', 'l04-l05-parte1'), ('run-evidence.py', 'bf01')]:
            with self.subTest(script=script), tempfile.TemporaryDirectory(prefix='id-wrapper-') as tmp:
                work = Path(tmp)
                env = dict(os.environ)
                env.pop('INFRADESKTOP_EVIDENCE_DIR', None)
                for status in [7, 0]:
                    result = subprocess.run([sys.executable, str(ROOT/'scripts'/script), 'gate', sys.executable, '-c', f'print("owned fixture {status}"); raise SystemExit({status})'], cwd=work, env=env, capture_output=True, text=True)
                    self.assertEqual(result.returncode, status, result.stdout + result.stderr)
                records = sorted((work/'docs/evidence'/default).glob('gate*.json'))
                self.assertEqual(len(records), 2)
                self.assertEqual(sorted(json.loads(p.read_text())['exit'] for p in records), [0, 7])
                self.assertIn('owned fixture 7', (work/'docs/evidence'/default/'gate.log').read_text())

    def test_explicit_output(self):
        with tempfile.TemporaryDirectory(prefix='id-wrapper-out-') as tmp:
            work = Path(tmp)
            env = dict(os.environ, INFRADESKTOP_EVIDENCE_DIR=str(work/'new/nested/output'))
            for script in ['part1-gate.py', 'run-evidence.py']:
                result = subprocess.run([sys.executable, str(ROOT/'scripts'/script), script, sys.executable, '-c', 'print("owned fixture")'], cwd=work, env=env, capture_output=True, text=True)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            self.assertFalse((work/'docs').exists())
            self.assertEqual(len(list((work/'new/nested/output').glob('*.json'))), 2)


class VisualGeometry(unittest.TestCase):
    def test_crop_and_viewport_contract(self):
        from PIL import Image
        with tempfile.TemporaryDirectory(prefix='id-compare-') as tmp:
            work = Path(tmp)
            for name in ['capture', 'reference']:
                Image.new('RGB', (8, 8), 'white').save(work/(name+'.png'))
            def digest(p):
                return hashlib.sha256(p.read_bytes()).hexdigest()
            row = {'key': 'synthetic', 'viewport': [8, 8], 'crop': None}
            for name in ['capture', 'reference']:
                p = work/(name+'.png')
                row.update({name: str(p), name+'_sha256': digest(p)})
            for crop, viewport, expected in [(None, [8,8], 0), ([0,0,8,8], [8,8], 1), (None, [9,8], 1)]:
                row.update(crop=crop, viewport=viewport)
                (work/'capture-manifest.json').write_text(json.dumps({'inputs': [], 'rows': [row]}))
                result = subprocess.run([sys.executable, str(ROOT/'scripts/compare-visual.py'), '--captures', str(work)], capture_output=True, text=True)
                self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
                if expected:
                    self.assertIn('crop' if crop else 'viewport mismatch', result.stdout + result.stderr)


if __name__ == '__main__':
    unittest.main()
