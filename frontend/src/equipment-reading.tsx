import { useLayoutEffect, useRef, useState } from 'react';
import { Button } from './desktop';
import { equipmentMessage } from './equipment-api';
import type { Device } from './equipment-api';
import {
  exactVolume,
  formatDate,
  formatDuration,
  formatMetric,
  formatRouterDuration,
  formatVolumePair,
  uptimeTitle,
} from './network-format';

export const id = () => crypto.randomUUID();

export const stamp = (d: Device) =>
  d.result?.observed_at ? formatDate(d.result.observed_at) : 'Não consultado';

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

const READING_TABS: [string, string][] = [
  ['summary', 'Resumo'],
  ['physical', 'Interfaces físicas'],
  ['vlan', 'VLANs'],
  ['vpn', 'VPNs e túneis'],
  ['other', 'Outras interfaces'],
];

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

function ReadingTabs({
  tab,
  pickTab,
  openPPPoE,
}: {
  tab: string;
  pickTab: (next: string) => void;
  openPPPoE?: () => void;
}) {
  return (
    <nav className="equipment-reading-tabs" aria-label="Áreas do equipamento">
      <div
        role="tablist"
        aria-label="Dados do equipamento"
        onKeyDown={(event) => {
          if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key))
            return;
          const controls = Array.from(
            event.currentTarget.querySelectorAll<HTMLButtonElement>(
              '[role=tab]',
            ),
          );
          const index = controls.indexOf(event.target as HTMLButtonElement);
          if (index < 0) return;
          const next =
            event.key === 'Home'
              ? 0
              : event.key === 'End'
                ? controls.length - 1
                : (index +
                    (event.key === 'ArrowRight' ? 1 : -1) +
                    controls.length) %
                  controls.length;
          event.preventDefault();
          controls[next]?.click();
          controls[next]?.focus();
        }}
      >
        {READING_TABS.map(([value, label]) => (
          <button
            type="button"
            key={value}
            role="tab"
            aria-selected={tab === value}
            onClick={() => pickTab(value)}
          >
            {label}
          </button>
        ))}
      </div>
      {openPPPoE && (
        <Button kind="primary" onClick={openPPPoE}>
          Abrir sessões PPPoE
        </Button>
      )}
    </nav>
  );
}

function ReadingSummary({
  device,
  heading,
}: {
  device: Device;
  heading: React.RefObject<HTMLHeadingElement | null>;
}) {
  const r = device.result!;
  const items: [string, string][] = [
    ['RouterOS', r.version || 'Não informado'],
    ['Placa / modelo', r.board || 'Não informado'],
    ['Tempo ligado', formatRouterDuration(r.uptime)],
    [
      'CPU',
      r.cpu_percent === undefined
        ? 'Não informada'
        : formatMetric(r.cpu_percent, '%'),
    ],
    [
      'Memória livre / total',
      formatVolumePair(r.free_memory_bytes, r.total_memory_bytes),
    ],
  ];
  return (
    <>
      <h2 ref={heading} tabIndex={-1}>
        Resumo
      </h2>
      <h3>{r.identity || 'Identidade não informada'}</h3>
      <div className="equipment-summary">
        {items.map(([label, value]) => (
          <div className="kv" key={label}>
            <span className="key">{label}</span>
            <span
              title={
                label === 'Memória livre / total'
                  ? `${exactVolume(r.free_memory_bytes)} / ${exactVolume(r.total_memory_bytes)}`
                  : label === 'Tempo ligado'
                    ? r.uptime
                    : undefined
              }
            >
              {value}
            </span>
          </div>
        ))}
      </div>
      <p className="muted">
        Última leitura válida: {stamp(device)} ·{' '}
        {r.quality === 'complete'
          ? 'Resumo completo'
          : 'Leitura com parte limitada ou indisponível'}
      </p>
      <details className="equipment-help">
        <summary>Origem da leitura</summary>
        <p className="fingerprint">{r.source}</p>
      </details>
    </>
  );
}

function ReadingInterfacePart({
  device,
  allRowsCount,
}: {
  device: Device;
  allRowsCount: number;
}) {
  const part = device.result!.interface_part;
  if (!part)
    return (
      <p className="muted">
        Listagem da leitura anterior; escopo e contagens não registrados.
      </p>
    );
  return (
    <>
      <p>
        {part.status === 'stale'
          ? 'Última listagem válida preservada'
          : part.status === 'failed'
            ? 'Listagem indisponível'
            : 'Interfaces estáticas'}
        {part.observed_at ? ` · ${formatDate(part.observed_at)}` : ''}
      </p>
      {['failed', 'stale'].includes(part.status) ? (
        <p className="inventory-notice">{equipmentMessage(part.code)}</p>
      ) : part.counts_known ? (
        <p className="muted">
          {allRowsCount} de {part.eligible} estáticas · {part.dynamic} dinâmicas
          fora desta listagem · {part.total} no destino.{' '}
          {part.status === 'limited'
            ? 'Listagem limitada por volume ou tamanho: até 128 estáticas.'
            : ''}
        </p>
      ) : (
        <p className="muted">Contagens não registradas nesta leitura.</p>
      )}
      {part.status === 'stale' && (
        <details>
          <summary>Origem da listagem preservada</summary>
          <p className="fingerprint">{part.source}</p>
        </details>
      )}
    </>
  );
}

function ReadingInterfacesTable({
  rows,
  page,
  showPage,
  table,
}: {
  rows: NonNullable<NonNullable<Device['result']>['interfaces']>;
  page: number;
  showPage: (next: number) => void;
  table: React.RefObject<HTMLDivElement | null>;
}) {
  return (
    <>
      {!!rows.length && (
        <div
          className="table-scroll"
          tabIndex={0}
          aria-label="Interfaces observadas"
          ref={table}
        >
          <table className="app-table">
            <thead>
              <tr>
                <th>Nome</th>
                <th>Tipo</th>
                <th>Estado</th>
              </tr>
            </thead>
            <tbody>
              {rows.slice(page * 16, (page + 1) * 16).map((i) => (
                <tr key={i.name}>
                  <td>{i.name}</td>
                  <td>{i.kind}</td>
                  <td>
                    {i.disabled ? 'Desativada' : i.running ? 'Ativa' : 'Parada'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {rows.length > 16 && (
        <div className="list-footer">
          <span>
            Página {page + 1} de {Math.ceil(rows.length / 16)}
          </span>
          <div>
            <Button disabled={!page} onClick={() => showPage(page - 1)}>
              Anterior
            </Button>
            <Button
              disabled={(page + 1) * 16 >= rows.length}
              onClick={() => showPage(page + 1)}
            >
              Próxima
            </Button>
          </div>
        </div>
      )}
    </>
  );
}

function ReadingInterfacesSection({
  device,
  separate,
  tab,
  page,
  showPage,
  table,
}: {
  device: Device;
  separate: boolean;
  tab: string;
  page: number;
  showPage: (next: number) => void;
  table: React.RefObject<HTMLDivElement | null>;
}) {
  const r = device.result!;
  const allRows = r.interfaces || [];
  const rows = separate
    ? allRows.filter((row) => classifyInterfaceKind(row.kind) === tab)
    : allRows;
  return (
    <>
      <h2>
        {separate
          ? READING_TABS.find(([value]) => value === tab)?.[1]
          : 'Interfaces'}
      </h2>
      {separate && (
        <p>
          {rows.length} nesta categoria da leitura armazenada. Logins de
          clientes ficam em Sessões PPPoE.
        </p>
      )}
      <ReadingInterfacePart device={device} allRowsCount={allRows.length} />
      <ReadingInterfacesTable
        rows={rows}
        page={page}
        showPage={showPage}
        table={table}
      />
      {!rows.length && r.interface_part?.status !== 'failed' && (
        <p className="muted">Nenhuma interface nesta listagem.</p>
      )}
    </>
  );
}

export function EquipmentReading({
  device,
  focus,
  separate = false,
  openPPPoE,
}: {
  device: Device;
  focus: boolean;
  separate?: boolean;
  openPPPoE?: () => void;
}) {
  const heading = useRef<HTMLHeadingElement>(null);
  const table = useRef<HTMLDivElement>(null);
  useLayoutEffect(() => {
    if (focus) heading.current?.focus({ preventScroll: true });
  }, [focus]);
  const [page, setPage] = useState(0);
  const [tab, setTab] = useState('summary');
  const showPage = (next: number) => {
    setPage(next);
    if (table.current) {
      table.current.scrollTop = 0;
      table.current.scrollIntoView({ block: 'start' });
      table.current.focus({ preventScroll: true });
    }
  };
  const pickTab = (next: string) => {
    setTab(next);
    setPage(0);
  };
  return (
    <section className="equipment-result" aria-label="Resultado da consulta">
      {separate && (
        <ReadingTabs tab={tab} pickTab={pickTab} openPPPoE={openPPPoE} />
      )}
      {(!separate || tab === 'summary') && (
        <ReadingSummary device={device} heading={heading} />
      )}
      {(!separate || tab !== 'summary') && (
        <ReadingInterfacesSection
          device={device}
          separate={separate}
          tab={tab}
          page={page}
          showPage={showPage}
          table={table}
        />
      )}
    </section>
  );
}
