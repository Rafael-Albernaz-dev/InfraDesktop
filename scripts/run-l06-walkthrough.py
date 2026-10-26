#!/usr/bin/env python3
"""Interactive isolated L06 journey. No installed daemon or human state is used."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import secrets
import select
import shutil
import signal
import subprocess
import tempfile
import time
from fixture_io import FixtureOutput

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--family', choices=['v6', 'v7'], default='v7')
    parser.add_argument('--smoke-seconds', type=float, help='Open only; stop automatically without authentication (runner validation).')
    args = parser.parse_args()
    if args.smoke_seconds is not None and not 1 <= args.smoke_seconds <= 30:
        parser.error('smoke duration must be between 1 and 30 seconds')
    os.umask(0o077)
    spec = importlib.util.spec_from_file_location('l06_native_runtime', ROOT / 'scripts/test-desktop.py')
    native = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(native)
    root = Path(tempfile.mkdtemp(prefix='infradesktop-l06-walkthrough-'))
    for name in ['runtime', 'state', 'client']:
        (root / name).mkdir(mode=0o700)
    key = root / 'synthetic-master'
    key.write_bytes(secrets.token_bytes(32))
    peer = daemon = ui = None
    stopped = False

    def stop(*_):
        nonlocal stopped
        stopped = True
    for sig in (signal.SIGINT, signal.SIGTERM):
        signal.signal(sig, stop)
    try:
        peer = subprocess.Popen([str(ROOT / 'build/bin/part1-ssh-fixture'), '--family', args.family, '--mode', 'interfaces', '--realtime'], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        output = FixtureOutput(peer)
        fixture = output.json_line('walkthrough fixture')
        assert fixture['scope'] == 'isolated loopback fixture'
        env = dict(os.environ, INFRADESKTOP_DEV_VAULT_KEY=str(key))
        env.pop('CREDENTIALS_DIRECTORY', None)
        daemon = subprocess.Popen([str(ROOT / 'build/bin/infradesktopd-part1-test'), '--runtime', str(root / 'runtime'), '--state', str(root / 'state')], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        native.wait_for('walkthrough daemon', lambda: (root / 'runtime/service.sock').exists() if daemon.poll() is None else False)
        ui, app = native.open_ui(root / 'runtime', root / 'client', owned_home=root / 'home', viewport=(1440,860))
        print(json.dumps({'scope':'SYNTHETIC LOOPBACK ONLY; not RouterOS firmware acceptance','root':str(root),'ui_pid':ui.pid,'daemon_pid':daemon.pid,'fixture_pid':peer.pid,'address':'127.0.0.1','port':fixture['port'],'fingerprint':fixture['fingerprint'],'ssh_user':'reader','ssh_password':'local-fixture-only'}, indent=2), flush=True)
        print('Create a NEW synthetic account/company. Add this loopback device, accept its displayed pin. Follow docs/evidence/L06-LOOP-NOTURNO.md. Commands here: r + Enter reopens the same synthetic client; q + Enter or Ctrl+C cleans only this invocation. All synthetic state is deleted at cleanup.', flush=True)
        deadline = time.monotonic() + args.smoke_seconds if args.smoke_seconds else None
        while not stopped and daemon.poll() is None:
            if deadline and time.monotonic() >= deadline:
                break
            if args.smoke_seconds:
                time.sleep(.1)
            elif select.select([0], [], [], .1)[0]:
                command = input().strip().lower()
                if command == 'q':
                    break
                if command == 'r':
                    native.terminate_ui(ui)
                    ui, app = native.open_ui(root / 'runtime', root / 'client', owned_home=root / 'home', viewport=(1440,860))
                    print('Reopened same synthetic state; no network refresh requested by runner.', flush=True)
            if ui.poll() is not None and args.smoke_seconds:
                raise RuntimeError('UI closed before smoke deadline')
    finally:
        if ui:
            native.terminate_ui(ui)
        if daemon:
            native.lifecycle.stop(daemon)
        if peer and peer.poll() is None:
            counters = output.finish()
            print('Synthetic peer cleanup: '+json.dumps(counters), flush=True)
        live = [p.pid for p in [ui,daemon,peer] if p and p.poll() is None]
        if live or (root / 'runtime/service.sock').exists():
            raise RuntimeError('owned cleanup failed: '+str(live))
        shutil.rmtree(root)
        print('Owned processes stopped; synthetic root removed. Human service untouched.', flush=True)


if __name__ == '__main__':
    main()
