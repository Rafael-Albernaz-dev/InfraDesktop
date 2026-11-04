import { RadioFamilies, RadioModes, RadioRoles } from './radio-contract';
export { formatPPUptime } from './pppoe-format';
export { message, recoveryActionMessage } from './microcopy';
export type { Intent } from './microcopy';

/** Shared pt-BR network presentation. Inputs remain raw for sorting and CSV. */
export type NetworkNumber = number | string | bigint | null | undefined;
export type NumberRange = { min: number; max: number };
const unavailable = 'Não informado nesta leitura';
const volumeUnits = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
const rateUnits = ['bps', 'Kbps', 'Mbps', 'Gbps', 'Tbps'];
const formatter = (digits = 0, minimum = 0) =>
  new Intl.NumberFormat('pt-BR', {
    maximumFractionDigits: digits,
    minimumFractionDigits: minimum,
  });
function numeric(value: NetworkNumber): number | undefined {
  if (value === null || value === undefined || value === '') return;
  if (typeof value === 'string' && !/^\d+(?:\.\d+)?$/.test(value)) return;
  const n = Number(value);
  return Number.isFinite(n) && n >= 0 ? n : undefined;
}
const digits = (n: number) => (n < 10 ? 2 : n < 100 ? 1 : 0);
export function networkScale(
  maximum: NetworkNumber,
  kind: 'volume' | 'rate',
  upper = false,
) {
  const n = numeric(maximum);
  const base = kind === 'volume' ? 1024 : 1000;
  const units = kind === 'volume' ? volumeUnits : rateUnits;
  let index = 0;
  if (n !== undefined) {
    while (index < units.length - 1 && n / base ** (index + 1) >= 1) index++;
    // Rounding must never render 1024 KB or 1000 Mbps.
    while (
      index < units.length - 1 &&
      (upper
        ? Math.ceil((n / base ** index) * 10 ** digits(n / base ** index)) /
          10 ** digits(n / base ** index)
        : Number((n / base ** index).toFixed(digits(n / base ** index)))) >=
        base
    )
      index++;
  }
  return { factor: base ** index, unit: units[index], index, kind };
}
export function formatAtScale(
  value: NetworkNumber,
  scale: ReturnType<typeof networkScale>,
  withUnit = true,
): string {
  const n = numeric(value);
  if (n === undefined) return unavailable;
  const scaled = n / scale.factor;
  const rounded = Number(scaled.toFixed(digits(scaled)));
  const minimum =
    scale.kind === 'volume' && scale.index > 0 && rounded >= 1 && rounded < 10
      ? 1
      : 0;
  return (
    formatter(digits(scaled), minimum).format(scaled) +
    (withUnit ? ' ' + scale.unit : '')
  );
}
export function formatVolume(value: NetworkNumber): string {
  return formatAtScale(value, networkScale(value, 'volume'));
}
export function formatRange(
  value: NumberRange | null | undefined,
  kind: 'volume' | 'rate',
): string {
  if (
    !value ||
    numeric(value.min) === undefined ||
    numeric(value.max) === undefined ||
    value.min > value.max
  )
    return unavailable;
  const scale = networkScale(value.max, kind, value.min !== value.max);
  if (value.min === value.max) return formatAtScale(value.min, scale);
  const outward = (n: number, upper: boolean) => {
    const scaled = n / scale.factor,
      power = 10 ** digits(scaled);
    return (
      (upper ? Math.ceil(scaled * power) : Math.floor(scaled * power)) / power
    );
  };
  const low = formatAtScale(
    outward(value.min, false) * scale.factor,
    scale,
    false,
  );
  return `${low}–${formatAtScale(outward(value.max, true) * scale.factor, scale, false)} ${scale.unit}`;
}
export function formatRate(value: NetworkNumber | NumberRange): string {
  return typeof value === 'object' && value !== null
    ? formatRange(value, 'rate')
    : formatAtScale(value, networkScale(value, 'rate'));
}
export function formatVolumePair(a: NetworkNumber, b: NetworkNumber): string {
  const values = [numeric(a), numeric(b)].filter(
    (n): n is number => n !== undefined,
  );
  const scale = networkScale(
    values.length ? Math.max(...values) : undefined,
    'volume',
  );
  return `${formatAtScale(a, scale)} / ${formatAtScale(b, scale)}`;
}
export function exactNumber(value: NetworkNumber, unit = ''): string {
  if (numeric(value) === undefined) return unavailable;
  let text: string;
  if (
    typeof value === 'bigint' ||
    (typeof value === 'string' && /^\d+$/.test(value))
  )
    text = BigInt(value).toLocaleString('pt-BR');
  else text = formatter(20).format(Number(value));
  return text + (unit ? ' ' + unit : '');
}
export const exactVolume = (value: NetworkNumber) =>
  exactNumber(value, 'bytes');
export function exactRate(value: NetworkNumber | NumberRange): string {
  if (typeof value === 'object' && value !== null) {
    if (
      numeric(value.min) === undefined ||
      numeric(value.max) === undefined ||
      value.min > value.max
    )
      return unavailable;
    return `${exactNumber(value.min)}–${exactNumber(value.max)} bit/s`;
  }
  return exactNumber(value, 'bit/s');
}
export function formatDuration(value: NetworkNumber, compact = false): string {
  const n = numeric(value);
  if (n === undefined) return unavailable;
  let seconds = Math.floor(n);
  const parts: string[] = [];
  for (const [unit, size] of [
    ['d', 86400],
    ['h', 3600],
    ['min', 60],
    ['s', 1],
  ] as const) {
    const count = Math.floor(seconds / size);
    seconds %= size;
    if (count || (size === 1 && !parts.length))
      parts.push(`${formatter().format(count)}${compact ? '' : ' '}${unit}`);
    if (parts.length === 2) break;
  }
  return parts.join(' ');
}
export function uptimeTitle(value: {
  known: boolean;
  min: number;
  max: number;
}): string {
  if (
    !value.known ||
    numeric(value.min) === undefined ||
    numeric(value.max) === undefined ||
    value.max < value.min
  )
    return unavailable;
  return `${exactNumber(value.min / 1e9)}–${exactNumber(value.max / 1e9)} s (intervalo observado)`;
}
export function formatRouterDuration(value?: string): string {
  if (!value) return unavailable;
  value = value.replace(/\s+/g, '');
  let seconds = 0;
  const units: Record<string, number> = {
    w: 604800,
    d: 86400,
    h: 3600,
    m: 60,
    s: 1,
    ms: 0.001,
  };
  const matches = [...value.matchAll(/(\d+(?:\.\d+)?)(ms|w|d|h|m|s)/g)];
  if (!matches.length || matches.map((m) => m[0]).join('') !== value) {
    const clock =
      /^(?:(\d+)w)?(?:(\d+)d)?(\d+):(\d{2}):(\d{2}(?:\.\d+)?)$/.exec(value);
    return clock && Number(clock[4]) < 60 && Number(clock[5]) < 60
      ? formatDuration(
          Number(clock[1] || 0) * 604800 +
            Number(clock[2] || 0) * 86400 +
            Number(clock[3]) * 3600 +
            Number(clock[4]) * 60 +
            Number(clock[5]),
        )
      : unavailable;
  }
  for (const m of matches) seconds += Number(m[1]) * units[m[2]];
  if (seconds < 1 && matches.some((m) => m[2] === 'ms'))
    return formatMetric(seconds * 1000, 'ms');
  return formatDuration(seconds);
}
export function formatDate(value?: string, history = false): string {
  if (!value || !Number.isFinite(Date.parse(value))) return unavailable;
  return new Date(value)
    .toLocaleString('pt-BR', {
      day: '2-digit',
      month: '2-digit',
      ...(history ? ({ year: 'numeric' } as const) : {}),
      hour: '2-digit',
      minute: '2-digit',
    })
    .replace(',', '');
}
export function formatMetric(
  value: NetworkNumber,
  unit: string,
  empty = unavailable,
): string {
  const n = numeric(value);
  // Signed physical quantities (dBm, dB and temperature) are legitimate.
  if (
    n === undefined &&
    !(
      typeof value === 'number' &&
      Number.isFinite(value) &&
      ['dBm', 'dB', '°C'].includes(unit)
    )
  )
    return empty;
  return (
    formatter(digits(Math.abs(Number(value)))).format(Number(value)) +
    ' ' +
    unit
  );
}
export function formatLatency(ns: NetworkNumber): string {
  const n = numeric(ns);
  return n === undefined ? unavailable : formatMetric(n / 1e6, 'ms');
}
export function formatLatencyRange(value: {
  known: boolean;
  min: number;
  max: number;
}): string {
  if (
    !value.known ||
    numeric(value.min) === undefined ||
    numeric(value.max) === undefined ||
    value.min > value.max
  )
    return unavailable;
  if (value.min === value.max) return formatLatency(value.min);
  const a = value.min / 1e6;
  const b = value.max / 1e6;
  const low = formatter(digits(a)).format(
    Math.floor(a * 10 ** digits(a)) / 10 ** digits(a),
  );
  const high = formatter(digits(b)).format(
    Math.ceil(b * 10 ** digits(b)) / 10 ** digits(b),
  );
  return `${low}–${high} ms`;
}
export function formatClockOffset(value: NumberRange): string {
  if (
    !Number.isFinite(value.min) ||
    !Number.isFinite(value.max) ||
    value.min > value.max
  )
    return unavailable;
  // Signed clock differences are not a negative duration/traffic counter.
  return `${formatter(2).format(value.min / 1e9)}–${formatter(2).format(value.max / 1e9)} s`;
}
export function portLabel(port?: {
  running?: boolean;
  speed_bps?: number;
  full_duplex?: boolean;
}): string {
  if (port?.running === false) return 'DOWN';
  if (port?.running !== true || port.speed_bps === undefined)
    return unavailable;
  return (
    formatRate(port.speed_bps).replace(' ', '') +
    (port.full_duplex === undefined
      ? ' · duplex indisponível'
      : port.full_duplex
        ? '-full'
        : '-half')
  );
}

export function formatAbsent(reason = 'não coletado'): string {
  return `${unavailable} (${(reason || 'não coletado').trim()})`;
}

export function formatTextOrAbsent(
  value?: string | null,
  reason = 'não coletado',
): string {
  const clean = (value ?? '').trim();
  if (!clean || ['—', '-', 'undefined', 'null', 'NaN'].includes(clean)) {
    return formatAbsent(reason);
  }
  return sanitizeToken(clean);
}

export function formatAge(iso?: string, nowMs: number = Date.now()): string {
  const raw = (iso || '').trim();
  if (!raw || !Number.isFinite(Date.parse(raw)))
    return formatAbsent('sem horário');
  const diffSec = Math.max(0, Math.floor((nowMs - Date.parse(raw)) / 1000));
  if (diffSec < 60) return 'agora';
  const mins = Math.floor(diffSec / 60);
  if (mins < 60) return `há ${formatter().format(mins)} min`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `há ${formatter().format(hours)} h`;
  return `há ${formatter().format(Math.floor(hours / 24))} d`;
}

export function formatDateWithAge(
  iso?: string,
  nowMs: number = Date.now(),
  history = false,
): string {
  const formatted = formatDate(iso, history);
  if (formatted === unavailable) return formatAbsent('sem horário');
  return `${formatted} (${formatAge(iso, nowMs)})`;
}

export function formatMacAddress(
  mac?: string | null,
  reason = 'MAC não coletado',
): string {
  const raw = (mac ?? '').trim();
  if (!raw || raw === '—' || raw === '-') return formatAbsent(reason);
  const hex = raw.replace(/[^0-9a-fA-F]/g, '').toUpperCase();
  if (hex.length === 12) {
    return `${hex.slice(0, 2)}:${hex.slice(2, 4)}:${hex.slice(4, 6)}:${hex.slice(6, 8)}:${hex.slice(8, 10)}:${hex.slice(10, 12)}`;
  }
  return raw.toUpperCase();
}

export function formatIPAddress(
  ip?: string | null,
  port?: number | null,
  reason = 'endereço não informado',
): string {
  const clean = (ip ?? '').trim();
  if (!clean || clean === '—' || clean === '-') return formatAbsent(reason);
  return typeof port === 'number' && port > 0 && !clean.includes(':')
    ? `${clean}:${port}`
    : clean;
}

export function formatDeviceModel(
  model?: string | null,
  reason = 'modelo não coletado',
): string {
  return formatTextOrAbsent(model, reason);
}

export function formatFirmwareVersion(
  version?: string | null,
  reason = 'firmware não coletado',
): string {
  return formatTextOrAbsent(version, reason);
}

const ROLE_LABELS: Record<string, string> = {
  concentrator: 'Concentrador PPPoE',
  [RadioRoles.ap]: 'Ponto de acesso (AP)',
  [RadioRoles.cpe]: 'Cliente rádio (CPE)',
  [RadioRoles.ptp]: 'Enlace ponto a ponto (PTP)',
  other: 'Outro equipamento',
};

export function formatEquipmentRole(role?: string | null): string {
  const key = (role ?? '').trim().toLowerCase();
  if (!key) return formatAbsent('papel não definido');
  return ROLE_LABELS[key] || 'Papel personalizado';
}

const FAMILY_LABELS: Record<string, string> = {
  [RadioFamilies.routerOSWirelessV6]: 'MikroTik RouterOS Wireless v6',
  [RadioFamilies.airOSV6]: 'Ubiquiti airOS v6',
  [RadioFamilies.intelbrasWOM]: 'Intelbras WOM',
  [RadioFamilies.automatic]: 'Detecção automática',
  [RadioFamilies.standalone]: 'Detecção automática',
  [RadioFamilies.unsupported]: 'Sem driver compatível nesta versão',
};

export function formatRadioFamily(
  family?: string | null,
  reason = 'família não identificada',
): string {
  const key = (family ?? '').trim();
  if (!key) return formatAbsent(reason);
  return FAMILY_LABELS[key] || formatAbsent(reason);
}

const MODE_LABELS: Record<string, string> = {
  'ap-bridge': 'Ponto de acesso (ap-bridge)',
  'station-bridge': 'Estação (station-bridge)',
  'station-wds': 'Estação WDS (station-wds)',
  station: 'Estação (CPE)',
  bridge: 'Bridge',
  [RadioRoles.ap]: 'Ponto de acesso (AP)',
  [RadioRoles.cpe]: 'Cliente rádio (CPE)',
  [RadioRoles.ptp]: 'Ponto a ponto (PTP)',
  [RadioModes.once]: 'Leitura pontual',
  [RadioModes.measure]: 'Medição curta',
  [RadioModes.live]: 'Tempo real',
  [RadioModes.index]: 'Índice de estações',
};

export function formatRadioMode(
  mode?: string | null,
  protocol?: string | null,
  reason = 'modo não coletado',
): string {
  const rawMode = (mode ?? '').trim();
  const rawProto = (protocol ?? '').trim();
  const label = rawMode
    ? MODE_LABELS[rawMode.toLowerCase()] || sanitizeToken(rawMode)
    : '';
  const proto = rawProto ? sanitizeToken(rawProto) : '';
  if (label && proto && label.toLowerCase() !== proto.toLowerCase()) {
    return `${label} · ${proto}`;
  }
  return label || proto || formatAbsent(reason);
}

const QUALITY_LABELS: Record<string, string> = {
  COMPLETE: 'Leitura completa',
  PARTIAL: 'Leitura parcial',
  UNAVAILABLE: 'Leitura indisponível',
  FAILED: 'Falha na leitura',
  GATED: 'Retida por proteção',
  IDLE: 'Aguardando leitura',
};

export function formatRadioQuality(quality?: string | null): string {
  const key = (quality ?? '').trim().toUpperCase();
  return QUALITY_LABELS[key] || 'Estado não classificado';
}

const LINK_ORIGIN_LABELS: Record<string, string> = {
  'sugerido-confirmado': 'Sugestão confirmada pelo operador',
  manual: 'Confirmado manualmente',
  sugerido: 'Sugestido pelo inventário',
};

export function formatLinkOrigin(origin?: string | null): string {
  const key = (origin ?? '').trim().toLowerCase();
  if (!key) return formatAbsent('origem não registrada');
  return LINK_ORIGIN_LABELS[key] || 'Vínculo registrado';
}

const TRUST_ORIGIN_LABELS: Record<string, string> = {
  'automatic-tofu': 'Aceita no primeiro acesso (TOFU)',
  tofu: 'Aceita no primeiro acesso (TOFU)',
  confirmed: 'Confirmada pelo operador',
  manual: 'Confirmada pelo operador',
};

export function formatTrustOrigin(origin?: string | null): string {
  const key = (origin ?? '').trim().toLowerCase();
  if (!key) return formatAbsent('chave não verificada');
  return TRUST_ORIGIN_LABELS[key] || 'Verificação registrada';
}

export function formatTickReason(reason?: string | null): {
  summary: string;
  action: string;
  technical?: string;
} {
  const raw = (reason ?? '').trim();
  if (!raw) {
    return {
      summary: 'Amostra parcial neste ciclo.',
      action:
        'Aguarde o próximo ciclo ou repita a leitura se o estado persistir.',
    };
  }
  const lower = raw.toLowerCase();
  if (lower.includes('tick-parse') || lower.includes('parse')) {
    return {
      summary: 'O equipamento respondeu com campos incompletos neste ciclo.',
      action: 'Aguarde o próximo ciclo ou repita a leitura manual.',
      technical: raw,
    };
  }
  if (lower.includes('deadline') || lower.includes('timeout')) {
    return {
      summary:
        'O equipamento demorou além do limite para responder neste ciclo.',
      action: 'Verifique a latência até o equipamento e tente novamente.',
      technical: raw,
    };
  }
  return {
    summary: 'Amostra parcial registrada neste ciclo.',
    action: 'Consulte o detalhe técnico abaixo caso precise investigar.',
    technical: raw,
  };
}

import {
  sanitizeToken,
  scrubSecrets,
  sanitizeVisibleText,
  TechnicalDetail,
} from './network-sanitize';

export { sanitizeToken, scrubSecrets, sanitizeVisibleText, TechnicalDetail };
