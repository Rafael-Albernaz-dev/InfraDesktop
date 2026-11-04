import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { chromium } from '../frontend/node_modules/@playwright/test/index.mjs';
import { createServer } from '../frontend/node_modules/vite/dist/node/index.js';
import { checkLayoutGuard, validateSceneCoverage } from './layout-guard.mjs';

checkLayoutGuard();
const read = (name) => JSON.parse(fs.readFileSync(name, 'utf8'));
const reference = 'frontend/visual/reference';
const manifest = fs.readFileSync(`${reference}/capturas.sha256`, 'utf8');
const sha = (file) => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
if (crypto.createHash('sha256').update(manifest).digest('hex') !== 'b31a8a1ab35ae16b2f57eb9dad5400acaa2be178bd32bc32fcb52379cbcf3e0b')
  throw Error('Manifesto congelado divergiu');
const names = new Set();
for (const line of manifest.trim().split('\n')) {
  const [hash, name] = line.trim().split(/\s+/);
  if (sha(`${reference}/${name}`) !== hash) throw Error(`Captura congelada divergiu: ${name}`);
  names.add(name);
}
const scenes = read('frontend/visual/layout-scenes.json');
validateSceneCoverage(scenes, names);
const pending = read('frontend/visual/layout-pending.json');
const out = path.resolve('docs/evidence/l-base-1/layout');
fs.mkdirSync(out, { recursive: true });
const results = [];
const server = await createServer({ root: path.resolve('frontend'), server: { host: '127.0.0.1', port: 0, strictPort: true } });
await server.listen();
const address = server.httpServer.address();
const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', headless: true });
try {
  for (const scene of scenes) for (const [theme, suffix] of [['light', 'claro'], ['dark', 'escuro']]) {
    const id = `${scene.id}-${suffix}`;
    if (pending.some((p) => p.id === scene.id)) { results.push({ id, pass: false, reason: 'Cena pendente' }); continue; }
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1, reducedMotion: 'reduce' });
    const errors = [];
    page.on('pageerror', (e) => errors.push(e.message));
    await page.goto(`http://127.0.0.1:${address.port}/visual/layout.html?scene=${scene.id}&theme=${theme}`);
    await page.locator('[data-layout-ready]').waitFor({ state: 'attached' });
    await page.evaluate(() => document.fonts.ready);
    if (scene.device) {
      await page.getByRole('heading', { name: scene.device, exact: true }).waitFor();
      if (scene.tab) await page.getByRole('tab', { name: scene.tab, exact: true }).click();
      if (scene.sub) await page.getByRole('tab', { name: scene.sub, exact: true }).click();
      if (scene.session) {
        await page.getByRole('button', { name: 'Detalhar cliente-0001 *0', exact: true }).click();
        await page.locator('.ch-plot canvas').waitFor();
      }
    }
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await page.addStyleTag({ content: '*,*::before,*::after{animation:none!important;transition:none!important;caret-color:transparent!important}' });
    const capture = path.join(out, id + '.png');
    await page.screenshot({ path: capture, fullPage: scene.id === 'vitrine' });
    const compared = spawnSync('python3', ['scripts/compare-layout.py', `${reference}/${scene.reference}-${suffix}.png`, capture, path.join(out, id)], { encoding: 'utf8' });
    const metric = compared.stdout.trim() ? JSON.parse(compared.stdout) : { pass: false, reason: compared.stderr };
    results.push({ id, ...metric, errors, pass: metric.pass && errors.length === 0 });
    console.log(id, JSON.stringify(results.at(-1)));
    await page.close();
  }
} finally { await browser.close(); await server.close(); }
fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify({ at: new Date().toISOString(), results }, null, 2) + '\n');
console.log(`layout: ${results.filter((r) => r.pass).length}/${results.length} PASS; ${pending.length} cenas pendentes`);
if (results.some((r) => !r.pass)) process.exitCode = 1;
