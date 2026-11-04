import { hygieneDataState } from './hygiene-data-state';
import { SessionPointsTable } from './monitoring-points';
import { DataBlock } from './ui/DataBlock';
import type { DataState } from './ui/DataBlock';
import { KeyValues } from './ui/KeyValues';
import { readingAge } from './equipment-reading-format';
import { Button } from './ui/Button';
import { DataTable } from './ui/DataTable';
import { Chart } from './ui/Chart';
import { useCallback, useEffect, useState } from 'react';
import type { Device } from './equipment-api';
import { equipmentMessage } from './equipment-api';
import { useDisplayNow } from './equipment-snapshot';
import { formatDate, formatDuration, uptimeTitle } from './network-format';
import { formatPPUptime } from './pppoe-format';
import type { DeviceMonitoringPoint, PPReadResult } from './presentation';
import { deviceMonitoringAPI as monitoringAPI } from './presentation';
import { PPPoEReadPanel } from './pppoe-diagnostics';
export { DeviceOperationalTables } from './device-operational';

function SessionCurve({ points }: { points: DeviceMonitoringPoint[] }) {
  const ordered = [...points].reverse();
  const values = ordered.map((p, i) => {
    const prev = ordered[i - 1];
    const missingInterval =
      prev &&
      Date.parse(p.at) - Date.parse(prev.at) >
        Math.max(p.resolution, prev.resolution) * 1000;
    return p.gap || missingInterval ? null : (p.online ?? null);
  });
  return (
    <Chart
      x={ordered.map((p) => Date.parse(p.at) / 1000)}
      series={[{ label: 'Sessões online', token: '--chart-down', values }]}
      format={(value) => (value == null ? '—' : String(Math.round(value)))}
      formatX={(value) =>
        new Date(value * 1000).toLocaleTimeString('pt-BR', {
          hour: '2-digit',
          minute: '2-digit',
        })
      }
      xSteps={[60, 300, 900, 3600, 10800, 21600, 43200, 86400]}
      markers={ordered.flatMap((p, i) => (p.drop ? [i] : []))}
      label="Curva de sessões online observadas"
    />
  );
}

function SessionMonitoringCurve({
  points,
}: {
  points: DeviceMonitoringPoint[];
}) {
  const valid = points.filter((p) => !p.gap && p.online !== undefined);
  const values = valid.map((p) => p.online!);
  return (
    <>
      <SessionCurve points={points} />
      <p className="muted">
        Online mínimo / máximo / último:{' '}
        {values.length ? Math.min(...values) : '—'} /{' '}
        {values.length ? Math.max(...values) : '—'} / {values[0] ?? '—'} ·
        lacunas não são zero. Ponto vermelho: queda brusca.
      </p>
      <details className="det">
        <summary>Pontos da curva</summary>
        <SessionPointsTable points={points} />
      </details>
    </>
  );
}
function MonitoringDuplicatesTable({
  rows,
  displayNow,
}: {
  rows: NonNullable<PPReadResult['rows']>;
  displayNow: number;
}) {
  return (
    <>
      <p>
        Última leitura deste concentrador e dos demais da empresa: ambas com
        idade de até 15 min e janelas de coleta sobrepostas. Sem essas
        condições: não comparável (leituras distantes).
      </p>
      <div className="table-region" tabIndex={0}>
        <DataTable className="data-table">
          <thead>
            <tr>
              <th>Login</th>
              <th>Concentrador A · IP / caller-id / uptime / idade</th>
              <th>Concentrador B · IP / caller-id / uptime / idade</th>
            </tr>
          </thead>
          <tbody>
            {rows
              .filter((r) => r.duplicate)
              .map((r, i) => (
                <tr key={i}>
                  <td>{r.login}</td>
                  {[r.duplicate!.a, r.duplicate!.b].map((peer, j) => {
                    const durText = formatDuration(
                      (displayNow - Date.parse(peer.at)) / 1000,
                    );
                    const addrText = peer.address || 'IP indisponível';
                    const callerText =
                      peer.caller_id || 'Caller ID indisponível';
                    return (
                      <td key={j} title={uptimeTitle(peer.uptime)}>
                        {peer.name} ({peer.endpoint}) · {addrText} ·{' '}
                        {callerText} · {formatPPUptime(peer.uptime)} · leitura
                        há {durText}
                      </td>
                    );
                  })}
                </tr>
              ))}
          </tbody>
        </DataTable>
      </div>
    </>
  );
}

function MonitoringHygieneItem({ r }: { r: PPReadResult['rows'][number] }) {
  const hAt = r.at ? formatDate(r.at) : 'Não informado nesta leitura';
  return (
    <p>
      {r.name}: {r.used ?? 'Não informado nesta leitura'} ·{' '}
      {r.size ?? 'Não informado nesta leitura'} ONLINE sem evidência disponível
      · origem {hAt}
    </p>
  );
}

function useDeviceMonitoringLoad(p: {
  profile: string;
  activation: string;
  deviceId: string;
  scope: string;
  cursor: string;
  refresh: number;
  setState: (s: 'loading' | 'ready' | 'error') => void;
  setError: (e: string) => void;
  setComparison: (c: {
    version?: boolean;
    packages?: boolean;
    count?: number;
  }) => void;
  setRows: (r: PPReadResult['rows']) => void;
  setNext: (n: string) => void;
  setAt: (at: string) => void;
}) {
  const {
    profile,
    activation,
    deviceId,
    scope,
    cursor,
    refresh,
    setState,
    setError,
    setComparison,
    setRows,
    setNext,
    setAt,
  } = p;
  useEffect(() => {
    let alive = true;
    setState('loading');
    void (async () => {
      const r = await monitoringAPI.current(
        profile,
        activation,
        deviceId,
        scope,
        cursor,
      );
      if (!alive) return;
      if (
        r.profile_id !== profile ||
        r.activation_id !== activation ||
        r.code !== 'OK' ||
        !r.data?.read
      ) {
        setState('error');
        setError(
          r.code === 'CURSOR_INVALID'
            ? 'Os dados mudaram ou a página expirou. Reabra a série local.'
            : equipmentMessage(r.code),
        );
        return;
      }
      setComparison({
        version: r.data.read.version_divergence,
        packages: r.data.read.package_divergence,
        count: r.data.read.compared_devices,
      });
      setAt(r.data.read.ended_at);
      setRows(r.data.read.rows);
      setNext(r.data.next_cursor || '');
      setState('ready');
      setError('');
    })().catch(() => {
      if (alive) {
        setState('error');
        setError('Série local indisponível.');
      }
    });
    return () => {
      alive = false;
    };
  }, [
    profile,
    activation,
    deviceId,
    scope,
    cursor,
    refresh,
    setState,
    setError,
    setComparison,
    setRows,
    setNext,
    setAt,
  ]);
}

const MON_TABS = [
  ['series', 'Curva de sessões'],
  ['flapping', 'Oscilação'],
  ['versions', 'Versões'],
  ['hygiene', 'Higiene de secrets'],
  ['duplicates', 'Entre concentradores'],
];

function MonitoringVersionsView({
  comparison,
}: {
  comparison: { version?: boolean; packages?: boolean; count?: number };
}) {
  const vText = comparison.version
    ? 'Divergência de versões nas leituras armazenadas.'
    : 'Versões das últimas leituras disponíveis.';
  const pText =
    comparison.packages === undefined
      ? 'Comparação de pacotes indisponível: menos de dois conjuntos completos.'
      : comparison.packages
        ? 'Divergência de pacotes nos conjuntos completos.'
        : 'Pacotes iguais nos conjuntos completos comparados.';
  return (
    <p>
      {vText} Equipamentos sem leitura não participam da comparação. {pText}{' '}
      Comparação considera todos os {comparison.count ?? 0} equipamentos com
      leitura, inclusive outras páginas.
    </p>
  );
}
function emptyMonitoringNotice(scope: string): string {
  if (scope === 'series')
    return 'Nenhum ponto armazenado. Atualize as sessões ou o monitoramento.';
  if (scope === 'flapping')
    return 'Nenhuma transição registrada na janela disponível.';
  return 'Nenhuma leitura disponível para este grupo.';
}

function MonitoringCardBody(p: {
  scope: string;
  rows: PPReadResult['rows'];
  comparison: { version?: boolean; packages?: boolean; count?: number };
  displayNow: number;
}) {
  if (p.scope === 'series')
    return (
      <SessionMonitoringCurve
        points={p.rows.flatMap((r) => (r.point ? [r.point] : []))}
      />
    );
  if (p.scope === 'flapping')
    return (
      <DataTable>
        <thead>
          <tr>
            <th>Usuário</th>
            <th>Transições 24 h</th>
          </tr>
        </thead>
        <tbody>
          {p.rows
            .filter((r) => r.kind === 'flapping')
            .map((r, i) => (
              <tr key={i}>
                <td title={`${r.at || ''} → ${r.value || ''}`}>{r.login}</td>
                <td>{r.used}</td>
              </tr>
            ))}
        </tbody>
      </DataTable>
    );
  if (p.scope === 'versions') {
    const versions = [
      ...new Set(
        p.rows
          .filter((r) => r.kind === 'version')
          .map((r) => r.value || 'Não informado nesta leitura'),
      ),
    ];
    return (
      <>
        <KeyValues
          rows={versions.map((version) => [
            version,
            <span
              key={version}
              title={p.rows
                .map(
                  (r) =>
                    `${r.name}: ${r.value} · ${r.at} · pacotes ${r.used ?? 'Não informado nesta leitura'} · origem ${r.source_at || ''}`,
                )
                .join(' / ')}
            >
              {p.rows.filter((r) => r.value === version).length} equipamento
              {p.rows.filter((r) => r.value === version).length === 1
                ? ''
                : 's'}
            </span>,
          ])}
        />
        {(p.comparison.version || p.comparison.packages) && (
          <MonitoringVersionsView comparison={p.comparison} />
        )}
      </>
    );
  }
  if (p.scope === 'hygiene')
    return (
      <>
        {p.rows
          .filter((r) => r.kind === 'hygiene')
          .map((r, i) => (
            <MonitoringHygieneItem key={i} r={r} />
          ))}
      </>
    );
  return <MonitoringDuplicatesTable rows={p.rows} displayNow={p.displayNow} />;
}
function MonitoringScopeCard(p: {
  profile: string;
  activation: string;
  device: Device;
  scope: string;
  title: string;
  refresh: number;
}) {
  const displayNow = useDisplayNow();
  const [cursor, setCursor] = useState(''),
    [next, setNext] = useState(''),
    [rows, setRows] = useState<PPReadResult['rows']>([]);
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading'),
    [error, setError] = useState(''),
    [at, setAt] = useState('');
  const [local, bump] = useState(0),
    [comparison, setComparison] = useState<{
      version?: boolean;
      packages?: boolean;
      count?: number;
    }>({});
  const [seenRefresh, setSeenRefresh] = useState(p.refresh);
  if (seenRefresh !== p.refresh) {
    setSeenRefresh(p.refresh);
    setCursor('');
  }
  useDeviceMonitoringLoad({
    profile: p.profile,
    activation: p.activation,
    deviceId: p.device.id,
    scope: p.scope,
    cursor,
    refresh: p.refresh + local,
    setState,
    setError,
    setComparison,
    setRows,
    setNext,
    setAt,
  });
  const retry = () => {
    setCursor('');
    bump((n) => n + 1);
  };
  const content = (
    <>
      {!rows.length && <p>{emptyMonitoringNotice(p.scope)}</p>}
      <MonitoringCardBody
        {...p}
        rows={rows}
        comparison={comparison}
        displayNow={displayNow}
      />
      {next && (
        <Button onClick={() => setCursor(next)}>
          Próxima página dos dados locais
        </Button>
      )}
      {cursor && (
        <Button onClick={() => setCursor('')}>Início dos dados locais</Button>
      )}
      {rows
        .filter((r) => r.kind === 'availability')
        .map((r, i) => (
          <p key={i}>
            {r.name === 'retention'
              ? 'Parte da série foi removida pelo limite de armazenamento.'
              : `${r.name}: ${r.value || 'Não informado nesta leitura'}`}
          </p>
        ))}
    </>
  );
  let data: DataState = { state: 'ready', content };
  if (state === 'loading')
    data = rows.length
      ? { state: 'refreshing', content }
      : { state: 'loading' };
  else if (state === 'error')
    data = {
      state: 'failed',
      reason: error,
      technical: 'Leitura local do monitoramento',
      retry,
      previous: rows.length ? content : undefined,
    };
  else if (p.scope === 'hygiene')
    data = hygieneDataState(rows, next, retry) || data;
  else if (!rows.length && !next)
    data = {
      state: 'empty',
      message:
        p.scope === 'duplicates'
          ? 'Nenhuma duplicidade encontrada'
          : emptyMonitoringNotice(p.scope),
    };
  return (
    <DataBlock
      title={p.title}
      className={p.scope === 'hygiene' ? 'monitoring-hygiene' : undefined}
      source="local"
      age={readingAge(at)}
      data={data}
    />
  );
}
function DeviceMonitoringContent({
  profile,
  activation,
  device,
}: {
  profile: string;
  activation: string;
  device: Device;
}) {
  const [refresh, bump] = useState(0);
  const completed = useCallback(() => bump((n) => n + 1), []);
  return (
    <section aria-label="Monitoramento operacional">
      <div className="bar">
        <span className="muted">
          Atualização sob demanda. Ler a série salva não consulta o equipamento.
        </span>
        <span className="grow" />
        <PPPoEReadPanel
          profile={profile}
          activation={activation}
          device={device}
          observation=""
          rows={[]}
          monitoring
          summaryOnly
          actionsOnly
          onRead={completed}
        />
        <Button onClick={completed}>Reabrir série local</Button>
      </div>
      <MonitoringScopeCard
        profile={profile}
        activation={activation}
        device={device}
        scope="series"
        title="Curva de sessões"
        refresh={refresh}
      />
      <div className="sp10" />
      <div className="grid monitoring-grid">
        {MON_TABS.slice(1).map(([scope, title]) => (
          <MonitoringScopeCard
            key={scope}
            profile={profile}
            activation={activation}
            device={device}
            scope={scope}
            title={title}
            refresh={refresh}
          />
        ))}
      </div>
    </section>
  );
}

export function DeviceMonitoring({
  profile,
  activation,
  device,
}: {
  profile: string;
  activation: string;
  device: Device;
}) {
  const monKey = [profile, activation, device.id, device.revision].join('|');
  return (
    <DeviceMonitoringContent
      key={monKey}
      profile={profile}
      activation={activation}
      device={device}
    />
  );
}
