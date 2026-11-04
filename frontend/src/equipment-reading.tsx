import { Fragment, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import type { Device } from './equipment-api';
import { equipmentMessage } from './equipment-api';
import {
  formatDate,
  formatMetric,
  formatRouterDuration,
  formatVolumePair,
  exactVolume,
} from './network-format';
import { stamp } from './equipment-reading-format';
import { DataBlock } from './ui/DataBlock';
import { DataTable } from './ui/DataTable';
import { Pager } from './ui/Pager';
import { Button } from './ui/Button';
export { id, stamp, latestSavedReading } from './equipment-reading-format';
function classifyInterfaceKind(kind: string): string {
  const value = kind.toLowerCase();
  if (['ether', 'wlan', 'wifi', 'w60g', 'lte'].includes(value))
    return 'physical';
  if (value === 'vlan') return 'vlan';
  if (
    /^(wireguard|pptp|l2tp|ovpn|sstp|eoip|gre|ipip|6to4|6in4|vti|vxlan)/.test(
      value,
    )
  )
    return 'vpn';
  return 'other';
}
function Summary({
  device,
  statistics,
}: {
  device: Device;
  statistics?: ReactNode;
}) {
  const r = device.result!;
  const pairs = [
    ['Identidade', r.identity],
    ['Modelo', r.board],
    ['Versão', r.version],
    ['Uptime', formatRouterDuration(r.uptime)],
    [
      'Origem da leitura',
      r.quality === 'complete'
        ? 'Resumo completo'
        : 'Leitura com parte limitada',
    ],
  ];
  return (
    <div className="grid">
      <DataBlock
        title="Identidade e sistema"
        source="SSH"
        age={stamp(device)}
        data={{
          state: 'ready',
          content: (
            <>
              <dl className="kv">
                {pairs.map(([key, value]) => (
                  <Fragment key={key}>
                    <dt>{key}</dt>
                    <dd>{value || 'Não informado nesta leitura'}</dd>
                  </Fragment>
                ))}
              </dl>
            </>
          ),
        }}
      />
      <DataBlock
        title="Recursos"
        source="SSH"
        age={stamp(device)}
        data={{
          state: 'ready',
          content: (
            <dl className="kv">
              <dt>CPU</dt>
              <dd>{formatMetric(r.cpu_percent, '%')}</dd>
              <dt>Memória livre / total</dt>
              <dd
                title={`${exactVolume(r.free_memory_bytes)} / ${exactVolume(r.total_memory_bytes)}`}
              >
                {formatVolumePair(r.free_memory_bytes, r.total_memory_bytes)}
              </dd>
            </dl>
          ),
        }}
      />
      {statistics}
    </div>
  );
}
function InterfaceMetadata({ device }: { device: Device }) {
  const part = device.result!.interface_part;
  if (!part)
    return (
      <details className="det">
        <summary>Origem da listagem</summary>
        <p>Listagem da leitura anterior; escopo e contagens não registrados.</p>
      </details>
    );
  return (
    <details className="det">
      <summary>Origem da listagem</summary>
      <p>
        {part.status === 'stale'
          ? 'Última listagem válida preservada'
          : 'Interfaces estáticas'}{' '}
        ·{' '}
        {part.observed_at
          ? formatDate(part.observed_at)
          : 'Não informado nesta leitura'}
      </p>
      {['failed', 'stale'].includes(part.status) ? (
        <p>{equipmentMessage(part.code)}</p>
      ) : part.counts_known ? (
        <p>
          {device.result?.interfaces?.length || 0} de {part.eligible} estáticas
          · {part.dynamic} dinâmicas fora desta listagem · {part.total} no
          destino.
          {part.status === 'limited' &&
            ' Listagem limitada por volume ou tamanho: até 128 estáticas.'}
        </p>
      ) : (
        <p>Contagens não registradas nesta leitura.</p>
      )}
      {part.status === 'stale' && <p>{part.source}</p>}
    </details>
  );
}
function Interfaces({ device }: { device: Device }) {
  const [filter, setFilter] = useState('physical'),
    [page, setPage] = useState(0);
  const surface = useRef<HTMLDivElement>(null);
  const rows = (device.result?.interfaces || []).filter(
    (row) => classifyInterfaceKind(row.kind) === filter,
  );
  const showPage = (next: number) => {
    setPage(next);
    surface.current?.focus({ preventScroll: true });
    surface.current?.scrollIntoView?.({ block: 'start' });
  };
  const content = (
    <div ref={surface} tabIndex={0} aria-label="Interfaces observadas">
      <DataTable>
        <thead>
          <tr>
            <th>Interface</th>
            <th>Tipo</th>
            <th>Estado</th>
          </tr>
        </thead>
        <tbody>
          {rows.slice(page * 16, (page + 1) * 16).map((row) => (
            <tr key={row.name}>
              <td>{row.name}</td>
              <td>{row.kind}</td>
              <td>
                {row.disabled ? 'Desativada' : row.running ? 'Ativa' : 'Parada'}
              </td>
            </tr>
          ))}
        </tbody>
      </DataTable>
      <Pager
        page={page + 1}
        total={Math.max(1, Math.ceil(rows.length / 16))}
        first={() => showPage(0)}
        next={() => showPage(page + 1)}
      />
      <InterfaceMetadata device={device} />
    </div>
  );
  const failed = device.result?.interface_part?.status === 'failed';
  return (
    <>
      <div className="chips">
        {[
          ['physical', 'Físicas'],
          ['vlan', 'VLANs'],
          ['vpn', 'VPNs e túneis'],
          ['other', 'Outras'],
        ].map(([value, label]) => (
          <Button
            key={value}
            className={filter === value ? 'on' : ''}
            aria-pressed={filter === value}
            onClick={() => {
              setFilter(value);
              setPage(0);
            }}
          >
            {label}
          </Button>
        ))}
      </div>
      <DataBlock
        title="Interfaces"
        source="SSH"
        age={stamp(device)}
        data={
          rows.length
            ? { state: 'ready', content }
            : failed
              ? {
                  state: 'ready',
                  content: <InterfaceMetadata device={device} />,
                }
              : { state: 'empty', message: 'Nenhuma interface encontrada' }
        }
      />
    </>
  );
}
export function EquipmentReading({
  device,
  section = 'summary',
  statistics,
}: {
  device: Device;
  focus: boolean;
  section?: 'summary' | 'interfaces';
  separate?: boolean;
  openPPPoE?: () => void;
  statistics?: ReactNode;
}) {
  return section === 'summary' ? (
    <Summary device={device} statistics={statistics} />
  ) : (
    <Interfaces device={device} />
  );
}
