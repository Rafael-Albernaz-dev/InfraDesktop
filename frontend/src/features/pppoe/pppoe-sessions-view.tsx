import type { PPRow } from '../../platform/wails/presentation';
import {
  exactRate,
  formatDate,
  formatRate,
  uptimeTitle,
} from '../equipment/network-format';
import { FRESH_QUERY } from './pppoe-context-actions';
import { runPPExport } from './pppoe-export';
import { formatPPUptime } from '../equipment/pppoe-format';
import type { PPObservation, PPQuery } from '../../platform/wails/pppoe-types';
import { SessionCPE } from './session-cpe';
import { Button } from '../../shared/ui/Button';
import { DataTable } from '../../shared/ui/DataTable';
import { Select } from '../../shared/ui/Select';
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
  const { current, disabled, busy, exportControlRef, csvExportControlRef } = p;
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
  const offline = current.offline || 0;
  const online = current.online || Math.max(0, current.count - offline);
  return (
    <>
      <div className="bar">
        <span className="muted">
          <span>{online} ONLINE</span> · <span>{offline} OFFLINE</span> ·
          leitura de {formatDate(current.provenance.ended_at)}
        </span>
        <span className="grow" />
        <Button
          type="submit"
          disabled={disabled}
          onClick={p.save}
          aria-label="Salvar referência desta leitura"
        >
          Salvar referência
        </Button>
        <Button
          type="submit"
          ref={csvExportControlRef}
          disabled={busy}
          onClick={() => onExport('operational')}
          aria-label="Exportar CSV operacional"
          title="O CSV contém dados identificáveis de assinantes."
        >
          Exportar CSV
        </Button>
        <Button
          type="submit"
          ref={exportControlRef}
          disabled={busy}
          onClick={() => onExport('sanitized')}
        >
          Exportar JSON sanitizado
        </Button>
      </div>
      {current.context_omitted && (
        <p role="status">
          Contexto não incluído por limite. As sessões e seus estados foram
          preservados.
        </p>
      )}
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
        <Select
          value={query.sort}
          onChange={(e) => change({ sort: e.target.value })}
        >
          <option value="login">Usuário</option>
          <option value="address">IP</option>
          <option value="uptime">Conectado há</option>
          <option value="state">Estado</option>
        </Select>
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
        Somente duplicados
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
      <label className="sr-only" htmlFor="session-search">
        Buscar sessões
      </label>
      <input
        id="session-search"
        placeholder="Buscar sessões"
        ref={searchControlRef}
        value={query.search}
        onChange={(e) => change({ search: e.target.value })}
      />
      <Select
        aria-label="Estado"
        value={query.state || ''}
        onChange={(e) => change({ state: e.target.value as PPQuery['state'] })}
      >
        <option value="">Todos</option>
        <option value="ONLINE">ONLINE</option>
        <option value="OFFLINE">OFFLINE</option>
      </Select>
      <SortAndCheckFilters
        query={query}
        change={change}
        showPlan={showPlan}
        setShowPlan={setShowPlan}
      />
      {(query.search || query.duplicates_only || query.state) && (
        <Button
          type="submit"
          onClick={() => {
            p.setQuery(FRESH_QUERY());
            p.setCursor('');
            p.setHistory([]);
            p.searchControlRef.current?.focus();
          }}
        >
          Limpar filtros
        </Button>
      )}
    </div>
  );
}
function SessionPlanCell({
  row,
  fastTrack,
  showPlan,
}: {
  row: PPRow;
  fastTrack?: number;
  showPlan: boolean;
}) {
  const dl = row.context?.download_limit,
    ul = row.context?.upload_limit;
  return (
    <td title={`${exactRate(dl)} download / ${exactRate(ul)} upload`}>
      {row.context?.profile || 'Não informado nesta leitura'}
      {showPlan && (
        <small className="cell-sub">
          {dl !== undefined && ul !== undefined
            ? `${formatRate(dl)} / ${formatRate(ul)}`
            : 'Não informado nesta leitura'}
          {fastTrack !== undefined &&
            fastTrack > 0 &&
            ' · limite lido, não garantido'}
        </small>
      )}
    </td>
  );
}
type SessionsTableProps = {
  rows: PPRow[];
  showPlan: boolean;
  fastTrack?: number;
  contextOmitted?: boolean;
  profile?: string;
  activation?: string;
  deviceId?: string;
  observation?: string;
  openEquipment?: (id: string) => void;
  onDetail: (row: PPRow, trigger: HTMLElement) => void;
};
function PPPoESessionRowItem(
  p: Omit<SessionsTableProps, 'rows'> & { row: PPRow },
) {
  const { row } = p;
  return (
    <tr className="click">
      <td>
        <Button
          className="row-trigger"
          aria-label={'Detalhar ' + row.login + ' ' + row.remote_id}
          onClick={(e) => p.onDetail(row, e.currentTarget)}
        >
          {row.login || 'Não informado nesta leitura'}
        </Button>
      </td>
      <td>{row.address || 'Não informado nesta leitura'}</td>
      <SessionPlanCell
        row={row}
        fastTrack={p.fastTrack}
        showPlan={p.showPlan}
      />
      <td title={uptimeTitle(row.uptime)}>
        {row.state === 'OFFLINE' ? '—' : formatPPUptime(row.uptime, true)}
      </td>
      <td>{row.state || 'ONLINE'}</td>
      <td>
        <SessionCPE
          profile={p.profile || ''}
          activation={p.activation || ''}
          deviceId={p.deviceId || ''}
          observation={p.observation || ''}
          row={row.row_id}
          open={p.openEquipment}
        />
      </td>
    </tr>
  );
}
export function PPPoESessionsTable(p: SessionsTableProps) {
  return (
    <div
      className="table-region"
      tabIndex={0}
      aria-label="Tabela de sessões PPPoE"
    >
      <DataTable>
        <thead>
          <tr>
            {['Usuário', 'IP', 'Plano', 'Conectado há', 'Estado', 'CPE'].map(
              (label) => (
                <th key={label}>{label}</th>
              ),
            )}
          </tr>
        </thead>
        <tbody>
          {p.rows.map((row) => (
            <PPPoESessionRowItem key={row.row_id} {...p} row={row} />
          ))}
        </tbody>
      </DataTable>
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
    <div className="pager">
      <Button
        aria-label="Início da lista"
        disabled={!p.cursor}
        onClick={() => {
          p.setCursor('');
          p.setHistory([]);
          p.searchControlRef.current?.focus();
        }}
      >
        Início
      </Button>
      <span role="status" aria-label={`${p.total} resultados`}>
        Página {p.history.length + 1}
      </span>
      <Button
        aria-label="Próxima página"
        disabled={p.pageState !== 'ready' || !p.next}
        onClick={() => {
          p.setHistory((v) => [...v, p.cursor]);
          p.setCursor(p.next);
          p.select(undefined);
          p.searchControlRef.current?.focus();
        }}
      >
        Próxima
      </Button>
      {p.history.length > 0 && (
        <Button
          onClick={() => {
            p.setCursor(p.history[p.history.length - 1]);
            p.setHistory(p.history.slice(0, -1));
            p.searchControlRef.current?.focus();
          }}
        >
          Página anterior
        </Button>
      )}
    </div>
  );
}
