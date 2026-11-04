// Current synthetic reflow smoke. Native Wails and operational acceptance are separate.
import fs from 'node:fs';
import path from 'node:path';
import { chromium, expect } from '../frontend/node_modules/@playwright/test/index.mjs';
import { createServer } from '../frontend/node_modules/vite/dist/node/index.js';
const out=path.resolve('docs/evidence/l-base-1b/scroll');fs.mkdirSync(out,{recursive:true});
const server=await createServer({root:path.resolve('frontend'),server:{host:'127.0.0.1',port:0,strictPort:true}});await server.listen();
const browser=await chromium.launch({executablePath:'/usr/bin/google-chrome',headless:true});const records=[];
try {
  for(const [width,height] of [[720,600],[1440,900]]) for(const theme of ['light','dark']) {
    const page=await browser.newPage({viewport:{width,height}});const errors=[];page.on('pageerror',e=>errors.push(e.message));
    await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/visual/layout.html?scene=ficha-ccr-centro-sesso-es&theme=${theme}`);
    await page.getByRole('tab',{name:'Sessões',exact:true}).click();
    await page.getByRole('button',{name:'Detalhar cliente-0001 *0',exact:true}).waitFor();
    await page.mouse.move(width-30,height-30);await page.mouse.wheel(0,5000);
    await expect(page.getByText('Pico entre amostras salvas até esta leitura.',{exact:true})).toBeInViewport();
    const button=page.getByRole('button',{name:'Top consumo',exact:true});await button.focus();await expect(button).toBeFocused();await expect(button).toBeInViewport();
    const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth);
    records.push({width,height,theme,errors,overflow,pass:!overflow&&!errors.length});
    await page.screenshot({path:path.join(out,`${width}-${theme}.png`),fullPage:true});await page.close();
  }
} finally {await browser.close();await server.close();}
fs.writeFileSync(path.join(out,'results.json'),JSON.stringify({records},null,2)+'\n');
if(records.some(r=>!r.pass))process.exitCode=1;
console.log(JSON.stringify(records));
