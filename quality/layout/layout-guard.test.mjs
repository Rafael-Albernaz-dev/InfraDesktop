import { ESLint } from '../../frontend/node_modules/eslint/lib/api.js';
import path from 'node:path';
import assert from 'node:assert/strict';
import test from 'node:test';
import { assertShrinking, validateSceneCoverage } from './layout-guard.mjs';

test('exceções e cenas pendentes só encolhem, sem permitir substituir dívida por arquivo novo', () => {
  assert.doesNotThrow(() => assertShrinking(['old.tsx'], ['old.tsx', 'other.tsx'], 'guard'));
  assert.throws(() => assertShrinking(['new.tsx'], ['old.tsx'], 'guard'), /cresceu/);
  assert.throws(() => assertShrinking(['old.tsx', 'old.tsx'], ['old.tsx'], 'guard'), /duplicatas/);
  assert.doesNotThrow(() => assertShrinking([], ['old.tsx'], 'guard'));
});

test('não permite esconder captura obrigatória substituindo-a por cena duplicada', () => {
  const scenes = ['login', 'criar', 'recuperar', 'codigo', 'empresas', 'visao', 'equipamentos', 'credenciais', 'config', 'vitrine', 'ficha-a', 'ficha-b'].map((id) => ({ id, reference: id, route: id.startsWith('ficha') ? 'ficha' : id }));
  const names = new Set(scenes.flatMap((s) => [`${s.reference}-claro.png`, `${s.reference}-escuro.png`]));
  assert.doesNotThrow(() => validateSceneCoverage(scenes, names));
  const replaced = scenes.map((s, i) => i === scenes.length - 1 ? { ...scenes.at(-2), id: 'renamed' } : s);
  assert.throws(() => validateSceneCoverage(replaced, names), /duplic|manifesto/);
});
test('hexadecimal é recusado também dentro do kit e controles comuns só nascem no kit', async () => {
  const eslint = new ESLint({ cwd: path.resolve('frontend') });
  for (const filePath of ['src/probe.tsx', 'src/shared/ui/Probe.tsx']) {
    const [result] = await eslint.lintText("export const color = '#abcdef';", { filePath });
    assert.ok(result.messages.some((m) => m.ruleId === 'no-restricted-syntax'), filePath);
  }
  const [control] = await eslint.lintText('export const label = "Equipamentos";', { filePath: 'src/probe.tsx' });
  assert.equal(control.errorCount, 0);
});
