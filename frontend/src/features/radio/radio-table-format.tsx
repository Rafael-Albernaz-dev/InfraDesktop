import type { RadioMetric } from './radio-api';
import { formatMetric } from '../equipment/network-format';
export function radioMetric(metrics: RadioMetric[], name: string) {
  return metrics.find((m) => m.name === name && m.quality !== 'unavailable');
}
export function radioRatePair(metrics: RadioMetric[]) {
  const tx = radioMetric(metrics, 'rate_tx'),
    rx = radioMetric(metrics, 'rate_rx');
  if (!tx || !rx || tx.unit !== rx.unit) return 'Não informado nesta leitura';
  return `${formatMetric(tx.value, tx.unit).replace(' ' + tx.unit, '')}/${formatMetric(rx.value, rx.unit)}`;
}
