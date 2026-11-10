import fs from 'node:fs';
import path from 'node:path';
import { Linter } from '../frontend/node_modules/eslint/lib/api.js';
import ts from '../frontend/node_modules/typescript-eslint/dist/index.js';
import radioDomain from '../frontend/eslint-radio-domain.mjs';
const filename = process.argv[2];
const owner = path.resolve('frontend/src/features/equipment/radio-contract.ts');
let findings = [];
if (path.resolve(filename) !== owner) {
  const linter = new Linter();
  const messages = linter.verify(fs.readFileSync(filename, 'utf8'), [{
    files: ['**/*.{ts,tsx}'],
    languageOptions: { parser: ts.parser, ecmaVersion: 2022, sourceType: 'module' },
    plugins: { 'radio-domain': radioDomain },
    rules: { 'radio-domain/no-inline-values': 'error' },
  }], { filename: path.basename(filename) });
  findings = messages.filter((message) => message.ruleId === 'radio-domain/no-inline-values' || message.fatal)
    .map((message) => ({ line: message.line || 1, value: message.message }));
}
process.stdout.write(JSON.stringify(findings));
