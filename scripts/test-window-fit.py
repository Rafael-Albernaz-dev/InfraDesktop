#!/usr/bin/env python3
"""Real optimized product opening on owned displays; no resize command in this test."""
from pathlib import Path
import importlib.util,json,os,subprocess,sys,tempfile,time,hashlib
ROOT=Path(__file__).resolve().parents[1]
if len(sys.argv)==1:
 out=ROOT/'build/bf04-window-fit'/time.strftime('%Y%m%dT%H%M%S');out.mkdir(parents=True)
 for w,h in [(1440,900),(900,1440),(768,1024),(1366,768),(1280,720),(1100,700),(1440,860),(720,600)]:
  subprocess.run(['xvfb-run','-a','-s',f'-screen 0 {w}x{h}x24 -nolisten tcp',sys.executable,__file__,str(w),str(h),str(out)],check=True)
 rows=[json.loads(p.read_text()) for p in sorted(out.glob('*.json'))];assert len(rows)==8
 (out/'manifest.json').write_text(json.dumps({'PASS':True,'display':'owned Xvfb; no WM decorations/docks; DPR1 scale1','human_workarea':'NOT RUN; Wails exposes logical monitor geometry, reserves32px horizontal/100px vertical where minimum allows','rows':rows},indent=2)+'\n')
 (ROOT/'build/bf04-window-fit/latest.txt').write_text(str(out)+'\n');print('8 real initial windows fit their owned logical displays: '+str(out));raise SystemExit
w,h=map(int,sys.argv[1:3]);out=Path(sys.argv[3]);os.environ['GDK_SCALE']='1';os.environ['GDK_DPI_SCALE']='1'
spec=importlib.util.spec_from_file_location('native',ROOT/'scripts/test-desktop.py');native=importlib.util.module_from_spec(spec);spec.loader.exec_module(native)
with tempfile.TemporaryDirectory(prefix='id-bf04-fit-') as tmp:
 base=Path(tmp);runtime=base/'runtime';client=base/'client';runtime.mkdir(mode=0o700);client.mkdir(mode=0o700)
 daemon=native.lifecycle.start(runtime,base/'state');ui=None
 try:
  ui=subprocess.Popen([str(ROOT/'build/bin/infradesktop')],env=dict(os.environ,INFRADESKTOP_RUNTIME=str(runtime),INFRADESKTOP_CLIENT_STATE=str(client),NO_AT_BRIDGE='0',GTK_MODULES='atk-bridge',GDK_BACKEND='x11'),stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
  app=native.wait_for('owned initial product',lambda:native.owned_app(ui));native.wait_for('initial service ready',lambda:'Serviço ativo'in native.contents(app))
  target=out/(str(w)+'x'+str(h)+'.png');expected=[min(1440,max(720,w-32)),min(860,max(600,h-100))]
  geometry=native.wait_for('actual initial client fits monitor',lambda:(g if (g:=native.capture(ui,target,preserve_focus=True))['size']==expected else None))
  assert 720<=geometry['size'][0]<=w and 600<=geometry['size'][1]<=h
  (out/(str(w)+'x'+str(h)+'.json')).write_text(json.dumps({'monitor':[w,h],'client':geometry['size'],'decoration':0,'DPR':1,'scale':1,'capture':str(target),'ui_sha256':hashlib.sha256((ROOT/'build/bin/infradesktop').read_bytes()).hexdigest()},indent=2)+'\n')
  native.close_window(ui,app)
 finally:native.terminate_ui(ui);native.lifecycle.stop(daemon)
