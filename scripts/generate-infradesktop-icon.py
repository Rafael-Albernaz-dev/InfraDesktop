#!/usr/bin/env python3
"""Rebuild InfraDesktop icons from shared geometry; requires Pillow (no downloads)."""
from __future__ import annotations
import argparse
import hashlib
import json
from pathlib import Path
from xml.sax.saxutils import escape
from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parents[1]
ASSETS = ROOT / 'packaging/linux/assets'
PROOF = ROOT / 'build/bf01-assets'
SIZES = (512, 256, 128, 64, 48, 32)
CANVAS = 36
TURQUOISE = '#0e7490'
FOREGROUND = '#ffffff'
# Original reference mark: a 26px SVG (viewBox 0 0 32 32), centered in 36px.
GLYPH_SCALE = 26 / 32
GLYPH_OFFSET = 5
STROKE = 1.6 * GLYPH_SCALE
NODES = ((12, 3, 8, 7), (3, 22, 8, 7), (21, 22, 8, 7))
LINES = (((16, 10), (16, 16)), ((7, 22), (7, 16), (25, 16), (25, 22)))


def coord(value):
    return GLYPH_OFFSET + value * GLYPH_SCALE


def shared_geometry():
    rects = [(coord(x), coord(y), w * GLYPH_SCALE, h * GLYPH_SCALE, GLYPH_SCALE)
             for x, y, w, h in NODES]
    paths = [[(coord(x), coord(y)) for x, y in line] for line in LINES]
    return rects, paths


def svg():
    rects, paths = shared_geometry()
    f = lambda x: f'{x:g}'
    parts = [
        '<?xml version="1.0" encoding="UTF-8"?>',
        '<!-- InfraDesktop original geometric network mark; see ORIGIN.md. -->',
        '<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 36 36" role="img" aria-labelledby="title desc">',
        '<title id="title">InfraDesktop</title>',
        '<desc id="desc">Three rectangular network nodes joined by an orthogonal branch on a turquoise rounded square.</desc>',
        f'<rect width="36" height="36" rx="5" fill="{TURQUOISE}"/>',
        f'<g fill="none" stroke="{FOREGROUND}" stroke-width="{f(STROKE)}" stroke-linecap="butt" stroke-linejoin="miter">',
    ]
    for x, y, w, h, r in rects:
        parts.append(f'<rect x="{f(x)}" y="{f(y)}" width="{f(w)}" height="{f(h)}" rx="{f(r)}"/>')
    for line in paths:
        path = 'M' + ' L'.join(f'{f(x)} {f(y)}' for x, y in line)
        parts.append(f'<path d="{escape(path)}"/>')
    return '\n'.join(parts + ['</g>', '</svg>', ''])


def png(size):
    # Supersampling affects antialiasing only; the same coordinates feed SVG and PNG.
    resolution = size * 4
    scale = resolution / CANVAS
    im = Image.new('RGBA', (resolution, resolution), (0, 0, 0, 0))
    d = ImageDraw.Draw(im)
    d.rounded_rectangle((0, 0, resolution - 1, resolution - 1), radius=5 * scale,
                        fill=TURQUOISE)
    rects, paths = shared_geometry()
    stroke = max(1, round(STROKE * scale))
    for x, y, w, h, r in rects:
        # Pillow strokes inside; enlarge boundary by half stroke to center it like SVG.
        half = stroke / 2
        bounds = (x * scale - half, y * scale - half,
                  (x + w) * scale + half, (y + h) * scale + half)
        d.rounded_rectangle(bounds, radius=r * scale + half, outline=FOREGROUND,
                            width=stroke)
    for path in paths:
        points = [(x * scale, y * scale) for x, y in path]
        half = stroke / 2
        # Orthogonal stroke rectangles with square joins reproduce SVG miter joins.
        for (x1, y1), (x2, y2) in zip(points, points[1:]):
            if x1 == x2:
                bounds = (x1 - half, min(y1, y2), x1 + half, max(y1, y2))
            else:
                bounds = (min(x1, x2), y1 - half, max(x1, x2), y1 + half)
            d.rectangle(bounds, fill=FOREGROUND)
        for x, y in points[1:-1]:
            d.rectangle((x - half, y - half, x + half, y + half), fill=FOREGROUND)
    result = im.resize((size, size), Image.Resampling.LANCZOS)
    # Remove a one-level Lanczos halo outside the rounded square.
    alpha = result.getchannel('A').point(lambda value: 0 if value <= 1 else value)
    result.putalpha(alpha)
    return result


def preview():
    im = Image.new('RGB', (960, 520), '#f8fafc')
    d = ImageDraw.Draw(im)
    d.rectangle((480, 0, 960, 520), fill='#020617')
    for origin, color, label in ((0, '#0f172a', 'Fundo claro'), (480, '#f8fafc', 'Fundo escuro')):
        d.text((origin + 24, 18), label, fill=color)
        icon = Image.open(ASSETS / 'infradesktop-256.png')
        im.paste(icon, (origin + 112, 54), icon)
        for size, x in ((128, 20), (64, 174), (48, 270), (32, 354)):
            icon = Image.open(ASSETS / f'infradesktop-{size}.png')
            im.paste(icon, (origin + x, 358), icon)
            d.text((origin + x, 496), f'{size}px / native', fill=color)
    im.save(PROOF / 'preview-light-dark.png')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='Compare generated icon bytes without writing assets')
    args = parser.parse_args()
    import io
    generated = {'infradesktop.svg': svg().encode()}
    for size in SIZES:
        output = io.BytesIO()
        png(size).save(output, format='PNG', optimize=False)
        generated[f'infradesktop-{size}.png'] = output.getvalue()
    if args.check:
        for name, data in generated.items():
            if (ASSETS / name).read_bytes() != data:
                raise SystemExit(f'FAIL: generated bytes differ: {name}')
        print('PASS: SVG and six PNGs reproduce byte-for-byte')
        return
    ASSETS.mkdir(parents=True, exist_ok=True)
    PROOF.mkdir(parents=True, exist_ok=True)
    for name, data in generated.items():
        (ASSETS / name).write_bytes(data)
    preview()
    manifest = {name: hashlib.sha256(data).hexdigest() for name, data in generated.items()}
    manifest['generator'] = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
    (PROOF / 'sha256.json').write_text(json.dumps(manifest, indent=2) + '\n')
    # Browser-rendered comparison sheet, source SVG next to PNG. No remote resources.
    rows = ''.join(f'<div class="pair"><figure><img width="{size}" height="{size}" src="../../packaging/linux/assets/infradesktop.svg"><figcaption>SVG {size}px</figcaption></figure><figure><img width="{size}" height="{size}" src="../../packaging/linux/assets/infradesktop-{size}.png"><figcaption>PNG {size}px</figcaption></figure></div>' for size in (256, 128, 64, 48, 32))
    html = '<!doctype html><meta charset="utf-8"><title>BF01 icon comparison</title><style>body{margin:0;display:flex;font:14px sans-serif}section{width:50%;padding:20px;box-sizing:border-box;background:#f8fafc;color:#0f172a}section.dark{background:#020617;color:#f8fafc}.pair{display:flex;gap:22px;align-items:center;border-bottom:1px solid #64748b55}figure{margin:12px 0;min-width:100px}figcaption{margin:6px 0}img{display:block}</style><section><h1>Claro</h1>' + rows + '</section><section class="dark"><h1>Escuro</h1>' + rows + '</section>'
    (PROOF / 'compare-svg-png.html').write_text(html)
    print('Generated SVG, six transparent PNGs, manifest and local previews')


if __name__ == '__main__':
    main()
