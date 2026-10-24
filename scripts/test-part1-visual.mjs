// Focal UI contract: real components, isolated metadata only, no product seed.
import {chromium} from '../frontend/node_modules/@playwright/test/index.mjs';
import {build,preview} from '../frontend/node_modules/vite/dist/node/index.js';
import react from '../frontend/node_modules/@vitejs/plugin-react/dist/index.js';
import fs from 'node:fs';import path from 'node:path';import crypto from 'node:crypto';import {pathToFileURL} from 'node:url';
import {inspectActions} from './visual-actions.mjs';
const focal=process.argv.includes('--navigation-only');
const out=path.resolve(focal?'docs/evidence/l04-l05-parte1-fix/visual':'docs/evidence/l04-l05-parte1/visual');fs.mkdirSync(out,{recursive:true});
const bundle=path.resolve('build/part1-visual-bundle');
await build({root:path.resolve('frontend'),configFile:false,plugins:[react()],build:{outDir:bundle,emptyOutDir:true,rollupOptions:{input:path.resolve('frontend/visual/equipment.html')}}});
const server=await preview({root:path.resolve('frontend'),configFile:false,build:{outDir:bundle},preview:{host:'127.0.0.1',port:0}}),port=server.httpServer.address().port;
const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true});const rows=[];
const sha=p=>crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');
const files=dir=>fs.readdirSync(dir,{withFileTypes:true}).flatMap(entry=>entry.isDirectory()?files(path.join(dir,entry.name)):[path.join(dir,entry.name)]);
const referenceHTML=path.resolve('docs/reference/infradesktop-ui-0.2.1/INFRADESKTOP-MOCKUPS-DESKTOP-V2.html');
if(sha(referenceHTML)!=='975e8c4a9c4e00e70735dd21e62736af78ea40c52e731100df0d4e8599e120d5')throw Error('Approved reference changed');

try{
 for(const [width,height] of [[1440,860],[720,600]])for(const theme of ['light','dark'])for(const state of (focal?['navigation']:['list','empty','vault','device-form','credential-form','trust','result','inactive','navigation'])){
  const key=`${state}-${theme}-${width}x${height}`,page=await browser.newPage({viewport:{width,height},deviceScaleFactor:1});const errors=[];page.on('pageerror',e=>errors.push(e.message));
  const fixtureState=['empty','vault','result'].includes(state)?state:'list';
  await page.goto(`http://127.0.0.1:${port}/visual/equipment.html?theme=${theme}&state=${fixtureState}`);await page.evaluate(()=>document.fonts.ready);await page.getByLabel('Lista de equipamentos').waitFor();
  await page.getByText(state==='vault'?'Cofre indisponível. O inventário permanece disponível; consultas exigem a chave desta instalação.':state==='empty'?'Nenhum equipamento cadastrado':'Equipamentos cadastrados · SSH',{exact:true}).waitFor();
  let expected=[];
  if(state==='device-form'){await page.getByRole('button',{name:'Adicionar',exact:true}).click();await page.getByRole('dialog',{name:'Adicionar equipamento',exact:true}).waitFor();expected=[{name:'Salvar equipamento'},{name:'Fechar painel'},{selector:'input[placeholder="IPv4 ou IPv6 literal"]'}];}
  if(state==='credential-form'){await page.getByRole('button',{name:'Credenciais',exact:true}).click();await page.getByRole('dialog',{name:'Credenciais de consulta',exact:true}).waitFor();await page.getByLabel('Credencial para editar').selectOption('read');expected=[{name:'Salvar credencial',disabled:true},{name:'Fechar painel'},{selector:'input[type="password"]'}];}
  if(state==='trust'){await page.getByRole('button',{name:'Verificar identidade',exact:true}).click();await page.getByRole('dialog').waitFor();expected=[{name:'Confirmar chave conferida',disabled:true},{selector:'form input'},{name:'Fechar painel'}];}
  if(state==='inactive'){await page.getByLabel('Estado dos equipamentos').selectOption('inactive');await page.getByRole('button',{name:'Reativar',exact:true}).waitFor();expected=[{name:'Reativar'},{name:'Consultar',disabled:true}];}
  if(state==='navigation' && width<1000){await page.getByRole('button',{name:'Navegação',exact:true}).click();await page.getByRole('button',{name:'Configurações',exact:true}).click();expected=[{name:'Selecionar empresa'},{name:'Aparência'},{name:'Segurança'}];}
  const actions=await page.evaluate(inspectActions,expected);if(actions.failures.length){await page.screenshot({path:path.join(out,key+'-failure.png')});throw Error(key+': '+JSON.stringify(actions));}
  const geometry=await page.evaluate(()=>({width:document.documentElement.scrollWidth,height:document.documentElement.scrollHeight,sidebar:document.querySelector('.sidebar').getBoundingClientRect().width,header:document.querySelector('.topbar').getBoundingClientRect().height,zoom:getComputedStyle(document.querySelector('.app')).zoom,font:getComputedStyle(document.querySelector('.app')).fontFamily}));
  if(geometry.width!==width||geometry.height!==height||geometry.header!==60||Number(geometry.zoom)!==1||!geometry.font.includes('Roboto')||errors.length)throw Error(key+' invalid geometry '+JSON.stringify({geometry,errors}));
  const capture=path.join(out,key+'.png');await page.screenshot({path:capture});
  if(width<1000 && state==='navigation'){
   await page.getByRole('button',{name:'Selecionar empresa',exact:true}).focus();await page.keyboard.press('Tab');await page.keyboard.press('Shift+Tab');if(await page.locator(':focus').getAttribute('aria-label')!=='Selecionar empresa')throw Error('Tab/ShiftTab navigation');await page.keyboard.press('Escape');if(await page.locator(':focus').getAttribute('aria-label')!=='Navegação')throw Error('Escape focus');await page.getByRole('button',{name:'Navegação',exact:true}).click();await page.getByRole('button',{name:'Equipamentos',exact:true}).click();if(await page.locator(':focus').getAttribute('aria-label')!=='Navegação')throw Error('selection focus');
  }
  const ref=await browser.newPage({viewport:{width,height},deviceScaleFactor:1});await ref.goto(pathToFileURL(path.resolve('docs/reference/infradesktop-ui-0.2.1/INFRADESKTOP-MOCKUPS-DESKTOP-V2.html')).href+`#tela=4&tema=${theme==='dark'?'escuro':'claro'}&cenario=${state==='empty'||state==='vault'?'vazio':'piloto'}`);await ref.evaluate(()=>document.fonts.ready);
  await ref.evaluate(({width,height})=>{const app=document.querySelector('#app');document.body.append(app);app.style.position='absolute';app.style.left='0px';app.style.top='-40px';app.style.width=width+'px';app.style.height=height+40+'px';app.dataset.compact=width<1250?'true':'false';document.querySelector('.reviewbar').remove();document.querySelector('#viewport').remove();},{width,height});
  const reference=path.join(out,key+'-reference.png');await ref.screenshot({path:reference});
  rows.push({key,state,theme,viewport:[width,height],capture,reference,crop:null,capture_sha256:sha(capture),reference_sha256:sha(reference),geometry,actions,runtime:'Chromium isolated component fixture',reference_relation:['device-form','credential-form','trust','navigation'].includes(state)?'Approved M04 composition context; no original same-state modal. New fields/actions use approved dialog/controls and authorized L04/L05 flow.':'Approved M04 same viewport/theme/list or empty; live fields and BF04 shell/L04 actions are explicit authorized deltas.'});
  await ref.close();await page.close();
 }
 fs.writeFileSync(path.join(out,'capture-manifest.json'),JSON.stringify({fixture:true,rows,inputs:[...files('frontend/src'),...files('frontend/visual'),...files('frontend/public/fonts'),...files(bundle),'frontend/wailsjs/go/presentation/App.js','frontend/wailsjs/go/models.ts','scripts/test-part1-visual.mjs',referenceHTML].map(p=>({path:p,sha256:sha(p)}))},null,2)+'\n');
 console.log(`${rows.length} focal states captured; review of pairs/overlays is a separate recorded gate.`);
}finally{await browser.close();await new Promise(resolve=>server.httpServer.close(resolve));}
