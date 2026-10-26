"""Command recorder shared by the historical wrappers; preserve earlier attempts."""
from pathlib import Path
import datetime
import json
import os
import subprocess
import sys
import time


def main(default):
    name, *argv = sys.argv[1:]
    if not name or Path(name).name != name or not argv:
        raise SystemExit('expected a gate name and command')
    out = Path(os.environ.get('INFRADESKTOP_EVIDENCE_DIR', default))
    out.mkdir(parents=True, exist_ok=True)
    base = out / name
    index = 1
    while Path(str(base)+'.json').exists() or Path(str(base)+'.log').exists():
        base = out / (name + '-attempt' + str(index))
        index += 1
    start = time.monotonic()
    at = datetime.datetime.now(datetime.timezone.utc).isoformat()
    log = Path(str(base)+'.log')
    with log.open('x') as stream:
        result = subprocess.run(argv, stdout=stream, stderr=subprocess.STDOUT)
    record = {'name': name, 'argv': argv, 'started_utc': at, 'seconds': round(time.monotonic()-start, 3), 'exit': result.returncode}
    Path(str(base)+'.json').write_text(json.dumps(record, indent=2)+'\n')
    print(json.dumps(record))
    print(log.read_text()[-9000:])
    raise SystemExit(result.returncode)
