import { RadioCodes, RadioFamilies, RadioTransports } from './radio-contract';

const INTERNAL_TOKEN_REPLACEMENTS: Array<[RegExp, string]> = [
  [/tick-(parse|frame|cpu|metric:[\w-]+)/gi, 'campo inválido no ciclo'],
  [
    new RegExp(`\\b${RadioFamilies.routerOSWirelessV6}\\b`, 'gi'),
    'MikroTik RouterOS Wireless v6',
  ],
  [new RegExp(`\\b${RadioFamilies.airOSV6}\\b`, 'gi'), 'Ubiquiti airOS v6'],
  [new RegExp(`\\b${RadioFamilies.intelbrasWOM}\\b`, 'gi'), 'Intelbras WOM'],
  [
    new RegExp(`\\b${RadioFamilies.standalone}\\b`, 'gi'),
    'detecção automática',
  ],
  [
    new RegExp(`\\b${RadioFamilies.unsupported}\\b`, 'gi'),
    'sem driver compatível',
  ],
  [/\bautomatic-tofu\b/gi, 'aceite no primeiro acesso'],
  [/\bsugerido-confirmado\b/gi, 'sugestão confirmada'],
  [new RegExp(`\\b${RadioTransports.ssh}\\b`, 'g'), 'SSH'],
  [
    new RegExp(`\\b${RadioCodes.REMOTE_AUTH_FAILED}\\b`, 'g'),
    'credencial recusada pelo equipamento',
  ],
  [
    new RegExp(`\\b${RadioCodes.UNSUPPORTED}\\b`, 'g'),
    'família sem driver compatível',
  ],
];

export function sanitizeToken(input: string): string {
  let out = input.replace(/\blive\b/gi, 'ao vivo');
  for (const [pattern, replacement] of INTERNAL_TOKEN_REPLACEMENTS) {
    out = out.replace(pattern, replacement);
  }
  return out;
}

const RAW_ERROR_TOKENS = [
  'connection refused',
  'context deadline exceeded',
  'i/o timeout',
  'no route to host',
  'handshake failed',
  'dial tcp',
  'permission denied',
  'panic:',
  '\\{"',
  '/tmp/',
  '/var/',
];

const RAW_TECHNICAL_PATTERN = new RegExp(
  [
    'tick-(?:parse|frame|cpu|metric:[\\w-]+)',
    RadioFamilies.routerOSWirelessV6,
    RadioFamilies.airOSV6,
    RadioFamilies.intelbrasWOM,
    RadioFamilies.standalone,
    `\\b${RadioFamilies.unsupported}\\b`,
    'automatic-tofu',
    'at\\s+\\S+\\s*\\(',
    '/home/',
    '/etc/',
    'password=',
    'secret=',
    'token=',
    ...RAW_ERROR_TOKENS,
  ].join('|'),
  'i',
);

const RAW_ERROR_FALLBACK_PATTERN = new RegExp(RAW_ERROR_TOKENS.join('|'), 'i');

export function scrubSecrets(raw: string): string {
  return raw
    .replace(/(password|passwd|secret|token|key)\s*[:=]\s*\S+/gi, '$1=[oculto]')
    .replace(/\/home\/[^\s"')]+/g, '[caminho-local]');
}

export function sanitizeVisibleText(
  raw?: string | null,
  fallback = 'Operação indisponível no momento. Verifique os dados do equipamento e tente novamente.',
): { visible: string; technical?: string } {
  const clean = (raw ?? '').trim();
  if (!clean || clean === 'undefined' || clean === 'null' || clean === 'NaN') {
    return { visible: fallback };
  }
  const scrubbed = scrubSecrets(clean);
  if (!RAW_TECHNICAL_PATTERN.test(clean)) {
    return { visible: sanitizeToken(scrubbed) };
  }
  if (RAW_ERROR_FALLBACK_PATTERN.test(clean)) {
    return { visible: fallback, technical: scrubbed };
  }
  return { visible: sanitizeToken(scrubbed), technical: scrubbed };
}

export function TechnicalDetail(props: {
  detail?: string | null;
  label?: string;
}) {
  const clean = scrubSecrets((props.detail ?? '').trim());
  if (!clean) return null;
  return (
    <details className="technical-detail">
      <summary>{props.label || 'Detalhe técnico'}</summary>
      <code>{clean}</code>
    </details>
  );
}
