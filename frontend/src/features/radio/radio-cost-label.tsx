import { tickFieldReason } from '../equipment/radio-tick-reason';
import type { RadioCost } from './radio-api';
import { formatMetric, sanitizeToken } from '../equipment/network-format';
export function radioCostLabel(cost: RadioCost) {
  const reason =
    tickFieldReason(cost.reason)?.summary || sanitizeToken(cost.reason);
  return (
    `${reason} Intervalo: ${formatMetric(cost.interval_seconds, 's')} ` +
    `· p95 ${formatMetric(cost.p95_seconds, 's')}`
  );
}
