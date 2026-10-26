"""Only the owned Wails/SQLite/SSH journey invokes this helper; no browser bridge."""
import json,sqlite3,stat,re
import native_physical as physical

def exercise(native,ui,app,root,company_db,device_id,click,field,text,node,exact,capture,step,reopen_owned_window):
 def current():
  with sqlite3.connect(company_db) as db:
   row=db.execute('select observation_id from pppoe_current where device_id=?',(device_id,)).fetchone()
   return row[0] if row else None
 def refs():
  with sqlite3.connect(company_db) as db:return db.execute('select id,observation_id from pppoe_references where device_id=?',(device_id,)).fetchall()
 click('Abrir sessões PPPoE');text('Nenhuma leitura de sessões disponível.');assert current() is None
 capture('l06-before-collection');click('Atualizar sessões');first=native.wait_for('first PPPoE committed',current);text('32 sessões');text('synthetic-0000');capture('l06-current')
 search=native.wait_for('PPPoE search',lambda:node('Buscar sessões',('entry',)))
 native.Atspi.Component.scroll_to(search,native.Atspi.ScrollType.ANYWHERE)
 physical.pointer(native,ui,app,search)
 physical.focused(native,search,'physical click focuses PPPoE search')
 physical.key(native,ui,search,0xff09)
 order=native.wait_for('PPPoE ordering',lambda:next((n for n in native.nodes(app) if n.get_role_name()=='combo box' and n.get_name().startswith('Ordenar ')),None))
 physical.focused(native,order,'Tab reaches PPPoE ordering')
 physical.key(native,ui,order,0xff09,shift=True)
 physical.focused(native,search,'ShiftTab returns exact PPPoE search')
 # Native selection action changes real ordering; observe selected text/state.
 physical.key(native,ui,search,0xff09)
 physical.focused(native,order,'ordering setup remains reached by Tab')
 physical.key(native,ui,order,0xff54)
 native.wait_for('IP selected in ordering',lambda:order.get_name()=='Ordenar IP')
 duplicate=native.wait_for('PPPoE duplicate filter',lambda:node('Somente logins duplicados',('check box',)))
 native.Atspi.Component.scroll_to(duplicate,native.Atspi.ScrollType.ANYWHERE)
 physical.pointer(native,ui,app,duplicate)
 native.wait_for('duplicate checkbox checked',lambda:duplicate.get_state_set().contains(native.Atspi.StateType.CHECKED))
 pager=native.wait_for('duplicate results pager',lambda:node('Próxima página'))
 native.Atspi.Component.scroll_to(pager,native.Atspi.ScrollType.BOTTOM_EDGE)
 text('32 resultados') # Assert after bringing this footer into the native viewport.
 capture('l06-duplicates-result')
 click('Limpar filtros');physical.focused(native,search,'clear filters returns search focus');text('synthetic-0000')
 # Wheel input acts on the new surface, not AT-SPI scroll_to as its oracle.
 before=native.Atspi.Component.get_extents(search,native.Atspi.CoordType.WINDOW).y
 physical.pointer(native,ui,app,search,button=5,count=24)
 native.wait_for('physical PPPoE wheel changes content position',lambda:native.Atspi.Component.get_extents(search,native.Atspi.CoordType.WINDOW).y < before)
 capture('l06-physical-wheel')
 step('PPPoE new controls physical focus/order/duplicates/wheel',Tab=True,ShiftTab=True,ordering='address',duplicates_recovered=True)
 details=native.wait_for('stable row detail',lambda:next((n for n in native.nodes(app) if n.get_role_name()=='push button' and n.get_name().startswith('Detalhar synthetic-0000 ')),None));detail_name=details.get_name();click(detail_name);text('Identidade local da linha:');heading=native.wait_for('detail focus',lambda:next((n for n in native.nodes(app) if n.get_role_name()=='heading' and n.get_name()=='synthetic-0000' and n.get_state_set().contains(native.Atspi.StateType.FOCUSED)),None));capture('l06-detail');click('Fechar detalhe');native.wait_for('return to row focus',lambda:node(detail_name).get_state_set().contains(native.Atspi.StateType.FOCUSED))
 field('Buscar sessões','missing-result');text('Nenhuma sessão corresponde aos filtros.');capture('l06-search-empty');click('Limpar filtros');text('synthetic-0000');click('Próxima página');text('synthetic-0008');capture('l06-page2');click('Início da lista')
 click('Salvar referência desta leitura');native.wait_for('explicit reference',lambda:len(refs())==1);assert refs()[0][1]==first
 click('Atualizar sessões');second=native.wait_for('new current immutable ID',lambda:(x if (x:=current())!=first else None));text('32 sessões');click('Comparação')
 combo=native.wait_for('reference selector',lambda:next((n for n in native.nodes(app) if n.get_role_name()=='combo box' and n.get_name().startswith('Referência para comparar ')),None))
 assert native.Atspi.Component.grab_focus(combo) # Explicit setup, not a focus-transition oracle.
 physical.key(native,ui,combo,0xff54);text('Detalhar par');capture('l06-comparison')
 pair=native.wait_for('pair action',lambda:next((n for n in native.nodes(app) if n.get_role_name()=='push button' and n.get_name()=='Detalhar par'),None))
 pair_label=native.contents(pair.get_parent())
 identities=re.search(r'([0-9a-f]{8}) → ([0-9a-f]{8})',pair_label)
 assert identities,'selected pair must expose both immutable row prefixes'
 expected=[]
 with sqlite3.connect(company_db) as db:
  for observation,prefix in zip((first,second),identities.groups()):
   found=db.execute('select row_json from pppoe_rows where observation_id=? and row_id like ?',(observation,prefix+'%')).fetchall()
   assert len(found)==1,'row prefix must identify exactly one row in its observation'
   expected.append(json.loads(found[0][0]))
 native.Atspi.Component.scroll_to(pair,native.Atspi.ScrollType.ANYWHERE);assert pair.get_action_iface().do_action(0);text('Sessões do par')
 for side,row in zip(('Referência','Atual'),expected):
  heading=native.wait_for('exact pair side '+side,lambda:next((n for n in native.nodes(app) if n.get_role_name()=='heading' and n.get_name()==side),None))
  content=native.contents(heading.get_parent())
  for value in ('Login: '+row['login'],'Linha: '+row['row_id'],'IP: '+row['address']):assert value in content,(side,value,content)
 capture('l06-pair');step('selected pair matches exact immutable SQLite inputs',base_row=expected[0]['row_id'],target_row=expected[1]['row_id']);click('Fechar par')
 # B uses the same production UI, IPC, service, SQLite and pinned loopback SSH.
 def diagnostic(scope):
  with sqlite3.connect(company_db) as db:
   record=db.execute('select o.meta_json from pppoe_diagnostic_current c join pppoe_diagnostic_observations o on o.id=c.diagnostic_id where c.device_id=? and c.scope=?',(device_id,scope)).fetchone()
   return json.loads(record[0]) if record else None
 for tab,scope,action,expected in [('VLANs e serviços','catalog','Atualizar serviços e VLANs','fixture-service-a'),('RADIUS','radius','Atualizar RADIUS','198.51.100.20')]:
  click(tab);text('Nenhuma leitura deste diagnóstico disponível.');capture('l06-'+scope+'-uncollected')
  trigger=native.wait_for('physical '+action,lambda:node(action));native.Atspi.Component.scroll_to(trigger,native.Atspi.ScrollType.ANYWHERE)
  physical.pointer(native,ui,app,trigger)
  saved=native.wait_for('persisted '+scope,lambda:diagnostic(scope));text(expected)
  assert saved['count']==(7 if scope=='catalog' else 2)
  native.Atspi.Component.scroll_to(exact(expected),native.Atspi.ScrollType.ANYWHERE)
  capture('l06-'+scope+'-ready')
  native.configure_window(ui,1440,860);capture('l06-'+scope+'-desktop',viewport=(1440,860));native.configure_window(ui,720,600)
  step('real '+scope+' UI/IPC/SQLite/pinned SSH',diagnostic_id=saved['diagnostic_id'],count=saved['count'],physical_refresh=True)
 click('Sessões');text('synthetic-0000')
 selected=native.wait_for('session ping detail',lambda:next((n for n in native.nodes(app) if n.get_role_name()=='push button' and n.get_name().startswith('Detalhar synthetic-0000 ')),None));selected_name=selected.get_name();click(selected_name);text('Ping pelo concentrador');capture('l06-ping-uncollected')
 trigger=native.wait_for('physical ping',lambda:node('Sondar IP pelo concentrador'));native.Atspi.Component.scroll_to(trigger,native.Atspi.ScrollType.ANYWHERE);physical.pointer(native,ui,app,trigger)
 saved_ping=native.wait_for('persisted ping',lambda:diagnostic('ping'));text('Enviados: 3 · Recebidos: 3 · Perda: 0%');assert saved_ping['source_observation']==second
 native.Atspi.Component.scroll_to(exact('Ping pelo concentrador'),native.Atspi.ScrollType.TOP_EDGE);capture('l06-ping-ready')
 native.configure_window(ui,1440,860);capture('l06-ping-desktop',viewport=(1440,860));native.configure_window(ui,720,600)
 close=node('Fechar detalhe');native.Atspi.Component.scroll_to(close,native.Atspi.ScrollType.ANYWHERE);physical.pointer(native,ui,app,close)
 native.wait_for('close returns exact ping session trigger',lambda:node(selected_name).get_state_set().contains(native.Atspi.StateType.FOCUSED))
 click(selected_name)
 focused=native.wait_for('reopened detail heading focused',lambda:next((n for n in native.nodes(app) if n.get_role_name()=='heading' and n.get_name()=='synthetic-0000' and n.get_state_set().contains(native.Atspi.StateType.FOCUSED)),None))
 physical.key(native,ui,focused,0xff1b)
 native.wait_for('Escape returns exact session trigger',lambda:node(selected_name).get_state_set().contains(native.Atspi.StateType.FOCUSED))
 step('real ping UI/IPC/SSH finite result and physical close/Escape',diagnostic_id=saved_ping['diagnostic_id'],source_observation=second,source_row=saved_ping['source_row'],probes=3)
 click('Comparação');text('Detalhar par')
 click('Exportar JSON sanitizado')
 cancel=native.wait_for('native picker cancel',lambda:next((n for n in native.nodes(app) if n.get_role_name()=='push button' and n.get_name() in ('Cancel','Cancelar')),None))
 assert cancel.get_action_iface().do_action(0)
 export_button=native.wait_for('export trigger after cancellation',lambda:node('Exportar JSON sanitizado'))
 physical.focused(native,export_button,'native picker cancel restores exact export trigger')
 capture('l06-export-cancel')
 click('Exportar JSON sanitizado')
 # GTK chooser belongs to this isolated process on this Xvfb. Use its native
 # EditableText value so special filename characters are not guessed as keycodes.
 picker=native.wait_for('owned native file chooser',lambda:next((n for n in native.nodes(app) if n.get_name()=='Exportar comparação PPPoE sanitizada' and n.get_role_name() in ('file chooser','dialog','frame')),None))
 input_metadata=[]
 candidates=[]
 for control in native.nodes(picker):
  interfaces=list(control.get_interfaces())
  if control.get_role_name() not in ('entry','text') and not any(i.rsplit('.',1)[-1]=='EditableText' for i in interfaces):continue
  labels=[]
  for relation in control.get_relation_set():
   if relation.get_relation_type()==native.Atspi.RelationType.LABELLED_BY:
    labels.extend(relation.get_target(i).get_name() for i in range(relation.get_n_targets()))
  text_iface=control.get_text_iface()
  value=native.Atspi.Text.get_text(control,0,-1) if 'Text' in interfaces else ''
  is_default=value=='pppoe-sanitizado.json'
  input_metadata.append({'role':control.get_role_name(),'name':control.get_name() if control.get_name() in ('Name','Name:','Nome','Nome:','pppoe-sanitizado.json','') else '[other synthetic input label]','interfaces':interfaces,'labelled_by':labels,'default_filename':is_default})
  if is_default and control.get_editable_text_iface():candidates.append(control)
 step('native picker controlled input metadata',inputs=input_metadata)
 assert len(candidates)==1,'exact default filename must identify one editable native picker input'
 entry=candidates[0]
 destination=root/'pppoe-export.json'
 # Focus is deliberately placed for location setup, separate from focus assertions.
 assert native.Atspi.Component.grab_focus(entry)
 physical.focused(native,entry,'picker Name setup focus')
 # Verify this GTK's absolute Name behavior through actual file
 # creation at this exact owned destination, or fail (no binding/JS fallback).
 assert native.Atspi.EditableText.set_text_contents(entry,str(destination))
 save=native.wait_for('owned picker save',lambda:next((n for n in native.nodes(app) if n.get_role_name()=='push button' and n.get_name() in ('Save','Salvar')),None));assert save.get_action_iface().do_action(0)
 native.wait_for('sanitized export file',destination.is_file);text('Arquivo sanitizado salvo.');raw=destination.read_bytes();export=json.loads(raw);assert export['profile']=='pppoe-sanitized-v1' and len(export['observations'])==2 and export['deltas'] and len(export['diagnostic_rows'])==10 and all(d['status']=='available' for d in export['diagnostics'])
 assert stat.S_IMODE(destination.stat().st_mode)==0o600
 for forbidden in (b'synthetic-',b'198.51.',b'02:00:',device_id.encode(),first.encode(),second.encode(),b'export_key',b'local-fixture-only'):assert forbidden not in raw
 ui,app=reopen_owned_window();text('32 sessões');assert current()==second and refs()[0][1]==first;capture('l06-reopen')
 step('L06 real journey current/filter/page/detail/reference/refresh/compare/picker/export/reopen',observation_ids=[first,second],reference_id=refs()[0][0],export_path=str(destination),export_bytes=len(raw),export_mode='0600',no_extra_collect_on_local_actions=True)
 delete_name='Excluir referência '+refs()[0][0][:8]
 click(delete_name);native.wait_for('reference association removed',lambda:not refs())
 heading=native.wait_for('reference heading',lambda:next((n for n in native.nodes(app) if n.get_role_name()=='heading' and n.get_name()=='Referências preservadas'),None))
 physical.focused(native,heading,'removed reference returns to references heading')
 capture('l06-reference-removed')
 for tab,scope,expected in [('VLANs e serviços','catalog','fixture-service-a'),('RADIUS','radius','198.51.100.20')]:
  click(tab);text(expected);assert diagnostic(scope) is not None;capture('l06-'+scope+'-reopen')
 click('Voltar ao equipamento')
