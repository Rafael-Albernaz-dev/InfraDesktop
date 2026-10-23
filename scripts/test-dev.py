#!/usr/bin/env python3
"""Assert DEV failure causes, bounded observation and owned-process cleanup."""
from pathlib import Path
import os
import json
import re
import sqlite3
import subprocess
import tempfile
import time
import signal
from dev_ownership import Invocation, identity, subreaper

ROOT = Path(__file__).resolve().parents[1]
COMMAND = ['python3', str(ROOT / 'scripts/run-dev.py')]
# daemon startup(5) + observer(10) + shared cleanup(4), with scheduling margin.
OUTER_BUDGET = 24


def wait_log(runner, log, marker):
    deadline = time.monotonic() + 16
    while marker not in log.read_text():
        assert runner.poll() is None, log.read_text()
        assert time.monotonic() < deadline, log.read_text()
        time.sleep(.05)


def assert_cleanup(root, text):
    assert 'DEV cleanup complete' in text, text
    assert not list(root.glob('runtime/r-*/service.sock')), text
    identities = re.findall(r'DEV owned pid=(\d+) start=(\d+)', text)
    assert identities, text
    for pid, start in identities:
        stat = Path('/proc') / pid / 'stat'
        if stat.exists():
            assert stat.read_text().rsplit(')', 1)[1].split()[19] != start, f'owned PID survived: {pid}'
    print('Verified cleanup of ' + str(len(identities)) + ' owned PID identities: ' + json.dumps(identities))


def real_ui_case(crash=False, slow=False):
    with tempfile.TemporaryDirectory(prefix='id-dev-real-') as tmp:
        root = Path(tmp)
        log = root / 'runner.log'
        env = dict(os.environ, INFRADESKTOP_DEV_ROOT=tmp, GDK_BACKEND='x11')
        if slow:
            observer = root / 'slow-observer.py'
            observer.write_text("import runpy,time,sys\nprint('slow observer entered',file=sys.stderr,flush=True)\ntime.sleep(.5)\nrunpy.run_path(" + repr(str(ROOT / 'scripts/ui-ready.py')) + ",run_name='__main__')\n")
            env['INFRADESKTOP_DEV_OBSERVER'] = str(observer)
        invocation = Invocation(COMMAND, env, log, root / 'owned.jsonl')
        runner = invocation.runner
        try:
            wait_log(runner, log, 'DEV UI ready pid=')
            text = log.read_text()
            if slow:
                assert 'slow observer entered' in text, text
            pid = int(text.split('DEV UI ready pid=')[1].split()[0])
            assert int(Path(f'/proc/{pid}/stat').read_text().rsplit(')', 1)[1].split()[1]) == runner.pid
            with sqlite3.connect(root / 'state/infradesktop.db') as database:
                assert database.execute('select count(*) from account').fetchone()[0] == 0
            if crash:
                os.kill(pid, 9)
            else:
                runner.terminate()
            runner.wait(timeout=8)  # shared cleanup is bounded to four seconds
            text = log.read_text()
            if crash:
                assert runner.returncode != 0 and 'DEV UI exited unexpectedly: -9' in text, text
            else:
                assert runner.returncode == 0, text
            assert_cleanup(root, text)
            print('Real rendered UI ' + ('post-ready crash' if crash else 'slow observer shutdown' if slow else 'shutdown') + ': PASS')
        finally:
            invocation.cleanup()


def negative_case(kind):
    with tempfile.TemporaryDirectory(prefix='id-dev-neg-') as tmp:
        root = Path(tmp)
        ui = root / 'ui'
        env = dict(os.environ, INFRADESKTOP_DEV_ROOT=tmp, INFRADESKTOP_DEV_UI=str(ui), GDK_BACKEND='x11')
        if kind != 'missing':
            ui.write_text('#!/bin/sh\n' + ('exit 23\n' if kind == 'broken' else 'exec /bin/sleep 30\n'))
            ui.chmod(0o700)
        if kind.startswith('observer'):
            observer = root / 'observer.py'
            observer.write_text("import time,sys\nprint('observer entered synchronous wait',file=sys.stderr,flush=True)\ntime.sleep(30)\n" if kind != 'observer-fail' else 'raise SystemExit(29)\n')
            env['INFRADESKTOP_DEV_OBSERVER'] = str(observer)
        start = time.monotonic()
        if kind == 'observer-cancel':
            log = root / 'runner.log'
            invocation = Invocation(COMMAND, env, log, root / 'owned.jsonl')
            runner = invocation.runner
            try:
                wait_log(runner, log, 'observer entered synchronous wait')
                runner.terminate()
                runner.wait(timeout=8)
                text = log.read_text()
                assert runner.returncode == 0 and 'DEV UI ready pid=' not in text, text
            finally:
                invocation.cleanup()
        else:
            log = root / 'runner.log'
            invocation = Invocation(COMMAND, env, log, root / 'owned.jsonl')
            result = invocation.runner
            try:
                result.wait(timeout=OUTER_BUDGET)
            finally:
                invocation.cleanup()
            text = log.read_text()
            expected = {'missing': 'DEV UI executable missing:', 'broken': 'DEV UI exited before readiness: 23',
                        'no-ui': 'DEV UI readiness deadline exceeded', 'observer-hang': 'DEV UI readiness deadline exceeded',
                        'observer-fail': 'DEV observer failed before readiness: 29'}[kind]
            assert result.returncode != 0 and expected in text, (kind, text)
            assert 'DEV UI ready pid=' not in text, text
        elapsed = time.monotonic() - start
        assert elapsed < OUTER_BUDGET, elapsed
        assert_cleanup(root, text)
        print(f'{kind}: exact cause/cancellation, cleanup in {elapsed:.2f}s: PASS')


def external_timeout_case(kill_supervisor=False):
    with tempfile.TemporaryDirectory(prefix='id-dev-timeout-') as tmp:
        root = Path(tmp)
        ui = root / 'ui'
        ui.write_text('#!/bin/sh\nsleep 60 &\nwait\n')
        ui.chmod(0o700)
        observer = root / 'observer.py'
        observer.write_text("import time\nprint('observer timeout ready',flush=True)\ntime.sleep(60)\n")
        control = subprocess.Popen(['/bin/sleep', '60'], start_new_session=True)
        control_id = identity(control.pid)
        invocation = Invocation(COMMAND, dict(os.environ, INFRADESKTOP_DEV_ROOT=tmp,
            INFRADESKTOP_DEV_UI=str(ui), INFRADESKTOP_DEV_OBSERVER=str(observer)),
            root / 'runner.log', root / 'owned.jsonl')
        try:
            wait_log(invocation.runner, root / 'runner.log', 'DEV child observer')
            deadline = time.monotonic() + 2
            while len(invocation.journal.read_text().splitlines()) != 3:
                assert time.monotonic() < deadline
                time.sleep(.025)
            os.kill(invocation.runner.pid, signal.SIGSTOP)
            start = time.monotonic()
            try:
                invocation.runner.wait(timeout=.3)
                raise AssertionError('external timeout was not induced')
            except subprocess.TimeoutExpired:
                pass
            if kill_supervisor:
                invocation.runner.kill()
                invocation.runner.wait(timeout=2)
            records, observed = invocation.cleanup()
            assert len(records) == 4 and len(observed) >= (4 if kill_supervisor else 5), (records, observed)
            assert identity(control.pid)['start'] == control_id['start']
            assert control.poll() is None
            print(f'external timeout: supervisor {"killed" if kill_supervisor else "frozen"}; {len(observed)} owned processes recovered in {time.monotonic()-start:.2f}s; unrelated control preserved: PASS')
        finally:
            invocation.cleanup()
            control.terminate()
            control.wait(timeout=2)


def main():
    subreaper()
    with tempfile.TemporaryDirectory(prefix='id-dev-late-') as tmp:
        journal = Path(tmp) / 'closed.jsonl'
        journal.write_text('{"closed":true}\n')
        late = subprocess.run(['python3', str(ROOT / 'scripts/dev_ownership.py'), str(journal), '/bin/sleep', '60'], timeout=2)
        assert late.returncode == 0
        print('late wrapper after external cleanup: no exec: PASS')
    external_timeout_case()
    external_timeout_case(kill_supervisor=True)
    real_ui_case()
    real_ui_case(slow=True)
    for kind in ('missing', 'broken', 'no-ui', 'observer-fail', 'observer-hang', 'observer-cancel'):
        negative_case(kind)
    real_ui_case(crash=True)


if __name__ == '__main__':
    main()
