/** Shared pt-BR network presentation. Inputs remain raw for sorting and CSV. */
export type NetworkNumber = number | string | bigint | null | undefined;
export type NumberRange = { min: number; max: number };
const unavailable = 'Indisponível';
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
export function formatMetric(value: NetworkNumber, unit: string): string {
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
    return unavailable;
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
  const a = value.min / 1e6,
    b = value.max / 1e6;
  return `${formatter(digits(a)).format(Math.floor(a * 10 ** digits(a)) / 10 ** digits(a))}–${formatter(digits(b)).format(Math.ceil(b * 10 ** digits(b)) / 10 ** digits(b))} ms`;
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
