#!/usr/bin/env python3
"""Synthetic public regressions for freshness, state sets and review completeness."""
from pathlib import Path
import importlib.util
import json
import subprocess
import tempfile
import base64

spec = importlib.util.spec_from_file_location('comparison', Path(__file__).with_name('compare-visual.py'))
comparison = importlib.util.module_from_spec(spec)
spec.loader.exec_module(comparison)


def main():
    with tempfile.TemporaryDirectory(prefix='id-visual-integrity-') as tmp:
        root = Path(tmp)
        source, bundle = root / 'source.ts', root / 'bundle.js'
        capture, reference = root / 'capture.png', root / 'reference.png'
        source.write_text('source')
        bundle.write_text('bundle')
        for path in (capture, reference):
            path.write_bytes(base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAFElEQVR4nGP8////fwYGBgYmBigAAD34BADwR4MaAAAAAElFTkSuQmCC'))
        manifest = {
            'inputs': [{'path': str(path), 'sha256': comparison.sha(path)} for path in (source, bundle)],
            'rows': [{
                'key': key, 'capture': str(capture), 'reference': str(reference),
                'capture_sha256': comparison.sha(capture), 'reference_sha256': comparison.sha(reference),
                'viewport': [2, 2], 'crop': None,
            } for key in ('one', 'two')],
        }
        comparison.verify(manifest)
        for path in (source, bundle, capture, reference):
            before = path.read_bytes()
            path.write_bytes(before + b'changed')
            try:
                comparison.verify(manifest)
            except ValueError:
                pass
            else:
                raise AssertionError('stale artifact accepted: ' + str(path))
            path.write_bytes(before)
        (root / 'capture-manifest.json').write_text(json.dumps(manifest))
        command = ['python3', str(Path(__file__).with_name('compare-visual.py')), '--captures', str(root)]

        def run(flags=(), expected=True, diagnostic=None):
            result = subprocess.run(command + list(flags), capture_output=True, text=True)
            assert (result.returncode == 0) == expected, result.stdout + result.stderr
            if diagnostic:
                assert diagnostic in result.stdout + result.stderr, result.stdout + result.stderr

        out = root / 'comparison'
        out.mkdir()
        # Integrity fixtures are plain public files. The verifier itself uses
        # only stdlib; actual image generation is exercised by test-visual.
        compared = {
            'capture_manifest_sha256': comparison.sha(root / 'capture-manifest.json'),
            'diagnostic_artifacts': [],
            'rows': [{
                **{key: row[key] for key in ('key', 'capture', 'reference', 'capture_sha256', 'reference_sha256')},
                **{kind: '../capture.png' for kind in ('pair', 'overlay', 'diff')},
                **{kind + '_sha256': comparison.sha(capture) for kind in ('pair', 'overlay', 'diff')},
            } for row in manifest['rows']],
        }
        (out / 'manifest.json').write_text(json.dumps(compared))
        review_path = out / 'review.json'
        review_flag = ['--verify-review']
        run(['--verify-current'], diagnostic='visual review NOT VERIFIED')
        run(review_flag, False, 'review.json')
        review = {
            'schema': 1, 'reviewer': 'synthetic regression', 'scope': 'test only',
            'comparison_sha256': comparison.sha(out / 'manifest.json'), 'pending': [],
            'rows': [{'key': key, 'verdict': 'PASS', 'differences': [], 'notes': 'Synthetic pair inspected'} for key in ('one', 'two')],
        }
        review_path.write_text(json.dumps({'comparison_sha256': review['comparison_sha256']}))
        run(['--verify-current'])
        run(review_flag, False, 'schema')
        review_path.write_text(json.dumps({**review, 'rows': review['rows'][:1]}))
        run(review_flag, False, 'state set incomplete')
        review_path.write_text(json.dumps({**review, 'rows': [{'key': 'one', 'verdict': 'PASS'}, review['rows'][1]]}))
        run(review_flag, False, 'differences')
        review_path.write_text(json.dumps(review))
        run(review_flag)
        review_path.write_text(json.dumps({**review, 'comparison_sha256': 'obsolete'}))
        run(['--verify-current'], False, 'STALE review')
        comparison.invalidate_review(out)
        assert not review_path.exists() and (out / 'review.invalidated.json').exists()
        original = (out / 'manifest.json').read_text()
        truncated = json.loads(original)
        truncated['rows'] = truncated['rows'][:1]
        (out / 'manifest.json').write_text(json.dumps(truncated))
        run(['--verify-current'], False, 'state set differs')
        (out / 'manifest.json').write_text(original)
        manifest['rows'][0]['reference'] = None
        manifest['rows'][1]['reference'] = None
        (root / 'capture-manifest.json').write_text(json.dumps(manifest))
        run(diagnostic='Fidelity NOT RUN')
        review_path.write_text(json.dumps({**review, 'rows': [], 'comparison_sha256': comparison.sha(out / 'manifest.json')}))
        run(review_flag, False, 'references NOT RUN')
        print('Freshness, subprocess-independent public states, absent/incomplete/stale review and truncated comparison: PASS')


if __name__ == '__main__':
    main()
