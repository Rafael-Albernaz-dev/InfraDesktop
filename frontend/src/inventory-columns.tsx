import './inventory-pulse.css';
import type { Device, Credential } from './equipment-api';
import { equipmentType } from './equipment-layout';
import { latestSavedReading } from './equipment-reading-format';
import { formatDate, formatDuration, formatMetric } from './network-format';

type Context = { device: Device; now: number; credentials: Credential[] };
type Column = {
  id: string;
  label: string;
  cell: (c: Context) => React.ReactNode;
};
const metric = (v: number | undefined, unit: string) =>
  v === undefined ? '—' : formatMetric(v, unit);
const latest = (d: Device) => {
  const saved = latestSavedReading(d);
  return {
    saved,
    radio:
      saved.at && saved.at === d.radio_summary?.observed_at
        ? d.radio_summary
        : undefined,
    monitoring:
      saved.at && saved.at === d.monitoring_at ? d.monitoring : undefined,
    result:
      saved.at && saved.at === d.result?.observed_at ? d.result : undefined,
  };
};
function memory(d: Device) {
  const { radio, monitoring, result } = latest(d);
  if (radio?.memory !== undefined) return metric(radio.memory, '%');
  const free = monitoring?.free_memory ?? result?.free_memory_bytes;
  const total = monitoring?.total_memory ?? result?.total_memory_bytes;
  return total && free !== undefined && free <= total
    ? metric(((total - free) / total) * 100, '%')
    : '—';
}
export const columns: Column[] = [
  { id: 'status', label: 'Estado', cell: pulseCell },
  {
    id: 'role',
    label: 'Papel',
    cell: ({ device }) => equipmentType(device.role),
  },
  { id: 'block', label: 'Bloco', cell: ({ device }) => device.block || '—' },
  {
    id: 'model',
    label: 'Modelo',
    cell: ({ device }) =>
      device.result?.board || device.radio_summary?.model || '—',
  },
  { id: 'address', label: 'Endereço', cell: ({ device }) => device.address },
  {
    id: 'online',
    label: 'Clientes online',
    cell: ({ device }) =>
      device.pppoe?.online ?? device.radio_summary?.clients ?? '—',
  },
  {
    id: 'offline',
    label: 'Clientes offline',
    cell: ({ device }) => device.pppoe?.offline ?? '—',
  },
  {
    id: 'uptime',
    label: 'Uptime',
    cell: ({ device }) => {
      const s = latestSavedReading(device);
      return s.at ? <span title={s.exactUptime}>{s.uptime}</span> : '—';
    },
  },
  {
    id: 'cpu',
    label: 'CPU',
    cell: ({ device }) => metric(latestSavedReading(device).cpu, '%'),
  },
  {
    id: 'last_reading',
    label: 'Última leitura',
    cell: ({ device, now }) => {
      const at = latestSavedReading(device).at;
      return at ? (
        <span title={formatDate(at)}>
          há {formatDuration((now - Date.parse(at)) / 1000)}
        </span>
      ) : (
        'Ainda sem leitura'
      );
    },
  },
  { id: 'memory', label: 'Memória', cell: ({ device }) => memory(device) },
  {
    id: 'temperature',
    label: 'Temperatura',
    cell: ({ device }) => {
      const d = latest(device);
      return metric(d.radio?.temperature ?? d.monitoring?.temperature, '°C');
    },
  },
  {
    id: 'voltage',
    label: 'Tensão',
    cell: ({ device }) => metric(latest(device).radio?.voltage, 'V'),
  },
  {
    id: 'system',
    label: 'Sistema',
    cell: ({ device }) => latestSavedReading(device).version || '—',
  },
  {
    id: 'credential',
    label: 'Credencial',
    cell: ({ device, credentials }) =>
      credentials.find((c) => c.id === device.credential_id)?.alias || '—',
  },
  {
    id: 'query',
    label: 'Consulta',
    cell: ({ device }) => device.transport?.toUpperCase() || '—',
  },
];
export const defaultColumns = [
  'equipment',
  'status',
  'role',
  'block',
  'model',
  'address',
  'online',
  'offline',
  'uptime',
  'cpu',
  'last_reading',
];
export const columnById = (id: string) => columns.find((c) => c.id === id);

function pulseCell({ device }: Context) {
  const p = device.pulse;
  const age = p?.checked_at
    ? Math.max(0, (Date.now() - Date.parse(p.checked_at)) / 1000)
    : undefined;
  const state = device.active && age !== undefined && age <= 90 ? p?.state : '';
  const title =
    age === undefined ? 'Ainda sem ping' : `Ping há ${formatDuration(age)}`;
  return (
    <span className={state === 'OFFLINE' ? 'st-off' : undefined} title={title}>
      {state || '—'}
    </span>
  );
}
