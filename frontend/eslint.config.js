import radioDomain from './eslint-radio-domain.mjs';
import js from '@eslint/js';
import ts from 'typescript-eslint';
import hooks from 'eslint-plugin-react-hooks';
import refresh from 'eslint-plugin-react-refresh';
export default [
  {
    files: ['src/radio*.{ts,tsx}'],
    plugins: { 'radio-domain': radioDomain },
    rules: {
      'radio-domain/no-inline-values': 'error',
      'max-lines-per-function': ['error', 60],
      complexity: ['error', 15],
    },
  },
  { files: ['src/radio*.tsx'], rules: { 'max-lines': ['error', 600] } },
  { ignores: ['dist/**', 'wailsjs/**'] },
  js.configs.recommended,
  ...ts.configs.recommended,
  {
    files: ['src/**/*.{ts,tsx}', 'visual/**/*.{ts,tsx}'],
    plugins: { 'react-hooks': hooks, 'react-refresh': refresh },
    rules: {
      ...hooks.configs.recommended.rules,
      'react-refresh/only-export-components': 'warn',
    },
  },
];
