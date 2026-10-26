// Browser is auxiliary: real React/shell with isolated synthetic bindings only.
import { chromium, expect } from '../frontend/node_modules/@playwright/test/index.mjs';
import { build, preview } from '../frontend/node_modules/vite/dist/node/index.js';
import react from '../frontend/node_modules/@vitejs/plugin-react/dist/index.js';
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
const out=path.resolve(process.env.INFRADESKTOP_EVIDENCE_DIR||'build/l06-diagnostics',`browser-${Date.now()}`);
fs.mkdirSync(out,{recursive:true});
const bundle=path.join(out,'bundle');
const hash=file=>crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
const sourceFiles=['frontend/src/pppoe.tsx','frontend/src/pppoe-diagnostics.tsx','frontend/src/pppoe.css','frontend/src/desktop.tsx','frontend/src/style.css','frontend/visual/pppoe.tsx'];
const sourceHashes=Object.fromEntries(sourceFiles.map(file=>[file,hash(file)]));
await build({root:path.resolve('frontend'),configFile:false,plugins:[react()],build:{outDir:bundle,emptyOutDir:true,rollupOptions:{input:path.resolve('frontend/visual/pppoe.html')}}});
const server=await preview({root:path.resolve('frontend'),configFile:false,build:{outDir:bundle},preview:{host:'127.0.0.1',port:0,strictPort:true}});
const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true});
const records=[];
const selections=process.argv.find(value=>value.startsWith('--cases='))?.slice(8).split(',');
const selected=(scope,state)=>!selections||selections.some(value=>value===scope+':*'||value===scope+':'+state);
const statesFor=scope=>['ready','loading','uncollected','error','partial','limited','stale',...(scope==='ping'?['no-reply']:['empty']),...(scope==='catalog'?['ambiguous']:[])];
const expected=4*['catalog','radius','ping'].reduce((total,scope)=>total+statesFor(scope).filter(state=>selected(scope,state)).length,0);
if(!expected)throw Error('No valid requested states');
try {
 for(const [width,height] of [[720,600],[1440,900]]) for(const theme of ['light','dark']) for(const scope of ['catalog','radius','ping']) {
  const states=statesFor(scope).filter(state=>selected(scope,state));
  for(const state of states){
   const page=await browser.newPage({viewport:{width,height}});
   const filename=`${scope}-${state}-${width}-${theme}.png`;
   try {
    await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/visual/pppoe.html?theme=${theme}&diagnostic=${state}`);
    if(scope==='ping') await page.getByRole('button',{name:'Detalhar Operação sintética 0 row-0',exact:true}).click();
    else await page.getByRole('tab',{name:scope==='catalog'?'VLANs e serviços':'RADIUS',exact:true}).click();
    const surface=page.getByRole('region',{name:scope==='catalog'?'Serviços, interfaces e VLANs':scope==='radius'?'RADIUS no concentrador':'Sondagem da sessão pelo concentrador',exact:true});
    let oracle='';
    if(state==='loading') {oracle=scope==='ping'?'Carregando última sondagem armazenada…':'Carregando diagnóstico armazenado…';await expect(surface.getByText(oracle,{exact:true})).toBeVisible();await expect(surface.getByRole('button',{name:scope==='ping'?'Sondar IP pelo concentrador':scope==='catalog'?'Atualizar serviços e VLANs':'Atualizar RADIUS',exact:true})).toBeDisabled();}
    else if(state==='error'){oracle='role=alert and local stored-data recovery';await expect(surface.getByRole('alert')).toBeVisible();await expect(surface.getByRole('button',{name:/Reabrir .*armazenad/})).toBeVisible();}
    else if(state==='uncollected'){oracle=scope==='ping'?'Nenhuma sondagem armazenada para esta sessão.':'Nenhuma leitura deste diagnóstico disponível.';await expect(surface.getByText(oracle,{exact:false})).toBeVisible();}
    else if(state==='empty'){oracle=scope==='radius'?'Nenhum servidor RADIUS configurado nesta leitura.':'Nenhum item configurado nesta leitura completa.';await expect(surface.getByText(oracle,{exact:true})).toBeVisible();}
    else {
     oracle=scope==='ping'?'Origem: concentrador · main':'Última leitura boa:';
     await expect(surface.getByText(oracle,{exact:false})).toBeVisible();
     if(['partial','limited','stale'].includes(state)) await expect(surface.getByText(/(última leitura boa|última sondagem boa) e sua data foram preservadas/)).toBeVisible();
     if(state==='no-reply'){await expect(surface.getByText(/Recebidos: 0 · Perda: 100%/)).toBeVisible();await expect(surface.getByText(/RTT mínimo: Indisponível/)).toBeVisible();}
     if(state==='ambiguous')await expect(surface.getByText(/Vínculo ambíguo/)).toBeVisible();
    }
    if(scope!=='ping'){
     await expect(page.getByRole('button',{name:'Atualizar sessões',exact:true})).toHaveCount(0);
     await expect(page.getByRole('heading',{name:'Referências preservadas',exact:true})).toHaveCount(0);
     await expect(surface.locator('button.primary')).toHaveCount(1);
    }
    // Align capture with the state actually asserted. This alignment is not
    // counted as physical scrolling; the ready path measures a real wheel below.
    await surface.evaluate(node=>node.scrollIntoView({block:'start'}));
    if(state==='ambiguous')await surface.getByText(/Vínculo ambíguo/).evaluate(node=>node.scrollIntoView({block:'center'}));
    await page.screenshot({path:path.join(out,filename)});
    const geometry=await page.locator('.pppoe-workspace').evaluate(node=>({width:node.clientWidth,height:node.clientHeight,scrollHeight:node.scrollHeight,overflow:getComputedStyle(node).overflow,documentWidth:document.documentElement.scrollWidth}));
    expect(geometry.documentWidth).toBe(width);
    const record={scope,state,width,height,theme,oracle,geometry,capture:filename,sha256:hash(path.join(out,filename)),status:'PASS',runtime:'Chromium isolated synthetic bindings; native proof separate'};
    if(state==='ready'){
     const keyboard=[];
     for(let i=0;i<6;i++){await page.keyboard.press('Tab');keyboard.push(await page.evaluate(()=>({tag:document.activeElement.tagName,text:(document.activeElement.textContent||'').trim(),focusVisible:document.activeElement.matches(':focus-visible')})));}
     await page.keyboard.press('Shift+Tab');
     const pane=await page.locator('.pppoe-workspace').boundingBox();await page.mouse.move(pane.x+pane.width/2,pane.y+pane.height/2);await page.mouse.wheel(0,5000);
     const bottom=`${scope}-bottom-${width}-${theme}.png`;await page.screenshot({path:path.join(out,bottom)});
     record.keyboard=keyboard;record.bottom=bottom;record.bottomSha256=hash(path.join(out,bottom));
     if(scope==='ping'){await page.keyboard.press('Escape');await expect(page.getByRole('complementary',{name:'Detalhes da sessão'})).toHaveCount(0);await expect(page.getByRole('button',{name:'Detalhar Operação sintética 0 row-0',exact:true})).toBeFocused();record.escape='closes native-compatible detail and returns to exact trigger; browser proof';}
    }
    records.push(record);
   }finally{await page.close();}
  }
 }
}finally{
 await browser.close();await new Promise(resolve=>server.httpServer.close(resolve));
 fs.writeFileSync(path.join(out,'manifest.json'),JSON.stringify({sourceHashes,records,selection:selections||'all',expected,status:records.length===expected?'PASS':'INCOMPLETE',runtime:'browser auxiliary; synthetic fixtures only'},null,2)+'\n');
 console.log(JSON.stringify({out,states:records.length}));
}
if(records.length!==expected)process.exitCode=1;
