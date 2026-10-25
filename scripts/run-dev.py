#!/usr/bin/env python3
"""Private DEV state and bounded supervision of only this invocation's children."""
from pathlib import Path
import ctypes
import os
import signal
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
READY_BUDGET = 10
CLEANUP_BUDGET = 4


def process_identity(pid):
    try:
        fields = Path(f'/proc/{pid}/stat').read_text().rsplit(')', 1)[1].split()
        return {'pid': pid, 'state': fields[0], 'group': int(fields[2]), 'start': fields[19]}
    except (FileNotFoundError, ProcessLookupError):
        return None


def main():
    os.umask(0o077)
    # Reap orphaned WebKit grandchildren of these isolated process groups.
    libc = ctypes.CDLL(None, use_errno=True)
    if libc.prctl(36, 1, 0, 0, 0) != 0:  # Linux PR_SET_CHILD_SUBREAPER
        raise OSError(ctypes.get_errno(), 'DEV child subreaper unavailable')
    override = os.environ.get('INFRADESKTOP_DEV_ROOT')
    dev_root = Path(override) if override else ROOT / '.local/bf01'
    if not dev_root.is_absolute():
        raise SystemExit('DEV root must be absolute')
    state, client = dev_root / 'state', dev_root / 'client'
    runtime_parent = dev_root / 'runtime' if override else ROOT / '.runtime/bf01'
    for directory in (state, client, runtime_parent):
        directory.mkdir(parents=True, exist_ok=True, mode=0o700)
        metadata = directory.stat()
        if directory.is_symlink() or metadata.st_uid != os.getuid() or metadata.st_mode & 0o777 != 0o700:
            raise SystemExit('unsafe existing DEV directory')
    children = []
    observed = {}
    stopping = False

    def stop(*_):
        nonlocal stopping
        stopping = True

    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, stop)

    def remember_groups():
        groups = {child.pid for child in children}
        for entry in Path('/proc').iterdir():
            if not entry.name.isdigit():
                continue
            item = process_identity(int(entry.name))
            if item and item['group'] in groups:
                observed[(item['pid'], item['start'])] = item

    def launch(label, argv, env, **kwargs):
        journal = os.environ.get('INFRADESKTOP_DEV_OWNERSHIP')
        if journal:
            argv = [sys.executable, str(ROOT / 'scripts/dev_ownership.py'), journal, *argv]
        child = subprocess.Popen(argv, env=env, start_new_session=True, **kwargs)
        children.append(child)
        print(f'DEV child {label} pid={child.pid}', flush=True)
        remember_groups()
        return child

    def signal_groups(sig):
        for child in reversed(children):
            try:
                os.killpg(child.pid, sig)
            except ProcessLookupError:
                pass

    def reap():
        for child in children:
            child.poll()
        for child in children:
            while True:
                try:
                    pid, _ = os.waitpid(-child.pid, os.WNOHANG)
                    if pid == 0:
                        break
                except ChildProcessError:
                    break

    def remaining():
        return [item for (pid, start), item in observed.items()
                if (current := process_identity(pid)) and current['start'] == start]

    with tempfile.TemporaryDirectory(prefix='r-', dir=runtime_parent) as tmp:
        runtime = Path(tmp)
        env = dict(os.environ, NO_AT_BRIDGE='0', GTK_MODULES='atk-bridge',
                   INFRADESKTOP_RUNTIME=str(runtime), INFRADESKTOP_STATE=str(state),
                   INFRADESKTOP_CLIENT_STATE=str(client))
        try:
            daemon = launch('daemon', [str(ROOT / 'build/bin/infradesktopd'), '--runtime',
                                       str(runtime), '--state', str(state)], env)
            deadline = time.monotonic() + 5
            while not (runtime / 'service.sock').exists():
                if stopping:
                    return
                if daemon.poll() is not None or time.monotonic() >= deadline:
                    raise SystemExit('DEV daemon unavailable')
                time.sleep(.05)
            print(f'DEV state: {state}; client: {client}', flush=True)
            ui_path = Path(os.environ.get('INFRADESKTOP_DEV_UI', str(ROOT / 'build/bin/infradesktop')))
            if not ui_path.is_absolute():
                raise SystemExit('DEV UI path must be absolute')
            if not ui_path.exists():
                raise SystemExit('DEV UI executable missing: ' + str(ui_path))
            try:
                ui = launch('ui', [str(ui_path)], env)
            except FileNotFoundError:
                raise SystemExit('DEV UI executable missing: ' + str(ui_path))
            observer_path = Path(os.environ.get('INFRADESKTOP_DEV_OBSERVER', str(ROOT / 'scripts/ui-ready.py')))
            if not observer_path.is_absolute():
                raise SystemExit('DEV observer path must be absolute')
            observer = launch('observer', [sys.executable, str(observer_path), str(ui.pid)], env,
                              stdout=subprocess.PIPE, text=True)
            deadline = time.monotonic() + READY_BUDGET
            while True:
                remember_groups()
                if stopping:
                    return
                if ui.poll() is not None:
                    raise SystemExit('DEV UI exited before readiness: ' + str(ui.returncode))
                if daemon.poll() is not None:
                    raise SystemExit('DEV daemon exited before UI readiness')
                if observer.poll() is not None:
                    text = observer.stdout.read().strip()
                    if observer.returncode != 0 or text != 'rendered pid=' + str(ui.pid):
                        raise SystemExit('DEV observer failed before readiness: ' + str(observer.returncode))
                    break
                if time.monotonic() >= deadline:
                    raise SystemExit('DEV UI readiness deadline exceeded')
                time.sleep(.05)
            print('DEV UI ready pid=' + str(ui.pid), flush=True)
            while ui.poll() is None:
                remember_groups()
                if stopping:
                    return
                if daemon.poll() is not None:
                    raise SystemExit('DEV daemon exited')
                time.sleep(.1)
            if ui.returncode != 0:
                raise SystemExit('DEV UI exited unexpectedly: ' + str(ui.returncode))
        finally:
            remember_groups()
            deadline = time.monotonic() + CLEANUP_BUDGET
            signal_groups(signal.SIGTERM)
            graceful = time.monotonic() + CLEANUP_BUDGET / 2
            while time.monotonic() < graceful:
                reap()
                if not remaining():
                    break
                time.sleep(.05)
            signal_groups(signal.SIGKILL)
            while time.monotonic() < deadline:
                reap()
                if not remaining():
                    break
                time.sleep(.05)
            for item in observed.values():
                print(f"DEV owned pid={item['pid']} start={item['start']}", flush=True)
            if remaining():
                raise SystemExit('DEV cleanup deadline exceeded: ' + str(remaining()))
            print('DEV cleanup complete', flush=True)


if __name__ == '__main__':
    main()
