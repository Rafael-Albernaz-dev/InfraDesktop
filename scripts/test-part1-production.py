#!/usr/bin/env python3
"""Prove the uninstalled production binary never falls back to the test key."""
from pathlib import Path
import importlib.util,os,subprocess,tempfile,uuid
spec=importlib.util.spec_from_file_location('process_test',Path('scripts/test-process.py'));lifecycle=importlib.util.module_from_spec(spec);spec.loader.exec_module(lifecycle)
with tempfile.TemporaryDirectory(prefix='id-part1-production-') as tmp:
 root=Path(tmp);runtime=root/'runtime';state=root/'state';runtime.mkdir(mode=0o700);state.mkdir(mode=0o700)
 key=root/'test-key';key.write_bytes(bytes([51])*32);key.chmod(0o600)
 env=dict(os.environ,INFRADESKTOP_DEV_VAULT_KEY=str(key));env.pop('CREDENTIALS_DIRECTORY',None)
 daemon=subprocess.Popen([str(lifecycle.ROOT/'build/bin/infradesktopd'),'--runtime',str(runtime),'--state',str(state)],env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
 try:
  import time
  until=time.monotonic()+5
  while not (runtime/'service.sock').exists():
   assert daemon.poll() is None and time.monotonic()<until
   time.sleep(.02)
  def call(op,**fields):
   r=lifecycle.account(runtime,op,**fields);assert r['code']=='OK',r['code'];return r['data']
  call('access.register',name='technical',password='isolated process password')
  login=call('access.login',name='technical',password='isolated process password')
  scope={'session':login['session'],'profile_id':login['profile_id'],'activation_id':login['activation_id']}
  created=call('company.create',**scope,name='Test company');selected=call('company.select',**scope,name=created['companies'][0]['id'])
  scope.update(profile_id=selected['profile_id'],activation_id=selected['activation_id'],expected_profile=selected['profile_id'],expected_activation=selected['activation_id'])
  view=call('inventory.list',**scope,equipment={})['equipment'];assert not view['vault_available']
  response=lifecycle.account(runtime,'credential.put',**scope,equipment={'id':str(uuid.uuid4()),'operation_id':str(uuid.uuid4()),'alias':'Fixture','user':'reader','secret':'fixture only'})
  assert response['code']=='VAULT_UNAVAILABLE','production used the DEV key'
  call('inventory.create',**scope,equipment={'id':str(uuid.uuid4()),'operation_id':str(uuid.uuid4()),'name':'No secret inventory','address':'127.0.0.1','port':22,'transport':'ssh'})
  assert len(call('inventory.list',**scope,equipment={})['equipment']['devices'])==1
  assert key.read_bytes()==bytes([51])*32
 finally:lifecycle.stop(daemon)
print('Production daemon ignores DEV key; vault unavailable, inventory usable, no key generated: PASS')
