#!/usr/bin/env python3
"""Own display, session bus and runtime for isolated native presentation capture."""
import os,subprocess,tempfile
with tempfile.TemporaryDirectory(prefix='id-wails-visual-runtime-') as runtime:
 os.chmod(runtime,0o700)
 env=dict(os.environ,XDG_RUNTIME_DIR=runtime,GIO_USE_VFS="local")
 env.pop('AT_SPI_BUS_ADDRESS',None)
 result=subprocess.run(['dbus-run-session','--','xvfb-run','-a','-s','-screen 0 2000x1700x24 -nolisten tcp','python3','scripts/test-wails-visual.py'],env=env)
 raise SystemExit(result.returncode)
