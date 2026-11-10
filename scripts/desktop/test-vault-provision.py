#!/usr/bin/env python3
import importlib.machinery
import importlib.util
from pathlib import Path
import subprocess
import tempfile
loader=importlib.machinery.SourceFileLoader('provision',str(Path('packaging/linux/provision-vault')))
spec=importlib.util.spec_from_loader(loader.name,loader);module=importlib.util.module_from_spec(spec);loader.exec_module(module)
with tempfile.TemporaryDirectory(prefix='id-vault-helper-') as tmp:
 root=Path(tmp);calls=[];material=[]
 def run(argv,**kwargs):
  calls.append(argv)
  assert not any('secret' in x for x in argv)
  if argv[1]=='encrypt':
   assert len(kwargs['input'])==32 and '--with-key=host' in argv
   material.append(kwargs['input'])
   return subprocess.CompletedProcess(argv,0,b'isolated-encrypted-fixture')
  return subprocess.CompletedProcess(argv,0,material[0])
 target=module.provision(root,'technician',run);before=target.read_bytes()
 module.provision(root,'technician',run)
 assert len(material)==1 and target.read_bytes()==before and target.stat().st_mode&0o777==0o600
 config=root/'etc/systemd/system/infradesktopd@technician.service.d/vault.conf'
 assert 'LoadCredentialEncrypted=' in config.read_text()
 config.unlink();module.provision(root,'technician',run);assert len(material)==1 # crash reconciliation
 config.write_text('conflicting configuration')
 try:module.provision(root,'technician',run)
 except RuntimeError:pass
 else:raise AssertionError('configuration overwritten')
 assert config.read_text()=='conflicting configuration' and target.read_bytes()==before
 config.unlink();target.unlink();target.symlink_to('elsewhere')
 try:module.provision(root,'technician',run)
 except RuntimeError:pass
 else:raise AssertionError('symlink accepted')
 assert all(argv[0]=='systemd-creds' for argv in calls)
print('Provision helper: idempotency, crash reconciliation, private files and no service operations PASS (isolated runner; real systemd PENDING)')
