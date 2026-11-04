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
  const monitorAt = Date.parse(d.monitoring_at || ''),
    summaryAt = Date.parse(d.result?.observed_at || '');
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
        : 'Indisponível',
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
