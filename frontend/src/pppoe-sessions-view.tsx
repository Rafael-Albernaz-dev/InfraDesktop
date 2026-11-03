import { equipmentMessage } from './equipment-api';
import {
  exactRate,
  formatDate,
  formatRate,
  uptimeTitle,
} from './network-format';
import { ContextCounts, FRESH_QUERY } from './pppoe-context';
import { formatPPUptime } from './pppoe-format';
import type { PPObservation, PPQuery, PPRow } from './presentation';
import { pppoeAPI as api } from './presentation';

function runPPExport(
  kind: 'sanitized' | 'operational',
  p: {
    profile: string;
    activation: string;
    deviceId: string;
    observationId: string;
    base: string;
    tab: string;
    setBusy: (b: boolean) => void;
    setNotice: (s: string) => void;
    setError: (s: string) => void;
    lifeRef: React.MutableRefObject<{ generation: number }>;
    exportFocusRef: React.MutableRefObject<'sanitized' | 'operational' | ''>;
  },
) {
  p.exportFocusRef.current = kind;
  p.setNotice('');
  p.setError('');
  const g = p.lifeRef.current.generation;
  p.setBusy(true);
  const task =
    kind === 'sanitized'
      ? api.export(
          p.profile,
          p.activation,
          p.deviceId,
          p.observationId,
          p.tab === 'compare' ? p.base : '',
        )
      : api.exportOperational(
          p.profile,
          p.activation,
          p.deviceId,
          p.observationId,
        );
  const okMsg =
    kind === 'sanitized'
      ? 'Arquivo sanitizado salvo.'
      : 'Arquivo CSV operacional salvo.';
  void task
    .then((code) => {
      if (g === p.lifeRef.current.generation) {
        p.setNotice(
          code === 'OK'
            ? okMsg
            : code === 'EXPORT_CANCELLED'
              ? ''
              : equipmentMessage(code),
        );
      }
    })
    .catch(() => {
      if (g === p.lifeRef.current.generation)
        p.setError('Falha na exportação.');
    })
    .finally(() => {
      if (g === p.lifeRef.current.generation) p.setBusy(false);
    });
}

export function PPPoEObsActions(p: {
  profile: string;
  activation: string;
  deviceId: string;
  current: PPObservation;
  tab: string;
  base: string;
  disabled: boolean;
  busy: boolean;
  setBusy: (b: boolean) => void;
  setNotice: (s: string) => void;
  setError: (s: string) => void;
  lifeRef: React.MutableRefObject<{ generation: number }>;
  exportFocusRef: React.MutableRefObject<'sanitized' | 'operational' | ''>;
  exportControlRef: React.RefObject<HTMLButtonElement | null>;
  csvExportControlRef: React.RefObject<HTMLButtonElement | null>;
  save: () => void;
}) {
  const {
    current,
    tab,
    disabled,
    busy,
    exportControlRef,
    csvExportControlRef,
  } = p;
  const onExport = (kind: 'sanitized' | 'operational') =>
    runPPExport(kind, {
      profile: p.profile,
      activation: p.activation,
      deviceId: p.deviceId,
      observationId: current.observation_id,
      base: p.base,
      tab: p.tab,
      setBusy: p.setBusy,
      setNotice: p.setNotice,
      setError: p.setError,
      lifeRef: p.lifeRef,
      exportFocusRef: p.exportFocusRef,
    });
  return (
    <>
      <p>
        Leitura de {formatDate(current.provenance.ended_at)} · {current.count}{' '}
        registros
      </p>
      {current.context_omitted && (
        <p role="status">
          Contexto não incluído por limite. As sessões e seus estados foram
          preservados.
        </p>
      )}
      {tab === 'sessions' && !current.context_omitted && (
        <ContextCounts
          key={current.observation_id}
          profile={p.profile}
          activation={p.activation}
          device={p.deviceId}
          observation={current.observation_id}
        />
      )}
      <div className="pppoe-actions">
        <button disabled={disabled} onClick={p.save}>
          Salvar referência desta leitura
        </button>
        <button
          ref={exportControlRef}
          disabled={busy}
          onClick={() => onExport('sanitized')}
        >
          Exportar JSON sanitizado
        </button>
        <button
          ref={csvExportControlRef}
          disabled={busy}
          onClick={() => onExport('operational')}
        >
          Exportar CSV operacional
        </button>
        <p>O CSV contém dados identificáveis de assinantes.</p>
      </div>
    </>
  );
}

function SortAndCheckFilters(p: {
  query: PPQuery;
  change: (q: Partial<PPQuery>) => void;
  showPlan: boolean;
  setShowPlan: (v: boolean) => void;
}) {
  const { query, change, showPlan, setShowPlan } = p;
  return (
    <>
      <label>
        Ordenar
        <select
          value={query.sort}
          onChange={(e) => change({ sort: e.target.value })}
        >
          <option value="login">Login</option>
          <option value="address">IP</option>
          <option value="uptime">Tempo conectado</option>
        </select>
      </label>
      <label>
        <input
          type="checkbox"
          checked={query.desc}
          onChange={(e) => change({ desc: e.target.checked })}
        />
        Decrescente
      </label>
      <label>
        <input
          type="checkbox"
          checked={query.duplicates_only}
          onChange={(e) => change({ duplicates_only: e.target.checked })}
        />
        Somente logins duplicados
      </label>
      <label>
        <input
          type="checkbox"
          checked={showPlan}
          onChange={(e) => setShowPlan(e.target.checked)}
        />
        Mostrar plano/limite
      </label>
    </>
  );
}

export function PPPoESessionFilters(p: {
  query: PPQuery;
  change: (q: Partial<PPQuery>) => void;
  showPlan: boolean;
  setShowPlan: (v: boolean) => void;
  setQuery: (q: PPQuery) => void;
  setCursor: (c: string) => void;
  setHistory: (h: string[]) => void;
  searchControlRef: React.RefObject<HTMLInputElement | null>;
}) {
  const { query, change, showPlan, setShowPlan, searchControlRef } = p;
  return (
    <div className="pppoe-filters">
      <label>
        Buscar sessões
        <input
          ref={searchControlRef}
          value={query.search}
          onChange={(e) => change({ search: e.target.value })}
        />
      </label>
      <SortAndCheckFilters
        query={query}
        change={change}
        showPlan={showPlan}
        setShowPlan={setShowPlan}
      />
      {(query.search || query.duplicates_only) && (
        <button
          onClick={() => {
            p.setQuery(FRESH_QUERY());
            p.setCursor('');
            p.setHistory([]);
            p.searchControlRef.current?.focus();
          }}
        >
          Limpar filtros
        </button>
      )}
    </div>
  );
}

function SessionPlanCell({
  row,
  fastTrack,
}: {
  row: PPRow;
  fastTrack?: number;
}) {
  const dl = row.context?.download_limit;
  const ul = row.context?.upload_limit;
  const hasLimits = dl !== undefined && ul !== undefined;
  return (
    <td title={`${exactRate(dl)} download / ${exactRate(ul)} upload`}>
      {hasLimits ? `${formatRate(dl)} / ${formatRate(ul)}` : 'Indisponível'}
      {fastTrack !== undefined && fastTrack > 0 && (
        <small className="cell-sub">limite lido, não garantido</small>
      )}
    </td>
  );
}

function PPPoESessionRowItem(p: {
  row: PPRow;
  showPlan: boolean;
  fastTrack?: number;
  contextOmitted?: boolean;
  onDetail: (row: PPRow, trigger: HTMLElement) => void;
}) {
  const { row, showPlan, fastTrack } = p;
  const ctx = row.context;
  const assocText =
    ctx?.association === 'associated'
      ? `${ctx.vlan ?? 'VLAN indisponível'} / ${ctx.service || 'Serviço indisponível'} / ${ctx.interface || 'Interface indisponível'}`
      : p.contextOmitted
        ? 'Contexto não incluído por limite'
        : 'Não associado';
  return (
    <tr>
      <td>{row.login || 'Indisponível'}</td>
      <td>{row.address || 'Indisponível'}</td>
      <td>{row.caller_id || 'Indisponível'}</td>
      <td title={uptimeTitle(row.uptime)}>
        {formatPPUptime(row.uptime, true)}
        {row.state === 'OFFLINE' && (
          <small className="cell-sub">Última sessão</small>
        )}
      </td>
      <td>{row.state || 'ONLINE'}</td>
      {showPlan && <SessionPlanCell row={row} fastTrack={fastTrack} />}
      <td>{assocText}</td>
      <td>
        {row.duplicates > 1
          ? `${row.duplicates} sessões do mesmo login`
          : 'Sem duplicidade'}
      </td>
      <td>
        <button
          aria-label={'Detalhar ' + row.login + ' ' + row.remote_id}
          onClick={(e) => p.onDetail(row, e.currentTarget)}
        >
          Detalhar
        </button>
      </td>
    </tr>
  );
}

export function PPPoESessionsTable(p: {
  rows: PPRow[];
  showPlan: boolean;
  fastTrack?: number;
  contextOmitted?: boolean;
  onDetail: (row: PPRow, trigger: HTMLElement) => void;
}) {
  return (
    <div
      className="table-scroll"
      tabIndex={0}
      aria-label="Tabela de sessões PPPoE"
    >
      <table className="app-table">
        <thead>
          <tr>
            <th>Login</th>
            <th>IP</th>
            <th>Caller ID</th>
            <th>Tempo conectado</th>
            <th>Estado</th>
            {p.showPlan && <th>Plano · download / upload</th>}
            <th>VLAN / serviço / interface</th>
            <th>Duplicidade</th>
            <th>Detalhe</th>
          </tr>
        </thead>
        <tbody>
          {p.rows.map((row) => (
            <PPPoESessionRowItem
              key={row.row_id}
              row={row}
              showPlan={p.showPlan}
              fastTrack={p.fastTrack}
              contextOmitted={p.contextOmitted}
              onDetail={p.onDetail}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function PPPoEPaginationActions(p: {
  history: string[];
  setHistory: React.Dispatch<React.SetStateAction<string[]>>;
  cursor: string;
  setCursor: (c: string) => void;
  next: string;
  pageState: string;
  total: number;
  select: (r: PPRow | undefined) => void;
  searchControlRef: React.RefObject<HTMLInputElement | null>;
}) {
  return (
    <div className="pppoe-actions">
      {p.history.length > 0 && (
        <button
          onClick={() => {
            p.setCursor(p.history[p.history.length - 1]);
            p.setHistory(p.history.slice(0, -1));
            p.searchControlRef.current?.focus();
          }}
        >
          Página anterior
        </button>
      )}
      {p.pageState === 'ready' && (
        <span role="status" aria-label={`${p.total} resultados`}>
          {p.total} resultados
        </span>
      )}
      {p.cursor && (
        <button
          onClick={() => {
            p.setCursor('');
            p.setHistory([]);
            p.searchControlRef.current?.focus();
          }}
        >
          Início da lista
        </button>
      )}
      {p.pageState === 'ready' && p.next && (
        <button
          onClick={() => {
            p.setHistory((v) => [...v, p.cursor]);
            p.setCursor(p.next);
            p.select(undefined);
            p.searchControlRef.current?.focus();
          }}
        >
          Próxima página
        </button>
      )}
    </div>
  );
}
