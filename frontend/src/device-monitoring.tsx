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

function SessionCurveSvg({ points }: { points: DeviceMonitoringPoint[] }) {
  const ordered = [...points].reverse();
  const maximum = Math.max(
    1,
    ...ordered.flatMap((p) =>
      p.online_max === undefined ? [] : [p.online_max],
    ),
  );
  const first = Math.min(...ordered.map((p) => Date.parse(p.at)));
  const last = Math.max(...ordered.map((p) => Date.parse(p.at)));
  const x = (p: DeviceMonitoringPoint) =>
    ((Date.parse(p.at) - first) / Math.max(1, last - first)) * 600;
  const paths: string[] = [];
  let path = '';
  for (let i = 0; i < ordered.length; i++) {
    const p = ordered[i];
    const prev = ordered[i - 1];
    const gapTime = prev ? Date.parse(p.at) - Date.parse(prev.at) : 0;
    const maxRes = prev ? Math.max(p.resolution, prev.resolution) * 1000 : 0;
    if (p.gap || p.online === undefined || (prev && gapTime > maxRes)) {
      if (path) paths.push(path);
      path = '';
      continue;
    }
    path +=
      (path ? ' L ' : 'M ') +
      x(p).toFixed(1) +
      ' ' +
      (135 - (p.online / maximum) * 125).toFixed(1);
  }
  if (path) paths.push(path);
  return (
    <svg
      viewBox="0 0 600 150"
      role="img"
      aria-label="Curva de sessões online observadas"
      style={{ width: '100%', maxHeight: 180 }}
    >
      <title>
        Sessões online; lacunas não são zero. Página atual da série.
      </title>
      {paths.map((d, i) => (
        <path
          key={i}
          d={d}
          fill="none"
          stroke="var(--accent, #466bd2)"
          strokeWidth="2"
        />
      ))}
      {ordered.map((p, i) => {
        if (p.online === undefined) return null;
        const cy = 135 - (p.online / maximum) * 125;
        const fill = p.drop
          ? 'var(--danger, #b94b4b)'
          : 'var(--accent, #466bd2)';
        return <circle key={i} cx={x(p)} cy={cy} r={3} fill={fill} />;
      })}
    </svg>
  );
}

function SessionPointsTable({ points }: { points: DeviceMonitoringPoint[] }) {
  return (
    <div
      className="table-scroll"
      tabIndex={0}
      aria-label="Pontos de sessões online"
    >
      <table className="app-table">
        <thead>
          <tr>
            <th>Período</th>
            <th>Online mínimo / máximo / último</th>
            <th>Origem das sessões</th>
            <th>Qualidade</th>
          </tr>
        </thead>
        <tbody>
          {points.map((p, i) => {
            const qualityLabels = [
              p.gap ? 'Lacuna de continuidade' : '',
              p.drop ? 'Queda brusca observada' : '',
              !p.gap && !p.drop ? 'Leitura compatível' : '',
            ]
              .filter(Boolean)
              .join(' · ');
            return (
              <tr key={i}>
                <td>
                  {formatDate(p.at)} ·{' '}
                  {p.resolution === 3600 ? 'hora' : 'minuto'}
                </td>
                <td>
                  {p.online_min ?? 'Indisponível'} /{' '}
                  {p.online_max ?? 'Indisponível'} /{' '}
                  {p.online ?? 'Indisponível'}
                </td>
                <td>
                  {p.session_at
                    ? formatDate(p.session_at)
                    : 'Sem leitura de sessões neste ponto'}
                </td>
                <td>
                  {p.online === undefined
                    ? 'Sem leitura de sessões'
                    : qualityLabels}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function SessionMonitoringCurve({
  points,
}: {
  points: DeviceMonitoringPoint[];
}) {
  return (
    <>
      <SessionCurveSvg points={points} />
      <SessionPointsTable points={points} />
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
      <div className="table-scroll" tabIndex={0}>
        <table className="app-table">
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
        </table>
      </div>
    </>
  );
}

function MonitoringFlappingItem({ r }: { r: PPReadResult['rows'][number] }) {
  const pAt = r.at ? formatDate(r.at) : 'Indisponível';
  const pVal = r.value ? formatDate(r.value) : 'Indisponível';
  return (
    <p>
      {r.login}: {r.used} transições · primeira {pAt} · última {pVal}
    </p>
  );
}

function MonitoringVersionItem({ r }: { r: PPReadResult['rows'][number] }) {
  const vVal = r.value || 'Indisponível';
  const vAt = r.at ? formatDate(r.at) : 'Data indisponível';
  const vPk =
    r.event === 'complete' ? (r.used ?? 'Indisponível') : 'Indisponível';
  const vSrc = r.source_at ? formatDate(r.source_at) : 'Indisponível';
  return (
    <p>
      {r.name}: {vVal} · {vAt} · pacotes {vPk} · origem {vSrc}
    </p>
  );
}

function MonitoringHygieneItem({ r }: { r: PPReadResult['rows'][number] }) {
  const hAt = r.at ? formatDate(r.at) : 'Indisponível';
  return (
    <p>
      {r.name}: {r.used ?? 'Indisponível'} · {r.size ?? 'Indisponível'} ONLINE
      sem evidência disponível · origem {hAt}
    </p>
  );
}

function MonitoringRowItem({
  r,
  scope,
}: {
  r: PPReadResult['rows'][number];
  scope: string;
}) {
  if (r.kind === 'flapping') return <MonitoringFlappingItem r={r} />;
  if (r.kind === 'version') return <MonitoringVersionItem r={r} />;
  if (r.kind === 'hygiene') return <MonitoringHygieneItem r={r} />;
  if (r.name === 'retention') {
    return <p>Parte da série foi removida pelo limite de armazenamento.</p>;
  }
  if (scope === 'duplicates') {
    return (
      <p>
        {r.name}: {r.value || 'não comparável (leitura indisponível)'}
      </p>
    );
  }
  return <p>{r.name}: indisponível nesta fonte.</p>;
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
  } = p;
  useEffect(() => {
    let alive = true;
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

function MonitoringScopeBody(p: {
  scope: string;
  rows: PPReadResult['rows'];
  comparison: { version?: boolean; packages?: boolean; count?: number };
  displayNow: number;
}) {
  const { scope, rows, comparison, displayNow } = p;
  if (scope === 'series') {
    return (
      <>
        <p>
          Pontos por minuto durante 24 h; depois, mínimo, máximo e último por
          hora até 7 dias. Quebras de continuidade não comprovam queda. Queda
          brusca: pelo menos 5 sessões e 20% entre leituras compatíveis.
        </p>
        <SessionMonitoringCurve
          points={rows.flatMap((r) => (r.point ? [r.point] : []))}
        />
      </>
    );
  }
  if (scope === 'flapping') {
    return (
      <p>
        Transições ONLINE ↔ OFFLINE observadas nas últimas 24 h. Leituras
        duplicadas, falhas, reboot e contexto incompatível não contam como
        oscilação.
      </p>
    );
  }
  if (scope === 'duplicates') {
    return <MonitoringDuplicatesTable rows={rows} displayNow={displayNow} />;
  }
  if (scope === 'versions') {
    return <MonitoringVersionsView comparison={comparison} />;
  }
  return null;
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
  const displayNow = useDisplayNow();
  const [scope, setScope] = useState('series');
  const [cursor, setCursor] = useState('');
  const [next, setNext] = useState('');
  const [rows, setRows] = useState<PPReadResult['rows']>([]);
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading');
  const [error, setError] = useState('');
  const [refresh, bump] = useState(0);
  const [comparison, setComparison] = useState<{
    version?: boolean;
    packages?: boolean;
    count?: number;
  }>({});
  const completed = useCallback(() => {
    setCursor('');
    bump((n) => n + 1);
  }, []);
  useDeviceMonitoringLoad({
    profile,
    activation,
    deviceId: device.id,
    scope,
    cursor,
    refresh,
    setState,
    setError,
    setComparison,
    setRows,
    setNext,
  });
  function change(val: string) {
    setScope(val);
    setCursor('');
    setNext('');
    setRows([]);
    setState('loading');
  }
  return (
    <section
      className="pppoe-diagnostics"
      aria-label="Monitoramento operacional"
    >
      <h2>Monitoramento operacional</h2>
      <p>
        Atualização sob demanda. Ler a série armazenada não consulta o
        equipamento. Dados ausentes continuam indisponíveis.
      </p>
      <PPPoEReadPanel
        profile={profile}
        activation={activation}
        device={device}
        observation=""
        rows={[]}
        monitoring
        onRead={completed}
      />
      <nav role="tablist" aria-label="Dados locais do monitoramento">
        {MON_TABS.map(([val, label]) => (
          <button
            key={val}
            role="tab"
            aria-selected={scope === val}
            onClick={() => change(val)}
          >
            {label}
          </button>
        ))}
      </nav>
      {state === 'loading' && <p role="status">Lendo dados armazenados…</p>}
      {state === 'error' && (
        <>
          <p role="alert">{error}</p>
          <button
            onClick={() => {
              setCursor('');
              setRows([]);
              setNext('');
              setState('loading');
              bump((n) => n + 1);
            }}
          >
            Reabrir série local
          </button>
        </>
      )}
      {state === 'ready' && (
        <>
          <MonitoringScopeBody
            scope={scope}
            rows={rows}
            comparison={comparison}
            displayNow={displayNow}
          />
          {!rows.length && <p>{emptyMonitoringNotice(scope)}</p>}
          {rows
            .filter((r) => !r.point && !r.duplicate)
            .map((r, i) => (
              <div key={i}>
                <MonitoringRowItem r={r} scope={scope} />
              </div>
            ))}
          {next && (
            <button
              onClick={() => {
                setCursor(next);
                setRows([]);
                setState('loading');
              }}
            >
              Próxima página dos dados locais
            </button>
          )}
          {cursor && (
            <button
              onClick={() => {
                setCursor('');
                setRows([]);
                setState('loading');
              }}
            >
              Início dos dados locais
            </button>
          )}
        </>
      )}
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
