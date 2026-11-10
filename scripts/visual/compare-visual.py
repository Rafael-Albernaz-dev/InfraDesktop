#!/usr/bin/env python3
"""Whole client pairs, overlays and diffs; freshness and review are separate gates."""
from pathlib import Path
import argparse
import hashlib
import json


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def keyed(rows):
    result = {row['key']: row for row in rows}
    if len(result) != len(rows):
        raise ValueError('duplicate state keys')
    return result


def verify(manifest):
    keyed(manifest['rows'])
    for item in manifest['inputs']:
        if not Path(item['path']).is_file() or sha(item['path']) != item['sha256']:
            raise ValueError('STALE input: ' + item['path'])
    for row in manifest['rows']:
        for key in ('capture', 'reference'):
            if row[key] and sha(row[key]) != row[key + '_sha256']:
                raise ValueError('STALE ' + key + ': ' + row[key])


def verify_comparison(manifest, capture_manifest, out):
    comparison = json.loads((out / 'manifest.json').read_text())
    if comparison['capture_manifest_sha256'] != sha(capture_manifest):
        raise ValueError('STALE comparison')
    expected = keyed([row for row in manifest['rows'] if row['reference']])
    actual = keyed(comparison['rows'])
    if expected.keys() != actual.keys():
        raise ValueError('comparison state set differs from captures')
    for key, row in actual.items():
        for field in ('capture', 'reference', 'capture_sha256', 'reference_sha256'):
            if row[field] != expected[key][field]:
                raise ValueError('comparison capture/reference mismatch: ' + key)
        for field in ('pair', 'overlay', 'diff'):
            if sha(out / row[field]) != row[field + '_sha256']:
                raise ValueError('STALE comparison artifact: ' + key)
    for artifact in comparison.get('diagnostic_artifacts', []):
        if sha(artifact['path']) != artifact['sha256']:
            raise ValueError('STALE render investigation')
    review = out / 'review.json'
    if review.exists():
        if json.loads(review.read_text())['comparison_sha256'] != sha(out / 'manifest.json'):
            raise ValueError('STALE review')
    return comparison


def verify_review(out, comparison):
    if not comparison['rows']:
        raise ValueError('review unavailable: references NOT RUN')
    review = json.loads((out / 'review.json').read_text())
    if review.get('schema') != 1 or not review.get('reviewer') or not review.get('scope'):
        raise ValueError('review schema/reviewer/scope missing')
    if review.get('comparison_sha256') != sha(out / 'manifest.json'):
        raise ValueError('STALE review')
    if not isinstance(review.get('pending'), list):
        raise ValueError('review pending must be explicit')
    rows = keyed(review.get('rows', []))
    if rows.keys() != keyed(comparison['rows']).keys():
        raise ValueError('review state set incomplete')
    verdicts = {'PASS', 'PASS_WITH_AUTHORIZED_DELTA', 'FAIL', 'BLOCKED', 'NOT RUN'}
    for row in rows.values():
        if row.get('verdict') not in verdicts or not isinstance(row.get('differences'), list):
            raise ValueError('review verdict/differences missing: ' + row['key'])
        if not isinstance(row.get('notes'), str) or not row['notes'].strip():
            raise ValueError('review inspection notes missing: ' + row['key'])
    return review


def invalidate_review(out):
    review = out / 'review.json'
    if review.exists():
        review.rename(out / 'review.invalidated.json')


def compare(manifest, capture_manifest, captures, out):
    out.mkdir(parents=True, exist_ok=True)
    invalidate_review(out)
    if any(row['reference'] for row in manifest['rows']):
        from PIL import Image, ImageChops, ImageStat
    rows = []
    diagnostics = []
    for row in manifest['rows']:
        if not row['reference']:
            continue
        reference = Image.open(row['reference']).convert('RGB')
        capture = Image.open(row['capture']).convert('RGB')
        if row['crop']:
            left, top, width, height = row['crop']
            if reference.size != (1440, 900) or row['crop'] != [0, 40, 1440, 860]:
                raise ValueError('unexpected approved client crop')
            reference = reference.crop((left, top, left + width, top + height))
        if list(reference.size) != list(capture.size) or list(capture.size) != row['viewport']:
            raise ValueError('viewport mismatch: ' + row['key'])
        key = row['key']
        names = {kind: key + '-' + kind + '.png' for kind in ('pair', 'overlay', 'diff')}
        Image.blend(reference, capture, .5).save(out / names['overlay'], compress_level=1)
        pair = Image.new('RGB', (reference.width * 2, reference.height))
        pair.paste(reference)
        pair.paste(capture, (reference.width, 0))
        pair.save(out / names['pair'], compress_level=1)
        difference = ImageChops.difference(reference, capture)
        difference.save(out / names['diff'], compress_level=1)
        aligned = captures / (key + '-reference-origin-aligned.png')
        if aligned.exists():
            normalized = Image.open(aligned).convert('RGB')
            if normalized.size != capture.size:
                raise ValueError('aligned diagnostic size mismatch')
            aligned_diff = out / (key + '-origin-aligned-diff.png')
            ImageChops.difference(normalized, capture).save(aligned_diff, compress_level=1)
            diagnostics.extend({'path': str(path), 'sha256': sha(path)} for path in (aligned, aligned_diff))
        rows.append({
            'key': key,
            **{field: row[field] for field in ('reference', 'capture', 'reference_sha256', 'capture_sha256')},
            **names,
            **{kind + '_sha256': sha(out / name) for kind, name in names.items()},
            'descriptive_mean_abs_rgb': ImageStat.Stat(difference).mean,
            'different_pixels': sum(pixel != (0, 0, 0) for pixel in difference.getdata()),
            'verdict': 'REQUIRES_REVIEW',
        })
    computed = captures / 'computed-style-investigation.json'
    if computed.exists():
        diagnostics.append({'path': str(computed), 'sha256': sha(computed)})
    (out / 'manifest.json').write_text(json.dumps({
        'diagnostic_artifacts': diagnostics,
        'capture_manifest_sha256': sha(capture_manifest),
        'acceptance': 'Separate visual inspection; no numerical threshold or masks',
        'rows': rows,
    }, indent=2) + '\n')
    sections = []
    for row in rows:
        images = ''.join('<img src="' + row[kind] + '">' for kind in ('pair', 'overlay', 'diff'))
        sections.append('<section><h2>' + row['key'] + '</h2>' + images + '</section>')
    (out / 'index.html').write_text(
        '<!doctype html><meta charset="utf-8"><title>UI 0.2.1 comparison</title>'
        '<style>body{font:14px sans-serif;background:#eee}img{max-width:100%}'
        'section{background:white;margin:20px;padding:12px}</style>'
        '<h1>Reference left, capture right; whole client overlay and diff</h1>' + ''.join(sections)
    )
    print(f'{len(rows)} current pairs/overlays/diffs; fidelity REQUIRES_REVIEW' if rows else
          'Fidelity NOT RUN: no reference states; capture freshness is separate')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--captures', type=Path, required=True)
    parser.add_argument('--output', type=Path)
    parser.add_argument('--verify-current', action='store_true')
    parser.add_argument('--verify-review', action='store_true')
    args = parser.parse_args()
    capture_manifest = args.captures / 'capture-manifest.json'
    out = args.output or args.captures / 'comparison'
    try:
        manifest = json.loads(capture_manifest.read_text())
        verify(manifest)
        if args.verify_current or args.verify_review:
            comparison = verify_comparison(manifest, capture_manifest, out)
            if args.verify_review:
                review = verify_review(out, comparison)
                print('CURRENT complete review record; scope=' + review['scope'] + '; pending=' + str(review['pending']))
            else:
                print('CURRENT source/bundle/reference/capture/comparison; visual review NOT VERIFIED')
            return
        compare(manifest, capture_manifest, args.captures, out)
    except (ValueError, KeyError, TypeError, FileNotFoundError) as error:
        if out.exists():
            (out / 'STALE.txt').write_text(str(error) + '\n')
        raise SystemExit(str(error))


if __name__ == '__main__':
    main()
