#!/usr/bin/env python3
"""Real production Wails bindings and SQLite, explicit test vault and loopback SSH."""
from pathlib import Path
import hashlib,importlib.util,json,os,secrets,sqlite3,subprocess,tempfile,time
spec=importlib.util.spec_from_file_location('native',Path(__file__).with_name('test-desktop.py'));native=importlib.util.module_from_spec(spec);spec.loader.exec_module(native)
ROOT=native.lifecycle.ROOT
OUT=ROOT/'docs/evidence/l04-l05-parte1/native'
OUT.mkdir(parents=True,exist_ok=True)

def main():
 os.umask(0o077)
 root=Path(tempfile.mkdtemp(prefix='id-l04-l05-part1-'))
 for directory in ('runtime','state','client'): (root/directory).mkdir(mode=0o700)
 key=root/'test-master';key.write_bytes(secrets.token_bytes(32));key.chmod(0o600)
 peer=subprocess.Popen([str(ROOT/'build/bin/part1-ssh-fixture'),'--family','v7'],stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
 daemon=None;ui=None;report=[];capture_safe=False;query_visible=False
 try:
  fixture=json.loads(peer.stdout.readline());assert fixture['scope']=='isolated loopback fixture'
  argv=[str(ROOT/'build/bin/infradesktopd-part1-test'),'--runtime',str(root/'runtime'),'--state',str(root/'state')]
  daemon=subprocess.Popen(argv,env=dict(os.environ,INFRADESKTOP_DEV_VAULT_KEY=str(key)),stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
  native.wait_for('owned daemon socket',lambda:(root/'runtime/service.sock').exists())
  ui,app=native.open_ui(root/'runtime',root/'client')
  def text(value):
   def observed():
    app.clear_cache()
    return value in native.contents(app)
   return native.wait_for(value,observed)
  def click(value):native.click(app,value)
  def field(name,value):native.type_field(ui,app,name,value)
  def select(name,option):
   assert name=='Estado dos equipamentos' and option in ('Ativos','Inativos')
   node=native.wait_for('combobox '+name,lambda:native.find(app,'combo box',name))
   # Collapsed WebKit select does not expose GTK menu items consistently.
   # Exercise the actual focused control by physical X11 keys, then assert its result below.
   native.type_label_x11(ui,node,'',key_symbol=0xff50)
   if option=='Inativos':native.type_label_x11(ui,node,'',key_symbol=0xff54)
   native.type_label_x11(ui,node,'',key_symbol=0xff0d)
  def capture(name):
   previous=[None];measured=[None]
   def stable():
    measured[0]=native.capture(ui,OUT/name,preserve_focus=True)
    current=hashlib.sha256((OUT/name).read_bytes()).hexdigest();done=current==previous[0];previous[0]=current;return done
   native.wait_for('two stable painted frames '+name,stable)
   value=measured[0];value['sha256']=previous[0];report.append(value)
  click('Criar conta');field('Usuário','part1-technical');field('Senha','isolated native password');click('Criar conta local');text('Guardei meu código');click('Guardei meu código');text('Entrar');field('Senha','isolated native password');click('Entrar');text('Empresas');capture_safe=True
  click('Nova empresa');field('Nome da empresa','Native Part One');click('Criar empresa');text('Native Part One')
  row=native.wait_for('company row',lambda:next((n for n in native.nodes(app) if n.get_name()=='Selecionar Native Part One'),None));native.type_label_x11(ui,row,'',activate=True);click('Abrir empresa');text('Inventário disponível nesta empresa')
  click('Equipamentos');text('Nenhum equipamento cadastrado');capture('empty-light.png')
  assert not native.find(app,'entry','Buscar equipamento')
  assert len([n for n in native.nodes(app) if n.get_role_name()=='push button' and n.get_name()=='Adicionar equipamento'])==1
  click('Adicionar equipamento');field('Nome','Loopback Router');field('IP de gerenciamento','127.0.0.1')
  port=native.wait_for('SSH port',lambda:native.find(app,'spin button','Porta SSH') or native.find(app,'entry','Porta SSH'))
  native.type_label_x11(ui,port,str(fixture['port']));field('Usuário','reader');capture('device-form-light.png')
  capture_safe=False
  field('Senha','local-fixture-only');click('Salvar equipamento')
  native.wait_for('direct enrollment dialog closed',lambda:native.find(app,'dialog','Adicionar equipamento') is None)
  capture_safe=True;text('Loopback Router')
  with sqlite3.connect(root/'state/infradesktop.db') as db:company=db.execute('select profile from account').fetchone()[0]
  company_db=root/f'state/companies/{company}.db'
  def device():
   with sqlite3.connect(company_db) as db:return db.execute('select id,credential_id,revision,active,fingerprint,observation from devices').fetchone()
  native.wait_for('durable device with credential',lambda:device() and device()[1])
  click('Verificar identidade');text('Conferir identidade do destino');text(fixture['fingerprint']);capture('trust-light.png')
  assert peer.poll() is None
  field('Origem da conferência','Local controlled fixture');click('Confirmar chave conferida');native.wait_for('persisted exact host key',lambda:device()[4]==fixture['fingerprint'])
  click('Consultar');native.wait_for('persisted SSH observation',lambda:device()[5] is not None)
  text('Fixture router');query_visible=True;click('Ver resultado')
  region=native.wait_for('result region',lambda:next((n for n in native.nodes(app) if n.get_name()=='Resultado da consulta'),None))
  assert native.Atspi.Component.scroll_to(region,native.Atspi.ScrollType.TOP_EDGE)
  text('ether1');capture('result-light.png')
  click('Conta: part1-technical');click('Escuro');text('Loopback Router');capture('result-dark.png')
  click('Editar equipamento');text('Salvar equipamento');capture('device-form-dark.png');click('Fechar painel')
  click('Desativar');native.wait_for('inactive device committed',lambda:device()[3]==0);text('Nenhum equipamento ativo')
  select('Estado dos equipamentos','Inativos');text('Reativar');capture('inactive-dark.png');click('Reativar');native.wait_for('reactivated',lambda:device()[3]==1)
  # Narrow keyboard: real X11 Tab, Shift+Tab, Escape, selection/settings return.
  native.configure_window(ui,720,600);click('Navegação');picker=native.wait_for('focused navigation',lambda:native.find(app,'push button','Selecionar empresa'));native.wait_for('focus moved into sidebar',lambda:picker.get_state_set().contains(native.Atspi.StateType.FOCUSED))
  native.type_label_x11(ui,picker,'',key_symbol=0xff09);native.type_label_x11(ui,picker,'',key_symbol=0xff09,shift=True)
  native.wait_for('Shift Tab returns picker',lambda:picker.get_state_set().contains(native.Atspi.StateType.FOCUSED));native.type_label_x11(ui,picker,'',key_symbol=0xff1b)
  trigger=native.find(app,'push button','Navegação');native.wait_for('Escape returns trigger',lambda:trigger.get_state_set().contains(native.Atspi.StateType.FOCUSED))
  click('Navegação');click('Equipamentos');native.wait_for('selection returns trigger',lambda:trigger.get_state_set().contains(native.Atspi.StateType.FOCUSED));capture('narrow-dark.png');assert not native.find(app,'dialog','Adicionar equipamento');assert trigger.get_state_set().contains(native.Atspi.StateType.FOCUSED)
  # New direct form at the real minimum size, both themes, no password typed/captured.
  for theme,label in [('dark',None),('light','Claro')]:
   if label:click('Conta: part1-technical');click(label)
   opener=native.wait_for('header Add',lambda:native.find(app,'push button','Adicionar'))
   native.type_label_x11(ui,opener,'',activate=True)
   panel=native.wait_for('direct minimum dialog',lambda:native.find(app,'dialog','Adicionar equipamento'))
   close=native.wait_for('X close',lambda:native.find(app,'push button','Fechar painel'))
   native.wait_for('X initial focus',lambda:close.get_state_set().contains(native.Atspi.StateType.FOCUSED))
   # Shift+Tab wraps to the actual last form control and scrolls it into view.
   native.type_label_x11(ui,close,'',key_symbol=0xff09,shift=True)
   save=native.wait_for('focused Save at scroll end',lambda:native.find(app,'push button','Salvar equipamento'))
   native.wait_for('Save focused by Shift Tab',lambda:save.get_state_set().contains(native.Atspi.StateType.FOCUSED))
   def extents(node):
    r=native.Atspi.Component.get_extents(node,native.Atspi.CoordType.WINDOW)
    return {'x':r.x,'y':r.y,'width':r.width,'height':r.height}
   save_bounds=native.wait_for('Save within native viewport after scroll',lambda:(lambda b:b if b['x']>=0 and b['y']>=0 and b['x']+b['width']<=720 and b['y']+b['height']<=600 else None)(extents(save)))
   capture('direct-'+theme+'-720-scroll-save.png')
   native.type_label_x11(ui,save,'',key_symbol=0xff09)
   native.wait_for('Tab wraps to X',lambda:close.get_state_set().contains(native.Atspi.StateType.FOCUSED))
   close_bounds=extents(close);assert close_bounds['width']>=32 and close_bounds['height']>=32
   capture('direct-'+theme+'-720-top.png')
   native.type_label_x11(ui,close,'',key_symbol=0xff1b)
   native.wait_for('Escape closed direct dialog',lambda:native.find(app,'dialog','Adicionar equipamento') is None)
   native.wait_for('Escape returns exact header opener',lambda:opener.get_state_set().contains(native.Atspi.StateType.FOCUSED))
   # Actual pointer activation, using only this owned private-display window.
   native.type_label_x11(ui,opener,'',activate=True)
   xbutton=native.wait_for('reopened X button',lambda:native.find(app,'push button','Fechar painel'))
   native.wait_for('X focused after reopening',lambda:xbutton.get_state_set().contains(native.Atspi.StateType.FOCUSED))
   screen=native.Atspi.Component.get_extents(xbutton,native.Atspi.CoordType.SCREEN)
   x=native.C.CDLL('libX11.so.6');xt=native.C.CDLL('libXtst.so.6')
   x.XOpenDisplay.argtypes=[native.C.c_char_p];x.XOpenDisplay.restype=native.C.c_void_p;x.XCloseDisplay.argtypes=[native.C.c_void_p];x.XSync.argtypes=[native.C.c_void_p,native.C.c_int]
   xt.XTestFakeMotionEvent.argtypes=[native.C.c_void_p,native.C.c_int,native.C.c_int,native.C.c_int,native.C.c_ulong];xt.XTestFakeButtonEvent.argtypes=[native.C.c_void_p,native.C.c_uint,native.C.c_int,native.C.c_ulong]
   display=x.XOpenDisplay(None);assert display and app.get_process_id()==ui.pid
   try:
    assert xt.XTestFakeMotionEvent(display,-1,screen.x+screen.width//2,screen.y+screen.height//2,0)
    assert xt.XTestFakeButtonEvent(display,1,True,0);assert xt.XTestFakeButtonEvent(display,1,False,0);x.XSync(display,False)
   finally:x.XCloseDisplay(display)
   native.wait_for('physical pointer closes exact dialog',lambda:native.find(app,'dialog','Adicionar equipamento') is None)
   report.append({'step':'minimum direct form '+theme,'runtime':'Wails WebKit AT-SPI + physical X11 keyboard','viewport':[720,600],'save_after_scroll':save_bounds,'close_at_top':close_bounds,'keyboard':'ShiftTab X->Save; Tab Save->X; Escape->exact header opener','mouse':'XTest click at measured X center in owned private display','password_typed':False,'PASS':True})
  report.append({'step':'account, company, sealed credential, durable device, trust, fixed SSH read, result, inactive/reactivate, both themes, narrow keyboard','PASS':True})
  native.close_window(ui,app);ui=None
  native.lifecycle.stop(daemon);daemon=None
  # Closed coherent test set remains private for reproduction; no human state.
  assert not (root/'runtime/service.sock').exists()
 except BaseException:
  if ui is not None and ui.poll() is None:
   if capture_safe:
    native.capture(ui,OUT/'failure.png',preserve_focus=True)
   print('Control roles/names only:',[(node.get_role_name(),node.get_name()) for node in native.nodes(app) if node.get_role_name() in ('combo box','spin button','spinbutton')],flush=True)
  raise
 finally:
  native.terminate_ui(ui)
  if daemon is not None:native.lifecycle.stop(daemon)
  peer.terminate();peer.wait(timeout=5)
  (OUT/'journey.json').write_text(json.dumps({'root':str(root),'scope':'isolated test key/provider/loopback peer; normal product not seeded','daemon_argv':locals().get('argv'),'binaries':{name:hashlib.sha256((ROOT/'build/bin'/name).read_bytes()).hexdigest() for name in ('infradesktop','infradesktopd','infradesktopd-part1-test','part1-ssh-fixture')},'fixture_pid':peer.pid,'fixture_exit':peer.returncode,'steps':report,'human_acceptance':'NOT RUN','cleanup':'owned processes only; private test state and test key retained together'},indent=2)+'\n')
 print('L04/L05 real Wails journey PASS; private test state '+str(root))
if __name__=='__main__':main()
