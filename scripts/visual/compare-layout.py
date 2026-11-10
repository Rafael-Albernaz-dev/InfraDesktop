"""Exact changed pixels; 1% limit explicitly authorized by L-BASE-1.md F2 item 3."""
import json
import sys
from pathlib import Path
from PIL import Image, ImageChops

reference, capture, output = map(Path, sys.argv[1:])
a, b = (Image.open(p).convert('RGB') for p in (reference, capture))
if a.size != b.size:
    print(json.dumps({'pass': False, 'reason': 'dimensions', 'reference': a.size, 'capture': b.size}))
    sys.exit(1)
diff = ImageChops.difference(a, b)
changed = sum(pixel != (0, 0, 0) for pixel in diff.getdata())
fraction = changed / (a.width * a.height)
pair = Image.new('RGB', (a.width * 2, a.height))
pair.paste(a)
pair.paste(b, (a.width, 0))
pair.save(str(output) + '-pair.png')
Image.blend(a, b, .5).save(str(output) + '-overlay.png')
diff.save(str(output) + '-diff.png')
print(json.dumps({'pass': fraction <= .01, 'changed_pixels': changed, 'fraction': fraction}))
sys.exit(0 if fraction <= .01 else 1)
