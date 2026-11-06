import { test } from 'node:test';
import assert from 'node:assert/strict';
import { Linter } from 'eslint';
import ts from 'typescript-eslint';
import { limits, plugin, rulesFor } from './eslint-quality.mjs';
const lint = (code, path = 'new.tsx') => new Linter().verify(code, [
 { files: ['**/*.{ts,tsx}'], languageOptions: { parser: ts.parser }, plugins: { quality: plugin },
   rules: rulesFor(path), linterOptions: { noInlineConfig: true } },
], { filename: path });
test('global limits cannot grow', () => {
 assert.deepEqual(limits, { 'max-lines-per-function': 60, complexity: 15, 'max-lines': 500,
  'max-params': 5, 'max-depth': 4, 'quality/state-count': 8, 'quality/ref-count': 6 });
});
test('61-line function and new oversized source are rejected', () => {
 const code = 'function f() {\n' + Array.from({ length: 59 }, (_, n) => `const x${n} = ${n};`).join('\n') + '\n}';
 assert(lint(code).some((m) => m.ruleId === 'max-lines-per-function'));
 assert(lint(Array.from({ length: 501 }, (_, n) => `const x${n} = ${n};`).join('\n')).some((m) => m.ruleId === 'max-lines'));
});
test('inline suppression has no effect and nine states are rejected', () => {
 const code = '/* eslint-disable quality/state-count */\nfunction f() {\n' + 'useState(0);\n'.repeat(9) + '}';
 assert(lint(code).some((m) => m.ruleId === 'quality/state-count'));
});
