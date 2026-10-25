// Physical wheel input against the frozen fixture; supplements all-action geometry.
import {chromium} from '../frontend/node_modules/@playwright/test/index.mjs';
import {preview} from '../frontend/node_modules/vite/dist/node/index.js';
import fs from 'node:fs';import path from 'node:path';import crypto from 'node:crypto';
const source=path.resolve(process.argv[2]);const out=path.resolve('build/bf04-scroll',new Date().toISOString().replaceAll(':','-'));fs.mkdirSync(out,{recursive:true});
const server=await preview({root:path.resolve('frontend'),configFile:false,build:{outDir:path.join(source,'fixture-bundle')},preview:{host:'127.0.0.1',port:0}});const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true});const rows=[];
try{
 for(const theme of ['light','dark'])for(const [scene,variant,container,target] of [[2,'','.page-body','button:has-text("Abrir empresa")'],[4,'','.page-body','.inspector button'],[5,'','.page-body','.inspector button'],[6,'','.page-body','.inspector button'],[7,'','.page-body','button:has-text("Restaurar padrão")'],[1,'code','.auth','button:has-text("Guardei meu código")'],[3,'security-code','.dialog','button:has-text("Guardei meu código")']]){
  const page=await browser.newPage({viewport:{width:720,height:600}});await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/visual/index.html?scene=${scene}&theme=${theme}&variant=${variant}`);await page.evaluate(()=>document.fonts.ready);
  const area=page.locator(container).first(),action=page.locator(target).first();await action.waitFor({state:'attached'});const initial=await action.boundingBox();const bounds=await area.boundingBox();await page.mouse.move(bounds.x+bounds.width/2,bounds.y+bounds.height/2);
  let reachable=false;
  for(let step=0;step<8;step++){
   reachable=await action.evaluate(el=>{const r=el.getBoundingClientRect();if(r.y<0||r.bottom>innerHeight||r.x<0||r.right>innerWidth)return false;for(let a=el.parentElement;a;a=a.parentElement){const c=getComputedStyle(a),b=a.getBoundingClientRect();if(/auto|scroll|hidden|clip/.test(c.overflowY)&&(r.top<b.top||r.bottom>b.bottom))return false;if(/auto|scroll|hidden|clip/.test(c.overflowX)&&(r.left<b.left||r.right>b.right))return false;}return [[r.x+2,r.y+2],[r.right-2,r.y+2],[r.x+2,r.bottom-2],[r.right-2,r.bottom-2],[r.x+r.width/2,r.y+r.height/2]].every(([x,y])=>{const hit=document.elementFromPoint(x,y);return hit&&(hit===el||el.contains(hit));});});
   if(reachable)break;
   await page.mouse.wheel(0,480);await page.waitForFunction(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(true)))));
  }
  if(!reachable)throw Error(`wheel cannot reach ${scene}/${variant}/${theme}`);
  const capture=path.join(out,`${scene}-${variant||'base'}-${theme}.png`);await page.screenshot({path:capture});rows.push({scene,variant,theme,viewport:[720,600],initial,final:await action.boundingBox(),capture,sha256:crypto.createHash('sha256').update(fs.readFileSync(capture)).digest('hex'),input:'mouse.wheel, no scrollIntoView/evaluate scrolling',PASS:true});await page.close();
 }
 fs.writeFileSync(path.join(out,'manifest.json'),JSON.stringify({source,source_manifest_sha256:crypto.createHash('sha256').update(fs.readFileSync(path.join(source,'capture-manifest.json'))).digest('hex'),rows},null,2)+'\n');fs.writeFileSync('build/bf04-scroll/latest.txt',out+'\n');console.log(`14 actual wheel paths PASS: ${out}`);
}finally{await browser.close();await new Promise(resolve=>server.httpServer.close(resolve));}
