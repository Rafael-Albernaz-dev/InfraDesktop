import {test} from 'node:test';
import assert from 'node:assert/strict';
import {chromium} from '../frontend/node_modules/@playwright/test/index.mjs';
import {inspectActions} from './visual-actions.mjs';
test('closed disclosure exposes its summary; required hidden controls still fail', async () => {
 const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true});
 try {
  const page=await browser.newPage({viewport:{width:720,height:600}});
  await page.setContent('<div class="app"><details><summary>Porta SSH</summary><input aria-label="Porta" value="22"></details></div>');
  assert.deepEqual((await page.evaluate(inspectActions,[])).failures,[]);
  assert.ok((await page.evaluate(inspectActions,[{selector:'input'}])).failures.some(x=>x.includes('hidden')));
  await page.locator('summary').click();
  assert.deepEqual((await page.evaluate(inspectActions,[{selector:'input'}])).failures,[]);
 } finally {await browser.close();}
});
test('real DOM rejects removed, hidden, disabled, ancestor clipping and occlusion; scroll remains reachable',async()=>{
 const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true});
 try{
 const page=await browser.newPage({viewport:{width:720,height:600}});
 const html='<div class="app"><div id="holder" style="width:120px"><button style="width:100px;height:40px">Salvar</button></div></div>';
 for(const mutate of [()=>document.querySelector('button').remove(),()=>document.querySelector('button').style.display='none',()=>document.querySelector('button').disabled=true,()=>document.querySelector('button').style.opacity='0',()=>document.querySelector('#holder').style.transform='scale(.8)',()=>document.querySelector('button').style.transform='scale(.8)',()=>document.querySelector('button').style.zoom='.8',()=>{const p=document.querySelector('#holder');p.style.width='75px';p.style.overflow='hidden';},()=>{const d=document.createElement('div');d.style='position:fixed;inset:0;background:white';document.body.append(d);}]){
  await page.setContent(html);assert.equal((await page.evaluate(inspectActions,[{name:'Salvar'}])).failures.length,0);
  await page.evaluate(mutate);assert.ok((await page.evaluate(inspectActions,[{name:'Salvar'}])).failures.length>0);
 }
 await page.setContent('<div class="app" style="height:200px;overflow:auto"><div style="height:800px"></div><button style="width:100px;height:40px">Salvar</button></div>');
 assert.equal((await page.evaluate(inspectActions,[{name:'Salvar'}])).failures.length,0);
 await page.locator('.app').evaluate(el=>{el.scrollTop=0;el.style.overflow='hidden';});
 assert.ok((await page.evaluate(inspectActions,[{name:'Salvar'}])).failures.some(f=>f.includes('programmatic-only')));
 }finally{await browser.close();}
});
