#!/usr/bin/env python3
"""Synthetic subprocess failure paths, no GUI or product binary required."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest
from fixture_io import FixtureOutput


class FixtureFraming(unittest.TestCase):
    def exercise(self, body, expected=None):
        with tempfile.TemporaryDirectory(prefix='l06-fixture-io-') as directory:
            socket = Path(directory) / 'owned.sock'
            source = "import socket,signal,os,time,json\ns=socket.socket(socket.AF_UNIX);s.bind(" + repr(str(socket)) + ")\n" + body
            child = subprocess.Popen([sys.executable, '-u', '-c', source], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            pid = child.pid
            at = time.monotonic()
            try:
                framed = FixtureOutput(child, budget=0.25)
                if expected:
                    with self.assertRaisesRegex(AssertionError, expected):
                        framed.json_line('readiness')
                else:
                    self.assertEqual(framed.json_line('readiness'), {'ready': True})
                    self.assertEqual(framed.finish(), {'done': True})
            finally:
                if child.poll() is None:
                    child.terminate()
                    try:
                        child.wait(timeout=1)
                    except subprocess.TimeoutExpired:
                        child.kill()
                        child.wait(timeout=1)
                child.stdout.close()
                child.stderr.close()
                socket.unlink(missing_ok=True)
            self.assertLess(time.monotonic() - at, 2)
            self.assertFalse(Path('/proc', str(pid)).exists())
            self.assertFalse(socket.exists())

    def test_live_silence(self):
        self.exercise('time.sleep(10)', 'deadline \\(silence\\)')

    def test_live_partial(self):
        self.exercise("os.write(1,b'{\\\"ready\\\":');time.sleep(10)", 'deadline \\(partial line\\)')

    def test_premature_exit(self):
        self.exercise('raise SystemExit(3)', 'EOF; child exit=')

    def test_partial_exit(self):
        self.exercise("os.write(1,b'{');raise SystemExit(3)", 'partial line; child exit=')

    def test_positive_readiness_and_termination(self):
        self.exercise("signal.signal(signal.SIGTERM,lambda *_:(print(json.dumps({'done':True}),flush=True),exit(0)))\nprint(json.dumps({'ready':True}),flush=True)\ntime.sleep(10)")

    def test_legacy_unbounded_readiness_causal_red(self):
        # Execute the exact former readline path in a supervised owned parent.
        # SIGTERM unwinds its finally and waits its own silent fixture child.
        source = """import json,subprocess,sys,signal
peer=subprocess.Popen([sys.executable,'-c','import time;time.sleep(10)'],stdout=subprocess.PIPE)
def stop(*_): raise SystemExit(0)
signal.signal(signal.SIGTERM,stop)
print(json.dumps({'fixture_pid':peer.pid}),flush=True)
try:
 fixture=json.loads(peer.stdout.readline())
 print(json.dumps({'unexpected_progress':True}),flush=True)
finally:
 peer.terminate();peer.wait(timeout=1)
"""
        parent=subprocess.Popen([sys.executable,'-u','-c',source],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        fixture_pid=None
        try:
            output=FixtureOutput(parent,budget=0.25)
            fixture_pid=output.json_line('supervisor readiness')['fixture_pid']
            with self.assertRaisesRegex(AssertionError,r'deadline \(silence\)'):
                output.json_line('legacy unbounded readline blocked')
            print('CAUSAL RED: former readline makes no progress on live silence; owned supervisor unwinds cleanup',flush=True)
        finally:
            parent.terminate();parent.wait(timeout=2)
            parent.stdout.close();parent.stderr.close()
        self.assertFalse(Path('/proc',str(parent.pid)).exists())
        self.assertFalse(Path('/proc',str(fixture_pid)).exists())

    def test_final_silence_is_killed_and_waited(self):
        child = subprocess.Popen([sys.executable, '-u', '-c', "import signal,time;signal.signal(signal.SIGTERM,lambda *_:None);print('{}',flush=True);time.sleep(10)"], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        at = time.monotonic()
        try:
            framed = FixtureOutput(child, budget=0.25)
            self.assertEqual(framed.json_line('readiness'), {})
            with self.assertRaisesRegex(AssertionError, 'final counters: fixture deadline'):
                framed.finish()
            self.assertIsNotNone(child.poll())
            self.assertFalse(Path('/proc', str(child.pid)).exists())
            self.assertLess(time.monotonic()-at, 2)
        finally:
            if child.poll() is None: child.kill(); child.wait(timeout=1)
            child.stdout.close(); child.stderr.close()

    def test_C02a_native_fixture_pid_recorded_before_readiness_failure(self):
        import ast
        from types import SimpleNamespace
        scripts=Path(__file__).resolve().parent
        for name in ('test-part1-desktop.py','test-l06-desktop.py'):
            tree=ast.parse((scripts/name).read_text())
            setup=next(n for n in ast.walk(tree) if isinstance(n,ast.Try) and any(isinstance(x,ast.Assign) and any(isinstance(t,ast.Name) and t.id=='peer' for t in x.targets) for x in n.body))
            stop=next(i for i,n in enumerate(setup.body) if isinstance(n,ast.Assign) and any(isinstance(t,ast.Name) and t.id=='argv' for t in n.targets))
            startup=compile(ast.fix_missing_locations(ast.Module(body=setup.body[:stop],type_ignores=[])),name,'exec')
            for ready in (False,True):
                child=subprocess.Popen([sys.executable,'-u','-c',"import time;"+("print('{\"scope\":\"isolated loopback fixture\"}',flush=True);" if ready else '')+"time.sleep(10)"],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
                record={}
                env={'ROOT':scripts.parent,'family':'v7','record':record,'subprocess':SimpleNamespace(Popen=lambda *a,**k:child,PIPE=subprocess.PIPE),'FixtureOutput':lambda peer:FixtureOutput(peer,budget=0.25)}
                try:
                    if ready: exec(startup,env)
                    else:
                        with self.assertRaisesRegex(AssertionError,'deadline'): exec(startup,env)
                    self.assertEqual(record.get('fixture_pid'),child.pid,name)
                finally:
                    child.terminate();child.wait(timeout=1);child.stdout.close();child.stderr.close()
                self.assertFalse(Path('/proc',str(child.pid)).exists())

    def test_C02b_newline_at_N_and_N_plus_one_has_bounded_buffer(self):
        import json
        from fixture_io import MAX_LINE
        for size in (MAX_LINE,MAX_LINE+1):
            line=json.dumps({'x':'x'*(size-9)},separators=(',',':'))
            self.assertEqual(len(line),size-1)
            line=line[:-1]+' }' # JSON whitespace makes the exact requested N.
            self.assertEqual(len(line),size)
            child=subprocess.Popen([sys.executable,'-u','-c','import os;os.write(1,'+repr((line+'\n').encode())+')'],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
            framed=FixtureOutput(child,budget=0.25)
            try:
                if size==MAX_LINE: self.assertEqual(framed.json_line('N')['x'],'x'*(size-9))
                else:
                    with self.assertRaisesRegex(AssertionError,'framing limit'): framed.json_line('N+1')
                    self.assertLessEqual(len(framed.buffer),MAX_LINE+1)
            finally:
                child.wait(timeout=1);child.stdout.close();child.stderr.close()
            self.assertFalse(Path('/proc',str(child.pid)).exists())


if __name__ == '__main__':
    unittest.main()
