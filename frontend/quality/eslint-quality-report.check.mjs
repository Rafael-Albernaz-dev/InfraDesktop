import { test } from 'node:test';
import assert from 'node:assert/strict';
import { URL, fileURLToPath } from 'node:url';
import { frontendMeasurements } from './eslint-quality-report.mjs';

test('G8 measures actual TypeScript functions and reports strict debt', async () => {
  const source = [
    'export function measured(value: number) {',
    'if (value > 0) value++;',
    ...Array(57).fill('value += 1;'),
    'return value;',
    '}',
    '',
  ].join('\n');
  const filePath = fileURLToPath(
    new URL('../src/app/new-report-fixture.ts', import.meta.url),
  );
  const { metrics, debt } = await frontendMeasurements([{ filePath, source }]);
  assert.equal(metrics['src/app/new-report-fixture.ts'].lines, 61);
  assert.equal(metrics['src/app/new-report-fixture.ts'].function, 61);
  assert.equal(metrics['src/app/new-report-fixture.ts'].complexity, 2);
  assert.equal(debt['max-lines-per-function'], 1);
});
