const families = [
  'auto',
  'auto-avulso',
  'unsupported',
  'routeros-wireless-v6',
  'airos-v6',
  'intelbras-wom',
];
const rolesAndModes = [
  'ap',
  'cpe',
  'ptp',
  'ssh',
  'once',
  'measure',
  'live',
  'index',
];
const values = new Set([
  ...families,
  ...rolesAndModes,
  'UNSUPPORTED',
  'REMOTE_PERMISSION_DENIED',
]);
export const isRadioDomainValue = (value) =>
  typeof value === 'string' &&
  (values.has(value) || /^(RADIO_|SSH_|REMOTE_)[A-Z_]+$/.test(value));
export default {
  rules: {
    'no-inline-values': {
      meta: {
        type: 'problem',
        schema: [],
        messages: {
          inline: 'Use the typed radio contract constant for {{value}}.',
        },
      },
      create(context) {
        if (
          context.filename
            .replaceAll('\\', '/')
            .endsWith('/src/radio-contract.ts')
        )
          return {};
        const check = (node, value) => {
          if (
            isRadioDomainValue(value) &&
            !isRadioTestOracle(node, context.filename)
          )
            context.report({ node, messageId: 'inline', data: { value } });
        };
        return {
          Literal(node) {
            check(node, node.value);
          },
          TemplateLiteral(node) {
            if (node.expressions.length === 0)
              check(node, node.quasis[0].value.cooked);
          },
        };
      },
    },
  },
};

function expectationRoot(node) {
  let current = node;
  while (current?.type === 'MemberExpression') current = current.object;
  return (
    current?.type === 'CallExpression' &&
    current.callee.type === 'Identifier' &&
    current.callee.name === 'expect'
  );
}

// Expected wire values and visible text remain independent of production constants.
export function isRadioTestOracle(node, filename) {
  if (!/\.test\.[jt]sx?$/.test(filename)) return false;
  for (
    let child = node, parent = node.parent;
    parent;
    child = parent, parent = parent.parent
  ) {
    if (parent.type === 'CallExpression' && !parent.arguments.includes(child))
      continue;
    if (
      parent.type !== 'CallExpression' ||
      parent.callee.type !== 'MemberExpression'
    )
      continue;
    const member = parent.callee;
    if (member.property.type !== 'Identifier') continue;
    if (
      /^to(Be|Equal|Match|Have|Contain)/.test(member.property.name) &&
      expectationRoot(member.object)
    )
      return true;
    if (
      isVisibleQueryReceiver(member.object) &&
      /^(get|query|find)(All)?By/.test(member.property.name)
    )
      return true;
  }
  return false;
}

function isVisibleQueryReceiver(receiver) {
  const screen = receiver.type === 'Identifier' && receiver.name === 'screen';
  const within =
    receiver.type === 'CallExpression' &&
    receiver.callee.type === 'Identifier' &&
    receiver.callee.name === 'within';
  return screen || within;
}
