#!/usr/bin/env python3
"""Exercise only owned daemon processes, Unix runtime and signal cleanup."""
import json
import os
from pathlib import Path
import signal
import socket
import struct
import subprocess
import tempfile
import time
import uuid

ROOT = Path(__file__).resolve().parents[1]


def request(runtime, payload):
    body = json.dumps(payload).encode()
    with socket.socket(socket.AF_UNIX) as conn:
        conn.settimeout(3)
        conn.connect(str(runtime / 'service.sock'))
        conn.sendall(struct.pack('!I', len(body)) + body)
        def exact(n):
            chunks = bytearray()
            while len(chunks) < n:
                part = conn.recv(n - len(chunks))
                if not part:
                    raise RuntimeError('truncated response')
                chunks.extend(part)
            return chunks
        n = struct.unpack('!I', exact(4))[0]
        assert 0 < n <= 65536
        return json.loads(exact(n))


def account(runtime, op, **fields):
    payload={'version':1,'contract_revision':3,'request_id':'process-test','op':op,'capability':(runtime/'capability').read_text()}
    if op=='company.create' and 'operation_id' not in fields:
        fields['operation_id']=str(uuid.uuid4())
    payload.update(fields)
    return request(runtime,payload)


def status(runtime):
    response=account(runtime,'access.bootstrap')
    assert response['code']=='OK'
    return response['data']


def start(runtime,state=None):
    if state is None:
        state=runtime.parent/'state'
    state.mkdir(mode=0o700,exist_ok=True)
    command=[str(ROOT/'build/bin/infradesktopd'),'--runtime',str(runtime),'--state',str(state)]
    print('owned daemon argv: '+json.dumps(command),flush=True)
    daemon=subprocess.Popen(command,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
    try:
        until=time.monotonic()+5
        while time.monotonic()<until:
            if daemon.poll() is not None:
                raise RuntimeError('daemon did not start: '+daemon.stderr.read())
            if (runtime/'service.sock').exists():
                try:
                    status(runtime)
                    return daemon
                except (OSError,KeyError):pass
            time.sleep(0.02)
        raise RuntimeError('daemon readiness exceeded 5s')
    except BaseException:
        stop(daemon)
        raise


def stop(daemon, sig=signal.SIGTERM):
    if daemon.poll() is None:
        began = time.monotonic()
        daemon.send_signal(sig)
        try:
            daemon.wait(timeout=5)
        except subprocess.TimeoutExpired:
            daemon.kill()
            daemon.wait()
            raise AssertionError('owned process exceeded 5s shutdown budget')
        assert daemon.returncode == 0, daemon.stderr.read()
        print(f'owned daemon pid={daemon.pid} signal={sig.name} exit=0 shutdown={time.monotonic()-began:.3f}s')


def main():
    import sqlite3
    with tempfile.TemporaryDirectory(prefix='infradesktop-bf01-process-') as tmp:
        root=Path(tmp);runtime=root/'runtime';runtime.mkdir(mode=0o700);state=root/'state'
        daemon=start(runtime,state)
        def call(op,want='OK',**kw):
            r=account(runtime,op,**kw);assert r['code']==want,(op,r['code'],want);return r.get('data',{})
        def scope(v):return dict(session=v['session'],profile_id=v.get('profile_id',''),activation_id=v['activation_id'])
        try:
            before=status(runtime);assert before['setup']
            reg=call('access.register',name='process-technical',password='isolated process password')
            recovery=reg['recovery'];assert len(recovery)==64
            login=call('access.login',name='process-technical',password='isolated process password',remember=True)
            grant=login['grant'];session=scope(login)
            companies=call('company.create',**session,name='Process Alpha')['companies'];alpha=companies[0]['id']
            companies=call('company.create',**session,name='Process Beta')['companies'];beta=companies[1]['id']
            selected=call('company.select',**session,name=alpha);session.update(profile_id=alpha,activation_id=selected['activation_id'])
            call('preferences.theme',**session,theme='dark')
            second=subprocess.run([str(ROOT/'build/bin/infradesktopd'),'--runtime',str(runtime),'--state',str(state)],capture_output=True,text=True,timeout=5)
            assert second.returncode==1 and status(runtime)['instance_id']==before['instance_id']
            stop(daemon);daemon=start(runtime,state)
            assert status(runtime)['instance_id']!=before['instance_id']
            call('access.snapshot','UNAUTHENTICATED',**session)
            resumed=call('access.resume',grant=grant);assert resumed['profile_id']==alpha and resumed['theme']=='dark' and len(resumed['companies'])==2
            session=scope(resumed)
            selected=call('company.select',**session,name=beta)
            call('access.snapshot','PROFILE_CHANGED',**session)
            session.update(profile_id=beta,activation_id=selected['activation_id'])
            call('access.logout',**session)
            stop(daemon);daemon=start(runtime,state);call('access.resume','UNAUTHENTICATED',grant=grant)
            login=call('access.login',name='process-technical',password='isolated process password',remember=True);old_grant=login['grant'];old_scope=scope(login)
            reset=call('access.recover',name='process-technical',password='new isolated process password',recovery=recovery)
            assert reset['recovery']!=recovery
            call('access.snapshot','UNAUTHENTICATED',**old_scope)
            stop(daemon);daemon=start(runtime,state)
            call('access.resume','UNAUTHENTICATED',grant=old_grant)
            call('access.recover','UNAUTHENTICATED',name='process-technical',password='new isolated process password',recovery=recovery)
            # Wait for the documented persisted 1s backoff, not an arbitrary retry timeout.
            time.sleep(1.05)
            call('access.login','UNAUTHENTICATED',name='process-technical',password='isolated process password')
            time.sleep(2.05)
            ephemeral=call('access.login',name='process-technical',password='new isolated process password',remember=False)
            assert 'grant' not in ephemeral
            stop(daemon);daemon=start(runtime,state)
            call('access.snapshot','UNAUTHENTICATED',**scope(ephemeral))
            with sqlite3.connect(state/'infradesktop.db') as db:
                assert db.execute('select count(*) from account').fetchone()[0]==1
                assert db.execute('select count(*) from grants').fetchone()[0]==0
                assert db.execute('select count(*) from companies').fetchone()[0]==2
                assert db.execute('select profile,theme from account').fetchone()==(beta,'dark')
            print('PASS: account/recovery, durable grants/revocation, remembered vs ephemeral, companies/theme, real daemon restart and SQLite')
        finally:stop(daemon)
        assert not (runtime/'service.sock').exists() and not (runtime/'capability').exists()
    print('BF01 owned process journey PASS; no system service or human account changed')


if __name__=='__main__':main()
