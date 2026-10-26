#!/usr/bin/python3
"""Owned real Wails journey; no browser bridge/seed or human account/state."""
from fixture_io import FixtureOutput
from pathlib import Path
import hashlib,importlib.util,json,os,secrets,sqlite3,subprocess,tempfile,time
ROOT=Path(__file__).resolve().parents[1]
OUT=Path(os.environ.get('INFRADESKTOP_EVIDENCE_DIR',str(ROOT/'build/l06-native'/time.strftime('%Y%m%dT%H%M%S')))).resolve()
OUT.mkdir(parents=True,exist_ok=True)
spec=importlib.util.spec_from_file_location('native',ROOT/'scripts/test-desktop.py');native=importlib.util.module_from_spec(spec);spec.loader.exec_module(native)
native.DESKTOP_OUTPUT=OUT/'runtime-icon'
sha=lambda p:hashlib.sha256(Path(p).read_bytes()).hexdigest()

def main():
 os.umask(0o077);all_runs=[]
 for theme,family in [('light','v6'),('dark','v7')]:
  root=Path(tempfile.mkdtemp(prefix='id-l06-native-'+theme+'-'))
  for directory in ('runtime','state','client'):(root/directory).mkdir(mode=0o700)
  (root/'OWNED-L06-TEST').write_text('Only this synthetic root may be inspected by this harness.\n')
  key=root/'test-master';key.write_bytes(secrets.token_bytes(32));key.chmod(0o600)
  peer=None;daemon=None;ui=None;app=None;safe_capture=False;steps=[];captures=[];passed=False
  record={'root':str(root),'theme':theme,'family':family,'viewport':[720,600],'steps':steps,'captures':captures,'scope':'real optimized Wails; separate explicit-test-provider daemon; isolated Unix/SQLite/cofre/SSH loopback; no firmware/human acceptance','status':'RUNNING'}
  def flush():
   (OUT/(theme+'-journey.json')).write_text(json.dumps(record,indent=2)+'\n')
  def step(name,**data):
   steps.append({'step':name,**data});print(theme+': '+name,flush=True);flush()
  try:
   peer=subprocess.Popen([str(ROOT/'build/bin/part1-ssh-fixture'),'--family',family,'--mode','interfaces','--realtime'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
   record['fixture_pid']=peer.pid
   peer_output=FixtureOutput(peer);fixture=peer_output.json_line('readiness');assert fixture['scope']=='isolated loopback fixture';record['fixture']={k:v for k,v in fixture.items()}
   argv=[str(ROOT/'build/bin/infradesktopd-part1-test'),'--runtime',str(root/'runtime'),'--state',str(root/'state')]
   env=dict(os.environ,INFRADESKTOP_DEV_VAULT_KEY=str(key));env.pop('CREDENTIALS_DIRECTORY',None)
   daemon=subprocess.Popen(argv,env=env,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True);record['daemon_argv']=argv;record['daemon_pid']=daemon.pid
   def daemon_ready():
    if daemon.poll() is not None:
     record['daemon_startup_stderr']=daemon.stderr.read()
     raise AssertionError('owned daemon exited before readiness: '+record['daemon_startup_stderr'])
    return (root/'runtime/service.sock').exists()
   native.wait_for('owned daemon socket',daemon_ready)
   ui,app=native.open_ui(root/'runtime',root/'client',owned_home=root/'home',viewport=(720,600));record['ui_pid']=ui.pid
   def norm(value):return ' '.join(value.split())
   def node(name,roles=('push button','toggle button','page tab')):
    return next((n for n in native.nodes(app) if norm(n.get_name())==norm(name) and n.get_role_name() in roles),None)
   def text(value):
    def observed():app.clear_cache();return norm(value) in norm(native.contents(app))
    return native.wait_for(value,observed)
   def click(name):
    target=native.wait_for('control '+name,lambda:node(name));native.Atspi.Component.scroll_to(target,native.Atspi.ScrollType.ANYWHERE);assert target.get_action_iface().do_action(0)
   def field(name,value):native.type_field(ui,app,name,value)
   def exact(name):
    for n in native.nodes(app):
     if norm(n.get_name())==norm(name):return n
     if 'Text' in n.get_interfaces():
      try:
       if norm(native.Atspi.Text.get_text(n,0,-1))==norm(name):return n
      except native.GLib.Error:pass
    return None
   def bounds(n):
    r=native.Atspi.Component.get_extents(n,native.Atspi.CoordType.WINDOW);return {'x':r.x,'y':r.y,'width':r.width,'height':r.height}
   def capture(name,focus_name=None,viewport=(720,600)):
    assert safe_capture and app.get_process_id()==ui.pid
    target=OUT/(name+'-'+theme+'-'+str(viewport[0])+'.png');previous=[None];current=[None]
    def stable():
     current[0]=native.capture(ui,target,preserve_focus=True);h=sha(target);done=h==previous[0];previous[0]=h;return done
    native.wait_for('stable native paint '+name,stable)
    r={**current[0],'sha256':sha(target),'theme':theme,'viewport':list(viewport),'runtime':'Wails GTK/WebKit X11; measured owned client surface','browser_geometry':'NOT USED'}
    if focus_name:
     focused=native.wait_for('focused '+focus_name,lambda:next((n for n in native.nodes(app) if n.get_name()==focus_name and n.get_state_set().contains(native.Atspi.StateType.FOCUSED)),None));r['native_focus']={'name':focus_name,'role':focused.get_role_name(),'bounds':bounds(focused)}
    captures.append(r);flush()
   account='native-'+theme;password='isolated native password'
   click('Criar conta');field('Usuário',account);field('Senha',password);click('Criar conta local');text('Guardei meu código');click('Guardei meu código');text('Entrar');field('Senha',password);click('Entrar');text('Empresas');safe_capture=True
   def account_row():
    with sqlite3.connect(root/'state/infradesktop.db') as db:return db.execute('select profile,theme from account').fetchone()
   if theme=='dark':
    click('Conta: '+account);click('Escuro');native.wait_for('persisted account theme dark',lambda:account_row()[1]=='dark');click('Conta: '+account)
   assert account_row()[1]==theme
   step('real local account/login/recovery acknowledgement',theme_persisted=theme,one_time_code_captured=False)
   click('Nova empresa');company_name='Native '+theme+' company';field('Nome da empresa',company_name);click('Criar empresa');text(company_name)
   row=native.wait_for('company selection',lambda:exact('Selecionar '+company_name));native.type_label_x11(ui,row,'',activate=True);click('Abrir empresa');text('Nenhum equipamento cadastrado')
   profile=native.wait_for('selected company',lambda:account_row()[0]);company_db=root/f'state/companies/{profile}.db';assert company_db.is_file()
   def navigate(name):click('Navegação');click(name)
   def dashboard_device():
    app.clear_cache()
    expected=''.join((router_name+' · 127.0.0.1').split())
    matches=[n for n in native.nodes(app) if n.get_role_name()=='push button' and ''.join(n.get_name().split())==expected]
    assert len(matches)<=1,'duplicate dashboard device action'
    return matches[0] if matches else None
   navigate('Equipamentos');text('Nenhum equipamento cadastrado');capture('empty')
   click('Adicionar equipamento');router_name='Native '+theme+' router';field('Nome',router_name);field('IP de gerenciamento','127.0.0.1')
   # Secondary SSH port disclosure is exercised through the real WebKit accessible action.
   disclosure=native.wait_for('secondary SSH port disclosure',lambda:exact('Porta SSH · 22'))
   native.Atspi.Component.scroll_to(disclosure,native.Atspi.ScrollType.ANYWHERE)
   assert disclosure.get_action_iface().do_action(0)
   port=native.wait_for('SSH port field',lambda:node('Porta SSH',('spin button','spinbutton','entry')));native.type_label_x11(ui,port,str(fixture['port']))
   field('Usuário','reader');capture('device-form')
   safe_capture=False;field('Senha','local-fixture-only');click('Salvar e conectar');native.wait_for('enrollment dialog gone',lambda:exact('Adicionar equipamento') is None);text('Primeira conexão');safe_capture=True
   def device():
    with sqlite3.connect(company_db) as db:
     rows=db.execute('select id,name,credential_id,revision,active,fingerprint,observation,trust_acceptance,last_attempt from devices').fetchall()
     assert len(rows)==1
     return rows[0]
   saved=native.wait_for('durable enrolled device',lambda:device());device_id=saved[0];assert saved[1]==router_name and saved[2] and saved[5]=='' and saved[6] is None
   assert not exact('Origem da conferência')
   capture('first-acceptance')
   step('save leads to deliberate first acceptance',one_saved_device=True,credential_linked=True,no_pin_before_acceptance=True,no_observation_before_acceptance=True)
   click('Confiar neste equipamento e conectar')
   def reading_after(previous=None):
    d=device()
    if not d[6]:return None
    r=json.loads(d[6]);return r if r['observed_at']!=previous else None
   native.wait_for('initial trusted summary',reading_after)
   def reopen_owned_window():
    nonlocal ui,app
    former=ui.pid
    native.close_window(ui,app);ui=None;app=None
    assert not Path('/proc',str(former)).exists()
    record.setdefault('closed_ui_pids',[]).append(former)
    ui,app=native.open_ui(root/'runtime',root/'client',owned_home=root/'home',viewport=(720,600));record['ui_pid']=ui.pid
    restored=native.wait_for('confirmed login or restored account',lambda:node('Entrar') or node('Conta: '+account))
    if restored.get_name()=='Entrar':
     field('Usuário',account);field('Senha',password);click('Entrar')
     native.wait_for('same synthetic account after login',lambda:node('Conta: '+account))
    navigate('Dashboard')
    link=native.wait_for('same device after process reopen',dashboard_device)
    native.Atspi.Component.scroll_to(link,native.Atspi.ScrollType.ANYWHERE)
    assert link.get_action_iface().do_action(0)
    text('Resumo');click('Abrir sessões PPPoE')
    step('real Wails process reopened with same runtime/client state',former_pid=former,new_pid=ui.pid)
    return ui,app
   from l06_native import exercise
   exercise(native,ui,app,root,company_db,device_id,click,field,text,node,exact,capture,step,reopen_owned_window)
   native.close_window(ui,app);ui=None
   native.lifecycle.stop(daemon);daemon=None;assert not (root/'runtime/service.sock').exists()
   stats=peer_output.finish();assert stats['authentications']==6 and stats['commands']==10 and stats['ping_requests']==1,stats
   record['fixture_final_counters']=stats;record['status']='PASS';passed=True
   step('owned runtime cleanup',ui_closed=True,daemon_stopped=True,socket_absent=True,fixture_exit=peer.returncode,authentications=stats['authentications'])
  except BaseException:
   record['status']='FAIL'
   if ui and ui.poll() is None and safe_capture:
    picker_open=any(n.get_name()=='Exportar comparação PPPoE sanitizada' and n.get_role_name() in ('file chooser','dialog','frame') for n in native.nodes(app))
    if picker_open:
     record['failure_owned_accessibility']='Native picker active; full tree omitted to avoid incidental host places. Controlled input metadata is recorded separately.'
    else:
     record['failure_owned_accessibility']=native.contents(app)
     record['failure_owned_controls']=[{'name':n.get_name(),'role':n.get_role_name()} for n in native.nodes(app) if n.get_role_name() in ('combo box','check box','push button')]
   if ui and ui.poll() is None and safe_capture:
    try:native.capture(ui,OUT/('failure-'+theme+'.png'),preserve_focus=True)
    except AssertionError as capture_error:record['failure_capture_error']=str(capture_error) or 'multiple owned windows during native picker'
    picker_cancel=node('Cancel') or node('Cancelar')
    if picker_cancel:picker_cancel.get_action_iface().do_action(0)
   raise
  finally:
   cleanup_errors=[]
   for name,process in [('ui',ui),('daemon',daemon),('fixture',peer)]:
    if process is None:continue
    try:
     if name=='ui':native.terminate_ui(process)
     elif name=='daemon':native.lifecycle.stop(process)
     elif process.poll() is None:process.terminate();process.wait(timeout=5)
    except BaseException as error:
     cleanup_errors.append(name+': '+type(error).__name__+': '+str(error)[-3000:])
     if process.poll() is None:
      try:process.kill();process.wait(timeout=5)
      except BaseException as forced:cleanup_errors.append(name+' forced: '+type(forced).__name__)
   record['cleanup_errors']=cleanup_errors
   record['processes_alive_after_cleanup']={str(pid):Path('/proc',str(pid)).exists() for pid in [record.get('ui_pid'),record.get('daemon_pid'),record.get('fixture_pid')]+record.get('closed_ui_pids',[]) if pid}
   record['socket_present_after_cleanup']=(root/'runtime/service.sock').exists()
   if cleanup_errors or any(record['processes_alive_after_cleanup'].values()) or record['socket_present_after_cleanup']:
    record['status']='FAIL';passed=False
   record['binaries']={name:sha(ROOT/'build/bin'/name) for name in ['infradesktop','infradesktopd','infradesktopd-part1-test','part1-ssh-fixture']}
   flush();all_runs.append(record)
   (OUT/'journey.json').write_text(json.dumps({'runs':all_runs,'status':'PASS' if len(all_runs)==2 and all(r['status']=='PASS' for r in all_runs) else 'INCOMPLETE','human_acceptance':'NOT RUN','firmware_homologation':'NOT RUN'},indent=2)+'\n')
   assert not cleanup_errors and not any(record['processes_alive_after_cleanup'].values()) and not record['socket_present_after_cleanup'],'owned process cleanup failed'
  assert passed
 print('Real Wails operational journey PASS: light/dark720, SSH loopback, immutable human state; no firmware acceptance.')
if __name__=='__main__':main()
