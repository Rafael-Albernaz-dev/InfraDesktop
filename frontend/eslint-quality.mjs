import fs from 'node:fs';
import { URL } from 'node:url';

export const limits = {
  'max-lines-per-function': 60, complexity: 15, 'max-lines': 500,
  'max-params': 5, 'max-depth': 4, 'quality/state-count': 8, 'quality/ref-count': 6,
};
const hookRule = (hook, defaultLimit) => ({
  meta: { type: 'problem', schema: [{ type: 'integer', minimum: 0 }], messages: { excess: '{{hook}}: {{count}} exceeds {{limit}}.' } },
  create(context) {
    const stack = [];
    const enter = () => stack.push(0);
    const leave = (node) => {
      const count = stack.pop(), limit = context.options[0] ?? defaultLimit;
      if (count > limit) context.report({ node, messageId: 'excess', data: { hook, count, limit } });
    };
    return {
      ':function': enter, ':function:exit': leave,
      CallExpression(node) {
        const callee = node.callee;
        const name = callee.type === 'Identifier' ? callee.name : callee.type === 'MemberExpression' ? callee.property.name : '';
        if (name === hook && stack.length) stack[stack.length - 1]++;
      },
    };
  },
});
const nested = {
 meta: { type: 'problem', schema: [{ type: 'integer', minimum: 0 }] },
 create(context) {
  const found = [];
  return { ConditionalExpression(node) {
   if (node.consequent.type === 'ConditionalExpression' || node.alternate.type === 'ConditionalExpression') found.push(node);
  }, 'Program:exit'() {
   for (const node of found.slice(context.options[0] ?? 0)) context.report({ node, message: 'Do not nest ternary expressions.' });
  } };
 }
};
export const plugin = { rules: { 'state-count': hookRule('useState', 8), 'ref-count': hookRule('useRef', 6), 'nested-ternary': nested } };
export function rulesFor(path, ceilings = {}) {
  const defaults = { ...limits, 'max-lines-per-function': /\.test\.[jt]sx?$/.test(path) ? 120 : 60 };
  const rules = Object.fromEntries(Object.entries(defaults).map(([name, value]) => {
    const limit = ceilings[name] ?? value;
    const option = name.startsWith('max-lines') ? { max: limit, skipBlankLines: true, skipComments: true } : limit;
    return [name, ['error', option]];
  }));
  return { ...rules, 'no-nested-ternary': 'error', ...(ceilings['no-nested-ternary'] === undefined ? {} : {
    'no-nested-ternary': 'off', 'quality/nested-ternary': ['error', ceilings['no-nested-ternary']],
  }) };
}
export function qualityConfigs() {
  const path = new URL('./quality-ratchet.json', import.meta.url);
  const ratchet = fs.existsSync(path) ? JSON.parse(fs.readFileSync(path, 'utf8')) : {};
  return [
    { files: ['src/**/*.{ts,tsx}', 'visual/**/*.{ts,tsx}'], plugins: { quality: plugin }, rules: rulesFor('') },
    { files: ['**/*.test.{ts,tsx}'], rules: rulesFor('fixture.test.tsx') },
    ...Object.entries(ratchet).map(([file, ceilings]) => ({ files: [file], rules: rulesFor(file, ceilings) })),
  ];
}
