import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { ESLint, Linter } from 'eslint';
import ts from 'typescript-eslint';
import { plugin, rulesFor } from './eslint-quality.mjs';
test('every real source retains its effective quality limits', async () => {
  const lint = new ESLint();
  const ratchet = JSON.parse(
    fs.readFileSync('quality/quality-ratchet.json', 'utf8'),
  );
  const files = (dir) =>
    fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
      const path = dir + '/' + entry.name;
      return entry.isDirectory()
        ? files(path)
        : /\.tsx?$/.test(path)
          ? [path]
          : [];
    });
  for (const path of [...files('src'), ...files('visual')]) {
    assert.equal(await lint.isPathIgnored(path), false, path + ' ignored');
    const config = await lint.calculateConfigForFile(path);
    for (const [rule, expected] of Object.entries(
      rulesFor(path, ratchet[path]),
    )) {
      const normalized = Array.isArray(expected)
        ? [2, ...expected.slice(1)]
        : [expected === 'off' ? 0 : 2];
      assert.deepEqual(config.rules[rule], normalized, path + ': ' + rule);
    }
    for (const rule of [
      'react-hooks/exhaustive-deps',
      'react-refresh/only-export-components',
    ])
      assert.equal(config.rules[rule][0], 2, path + ': ' + rule);
  }
  assert.match(
    fs.readFileSync('../quality/run.sh', 'utf8'),
    /eslint \. --max-warnings 0 --no-inline-config/,
  );
});
test('G2 accepts its boundary and rejects the seventh ref', () => {
  const config = [
    {
      files: ['**/*.tsx'],
      languageOptions: { parser: ts.parser },
      plugins: { quality: plugin },
      rules: rulesFor('fixture.tsx'),
    },
  ];
  const run = (n) =>
    new Linter().verify(
      'function f(){' + 'useRef(0);'.repeat(n) + 'useState(0);'.repeat(8) + '}',
      config,
      { filename: 'fixture.tsx' },
    );
  assert(!run(6).some((m) => m.ruleId.startsWith('quality/')));
  assert(run(7).some((m) => m.ruleId === 'quality/ref-count'));
});
