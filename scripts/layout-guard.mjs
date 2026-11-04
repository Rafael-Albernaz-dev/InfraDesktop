import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export function assertShrinking(current, baseline, label) {
  const allowed = new Set(baseline);
  if (current.length !== new Set(current).size || current.some((item) => !allowed.has(item)))
    throw Error(`${label}: lista cresceu ou contém duplicatas`);
}

export function validateSceneCoverage(scenes, names) {
  if (new Set(scenes.map((s) => s.id)).size !== scenes.length || new Set(scenes.map((s) => s.reference)).size !== scenes.length) throw Error('Cenas ou referências duplicadas');
  const expanded = new Set(scenes.flatMap((s) => [`${s.reference}-claro.png`, `${s.reference}-escuro.png`]));
  if (expanded.size !== names.size || [...names].some((name) => !expanded.has(name))) throw Error('Cobertura difere do manifesto');
  const required = ['login', 'criar', 'recuperar', 'codigo', 'empresas', 'visao', 'equipamentos', 'credenciais', 'config', 'vitrine'];
  if (required.some((route) => !scenes.some((scene) => scene.route === route)) || !scenes.some((scene) => scene.route === 'ficha')) throw Error('Rota sem cena');
  if (scenes.length * 2 !== names.size || scenes.some((s) => !names.has(`${s.reference}-claro.png`) || !names.has(`${s.reference}-escuro.png`))) throw Error('Cenas não correspondem ao manifesto');
}

export function checkLayoutGuard() {
  const read = (name) => JSON.parse(fs.readFileSync(name, 'utf8'));
  for (const name of ['frontend/layout-exceptions.json', 'frontend/layout-exceptions.baseline.json'])
    if (fs.existsSync(name)) throw Error('Lista de exceções de layout não é mais permitida');
  assertShrinking(read('frontend/visual/layout-pending.json').map((x) => x.id),
    read('frontend/visual/layout-pending.baseline.json').map((x) => x.id), 'Cenas pendentes');
  function check(dir) {
    for (const item of fs.readdirSync(dir, { withFileTypes: true })) {
      const file = path.join(dir, item.name);
      if (item.isDirectory()) check(file);
      else if (file.endsWith('.css') && file !== 'frontend/src/ui/tokens.css' &&
        /#[\da-f]{3,8}\b/i.test(fs.readFileSync(file, 'utf8')))
        throw Error(`Cor fora dos tokens: ${file}`);
    }
  }
  check('frontend/src');
  console.log('layout guard: PASS (zero exceções)');
}

if (process.argv[1] === fileURLToPath(import.meta.url)) checkLayoutGuard();
