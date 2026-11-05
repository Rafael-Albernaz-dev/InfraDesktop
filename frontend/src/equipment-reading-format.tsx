import type { PPReadRow } from './presentation';
import type { Device } from './equipment-api';
import {
  formatDuration,
  formatRouterDuration,
  uptimeTitle,
} from './network-format';
export const id = () => crypto.randomUUID();

export const readingAge = (at?: string, now = Date.now()) =>
  at ? `há ${formatDuration((now - Date.parse(at)) / 1000)}` : '';
export const stamp = (d: Device) => readingAge(d.result?.observed_at);

export function latestSavedReading(d: Device) {
  const radioAt = Date.parse(d.radio_summary?.observed_at || '');
  const monitorAt = Date.parse(d.monitoring_at || ''),
    summaryAt = Date.parse(d.result?.observed_at || '');
  if (d.radio_summary && Number.isFinite(radioAt) &&
      (!Number.isFinite(monitorAt) || radioAt >= monitorAt) &&
      (!Number.isFinite(summaryAt) || radioAt >= summaryAt))
    return {
      at: d.radio_summary.observed_at,
      version: d.radio_summary.version,
      cpu: d.radio_summary.cpu,
      uptime: formatRouterDuration(d.radio_summary.uptime),
      exactUptime: d.radio_summary.uptime,
    };
  if (
    d.monitoring &&
    Number.isFinite(monitorAt) &&
    (!Number.isFinite(summaryAt) || monitorAt >= summaryAt)
  )
    return {
      at: d.monitoring_at!,
      version: d.monitoring.version,
      cpu: d.monitoring.cpu,
      uptime: d.monitoring.uptime.known
        ? formatDuration(d.monitoring.uptime.min / 1e9)
        : 'Não informado nesta leitura',
      exactUptime: uptimeTitle(d.monitoring.uptime),
    };
  return {
    at: d.result?.observed_at || '',
    version: d.result?.version,
    cpu: d.result?.cpu_percent,
    uptime: formatRouterDuration(d.result?.uptime),
    exactUptime: d.result?.uptime,
  };
}

export function formatRestarts(row?: PPReadRow) {
  return (
    `Reinícios 7 d: ${row?.used ?? 'Não informado nesta leitura'} · 30 d: ${row?.size ?? 'Não informado nesta leitura'}. ` +
    'Detectados entre leituras; reinícios múltiplos no mesmo intervalo contam como um.'
  );
}
