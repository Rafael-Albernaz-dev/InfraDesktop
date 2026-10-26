// New A states only. Real React component, isolated bridge, no firmware/Wails claim.
import { chromium, expect } from '../frontend/node_modules/@playwright/test/index.mjs';
import { build, preview } from '../frontend/node_modules/vite/dist/node/index.js';
import react from '../frontend/node_modules/@vitejs/plugin-react/dist/index.js';
import fs from 'node:fs';
import path from 'node:path';
const out = path.resolve(process.env.INFRADESKTOP_EVIDENCE_DIR || 'build/l06-states', `states-${Date.now()}`);
fs.mkdirSync(out, { recursive: true });
const bundle = path.join(out, 'bundle');
await build({ root: path.resolve('frontend'), configFile: false, plugins: [react()], build: { outDir: bundle, emptyOutDir: true, rollupOptions: { input: path.resolve('frontend/visual/pppoe.html') } } });
const server = await preview({ root: path.resolve('frontend'), configFile: false, build: { outDir: bundle }, preview: { host: '127.0.0.1', port: 0, strictPort: true } });
const browser = await chromium.launch({ executablePath: '/usr/bin/google-chrome', headless: true });
const records = [];
const states = {
  uncollected: 'Nenhuma leitura de sessões disponível.',
  loading: 'Carregando leitura armazenada…',
  empty: 'Nenhuma sessão ativa nesta leitura completa.',
  'no-result': 'Nenhuma sessão corresponde aos filtros.',
  error: 'Reabrir leitura armazenada',
  'page-error': 'Não foi possível',
  partial: 'Tentativa incompleta:',
  limited: 'Tentativa limitada:',
};
try {
  for (const [width, height] of [[720, 600], [1440, 900]]) for (const theme of ['light', 'dark']) for (const [state, oracle] of Object.entries(states)) {
    const page = await browser.newPage({ viewport: { width, height } });
    try {
      await page.goto(`http://127.0.0.1:${server.httpServer.address().port}/visual/pppoe.html?theme=${theme}&state=${state}`);
      if (state === 'no-result') await page.getByLabel('Buscar sessões', { exact: true }).fill('sem-correspondencia');
      if (state === 'page-error') await expect(page.getByRole('alert')).toBeVisible();
      else await expect(page.getByText(oracle, { exact: false })).toBeVisible();
      if (state === 'loading') {
        await expect(page.getByRole('button', { name: 'Atualizar sessões' })).toBeDisabled();
        await expect(page.getByText('Nenhuma leitura de sessões disponível.', { exact: false })).toHaveCount(0);
      }
      if (['partial', 'limited'].includes(state)) await expect(page.getByText('A tabela continua sendo a última leitura boa', { exact: false })).toBeVisible();
      if (['error', 'page-error'].includes(state)) { await expect(page.getByText('Nenhuma sessão corresponde aos filtros.', { exact: false })).toHaveCount(0); await expect(page.getByText('0 resultados', { exact: true })).toHaveCount(0); }
      await page.screenshot({ path: path.join(out, `${state}-${width}-${theme}.png`) });
      const geometry = await page.locator('.pppoe-workspace').evaluate(w => ({ width: w.clientWidth, height: w.clientHeight, scrollHeight: w.scrollHeight, overflow: getComputedStyle(w).overflow, documentWidth: document.documentElement.scrollWidth }));
      expect(geometry.documentWidth).toBe(width);
      records.push({ state, width, height, theme, oracle: state === 'page-error' ? 'role=alert and no false no-result' : oracle, geometry, status: 'PASS' });
    } finally { await page.close(); }
  }
} finally {
  await browser.close();
  await new Promise(resolve => server.httpServer.close(resolve));
  fs.writeFileSync(path.join(out, 'states.json'), JSON.stringify({ records, runtime: 'browser synthetic real component; Wails pending', status: records.length === 32 ? 'PASS' : 'INCOMPLETE' }, null, 2) + '\n');
  console.log(JSON.stringify({ out, captures: records.length }));
}
