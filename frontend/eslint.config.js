import { qualityConfigs } from './eslint-quality.mjs';
import radioDomain from './eslint-radio-domain.mjs';
import js from '@eslint/js';
import ts from 'typescript-eslint';
import hooks from 'eslint-plugin-react-hooks';
import refresh from 'eslint-plugin-react-refresh';
export default [
  ...qualityConfigs(),
  {
    files: ['src/radio*.{ts,tsx}'],
    plugins: { 'radio-domain': radioDomain },
    rules: {
      'radio-domain/no-inline-values': 'error',
    },
  },

  { ignores: ['dist/**', 'wailsjs/**'] },
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/ui/**', 'src/**/*.test.tsx'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: 'JSXOpeningElement[name.name="button"]',
          message: 'Use Button do kit.',
        },
        {
          selector: 'JSXOpeningElement[name.name="table"]',
          message: 'Use DataTable do kit.',
        },
        {
          selector: 'JSXOpeningElement[name.name="svg"]',
          message: 'SVG pertence ao kit; gráficos usam Chart.',
        },
        {
          selector: 'JSXAttribute[name.name="style"]',
          message: 'Estilo pertence ao kit.',
        },
        {
          selector: 'Literal[value=/#[0-9a-fA-F]{3,8}\\b/]',
          message: 'Cor pertence a ui/tokens.css.',
        },
      ],
    },
  },
  {
    files: ['src/ui/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: 'Literal[value=/#[0-9a-fA-F]{3,8}/]',
          message: 'Cor pertence a ui/tokens.css.',
        },
      ],
    },
  },
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/ui/Chart.tsx'],
    rules: {
      'no-restricted-imports': ['error', { patterns: ['uplot', 'uplot/*'] }],
    },
  },
  js.configs.recommended,
  ...ts.configs.recommended,
  {
    files: ['src/**/*.{ts,tsx}', 'visual/**/*.{ts,tsx}'],
    plugins: { 'react-hooks': hooks, 'react-refresh': refresh },
    rules: {
      ...hooks.configs.recommended.rules,
      'react-refresh/only-export-components': 'error',
      'react-hooks/exhaustive-deps': 'error',
    },
  },
];
