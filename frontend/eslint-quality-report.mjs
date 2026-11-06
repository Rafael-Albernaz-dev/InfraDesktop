import console from 'node:console';
import fs from 'node:fs';
import process from 'node:process';
import { URL, fileURLToPath, pathToFileURL } from 'node:url';
import { ESLint } from 'eslint';
import { rulesFor } from './eslint-quality.mjs';
const root = fileURLToPath(new URL('./', import.meta.url));
const measurementRules = {
  ...rulesFor(''),
  'quality/nested-ternary': 'off',
  'max-lines-per-function': [
    'error',
    { max: 0, skipBlankLines: true, skipComments: true },
  ],
  complexity: ['error', 0],
  'max-lines': ['error', { max: 0, skipBlankLines: true, skipComments: true }],
};
export async function frontendMeasurements(sources) {
  const lint = new ESLint({
    cwd: root,
    allowInlineConfig: false,
    overrideConfig: [
      {
        files: ['src/**/*.{ts,tsx}', 'visual/**/*.{ts,tsx}'],
        rules: measurementRules,
      },
    ],
  });
  const results = sources
    ? (
        await Promise.all(
          sources.map((s) => lint.lintText(s.source, { filePath: s.filePath })),
        )
      ).flat()
    : await lint.lintFiles(['src', 'visual']);
  const textByPath = new Map(sources?.map((s) => [s.filePath, s.source]) || []),
    metrics = {},
    debt = {};
  for (const file of results) {
    const name = file.filePath.slice(root.length),
      source =
        textByPath.get(file.filePath) ?? fs.readFileSync(file.filePath, 'utf8');
    const metric = {
      lines: source ? source.replace(/\n$/, '').split('\n').length : 0,
      function: 0,
      complexity: 0,
    };
    for (const m of file.messages) {
      let value, threshold;
      if (m.ruleId === 'max-lines-per-function') {
        value = Number(m.message.match(/\((\d+)\)/)?.[1] || 0);
        metric.function = Math.max(metric.function, value);
        threshold = /\.test\.[jt]sx?$/.test(name) ? 120 : 60;
      } else if (m.ruleId === 'complexity') {
        value = Number(m.message.match(/complexity of (\d+)/)?.[1] || 0);
        metric.complexity = Math.max(metric.complexity, value);
        threshold = 15;
      } else if (m.ruleId === 'max-lines') {
        value = Number(m.message.match(/\((\d+)\)/)?.[1] || 0);
        threshold = 500;
      }
      if (threshold !== undefined && value <= threshold) continue;
      debt[m.ruleId] = (debt[m.ruleId] || 0) + 1;
    }
    metrics[name] = metric;
  }
  return { metrics, debt };
}
async function report() {
  const lint = new ESLint({ cwd: root, allowInlineConfig: false }),
    active = await lint.lintFiles(['src', 'visual']);
  const { metrics, debt } = await frontendMeasurements();
  console.log('strict violations by rule:', JSON.stringify(debt));
  const ratchet = JSON.parse(
    fs.readFileSync(new URL('./quality-ratchet.json', import.meta.url), 'utf8'),
  );
  console.log('ratchet files:', Object.keys(ratchet).length);
  console.log(JSON.stringify(ratchet, null, 2));
  console.log(
    'warnings:',
    active.reduce((n, r) => n + r.warningCount, 0),
  );
  console.log(
    'frontend metrics (physical lines; ESLint function lines and complexity):',
  );
  for (const [name, m] of Object.entries(metrics).sort(([a], [b]) =>
    a.localeCompare(b),
  ))
    console.log(
      `${name} lines=${m.lines} function=${m.function} complexity=${m.complexity}`,
    );
}
if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url)
  await report();
