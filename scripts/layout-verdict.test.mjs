// Aceite L-HIGIENE-A: cena pendente declarada não reprova; qualquer outra falha reprova.
// Escrito por Claude. Copiar para scripts/layout-verdict.test.mjs sem editar.
// Rodar: node --test scripts/layout-verdict.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import { layoutVerdict } from './layout-verdict.mjs';

const sceneIds = ['visao', 'ficha-ptp-0001-enlace', 'sessao-historico'];
const pending = [
  { id: 'ficha-ptp-0001-enlace', owner: 'L07-DRV', reason: 'Dado ainda sem percurso nesta ficha' },
  { id: 'sessao-historico', owner: 'L-PULSO', reason: 'Histórico de conexões ainda não implementado' },
];
const ok = (id) => ({ id, pass: true });
const parked = (id) => ({ id, pass: false, reason: 'Cena pendente' });
const both = (scene, make) => [make(`${scene}-claro`), make(`${scene}-escuro`)];
const full = () => [...both('visao', ok), ...both('ficha-ptp-0001-enlace', parked), ...both('sessao-historico', parked)];

test('tudo verde e pendentes declaradas: exit 0', () => {
  const v = layoutVerdict({ results: full(), pending, sceneIds });
  assert.equal(v.exitCode, 0, v.problems.join('; '));
  assert.deepEqual(v.problems, []);
});

test('falha real em cena não pendente: exit 1 com o id', () => {
  const results = full();
  results[0] = { id: 'visao-claro', pass: false, changed_pixels: 9999 };
  const v = layoutVerdict({ results, pending, sceneIds });
  assert.equal(v.exitCode, 1);
  assert.ok(v.problems.some((p) => p.includes('visao-claro')), v.problems.join('; '));
});

test('falha sem pendência declarada não é perdoada pelo motivo do resultado', () => {
  const v = layoutVerdict({ results: full(), pending: pending.slice(0, 1), sceneIds });
  assert.equal(v.exitCode, 1);
  assert.ok(v.problems.some((p) => p.includes('sessao-historico')), v.problems.join('; '));
});

test('pendente apontando para cena inexistente: exit 1', () => {
  const v = layoutVerdict({ results: full(), pending: [...pending, { id: 'fantasma', owner: 'X', reason: 'Y' }], sceneIds });
  assert.equal(v.exitCode, 1);
  assert.ok(v.problems.some((p) => p.includes('fantasma')), v.problems.join('; '));
});

test('pendente sem dono ou sem motivo: exit 1', () => {
  for (const bad of [{ id: 'sessao-historico', owner: '', reason: 'r' }, { id: 'sessao-historico', owner: 'L-PULSO' }]) {
    const v = layoutVerdict({ results: full(), pending: [pending[0], bad], sceneIds });
    assert.equal(v.exitCode, 1, JSON.stringify(bad));
  }
});

test('pendente repetido: exit 1', () => {
  const v = layoutVerdict({ results: full(), pending: [...pending, pending[0]], sceneIds });
  assert.equal(v.exitCode, 1);
});

test('nenhum resultado: exit 1', () => {
  assert.equal(layoutVerdict({ results: [], pending, sceneIds }).exitCode, 1);
});
