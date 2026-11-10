import { KeyValues } from '../../shared/ui/KeyValues';
import type { RadioLink, RadioMetric, RadioReading } from '../radio/radio-api';

const missing = 'Não informado nesta leitura';
function metric(reading: RadioReading | undefined, name: string) {
  const value =
    reading?.compatible &&
    reading.metrics.find(
      (item) =>
        item.name === name &&
        ['complete', 'observed', 'initial'].includes(item.quality),
    );
  return value && Number.isFinite(value.value) ? value : undefined;
}
function metricText(value: RadioMetric | undefined) {
  if (!value) return missing;
  return `${value.value}${value.unit === '%' ? '' : ' '}${value.unit}`;
}
function rates(reading?: RadioReading) {
  const tx = metric(reading, 'rate_tx'),
    rx = metric(reading, 'rate_rx');
  if (!tx || !rx || tx.unit !== rx.unit) return missing;
  return `${tx.value}/${rx.value} ${tx.unit}`;
}
export function SessionLinkSummary({
  link,
  names,
  reading,
  open,
}: {
  link: RadioLink;
  names: Record<string, string>;
  reading?: RadioReading;
  open?: (id: string) => void;
}) {
  const device = (id: string) =>
    names[id] && open ? (
      <a
        href={'#ficha/' + encodeURIComponent(id)}
        onClick={(event) => {
          event.preventDefault();
          open(id);
        }}
      >
        {names[id]}
      </a>
    ) : (
      names[id] || missing
    );
  return (
    <KeyValues
      rows={[
        ['AP', device(link.ap_id)],
        ['CPE', device(link.cpe_id)],
        ['Sinal', metricText(metric(reading, 'signal'))],
        ['CCQ', metricText(metric(reading, 'ccq'))],
        ['Taxas (TX/RX)', rates(reading)],
      ]}
    />
  );
}
