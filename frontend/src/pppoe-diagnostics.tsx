import { useDisplayNow } from './equipment-snapshot';
import {
  Fragment,
  useCallback,
  useEffect,
  useEffectEvent,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import {
  pppoeReadAPI as readAPI,
  deviceMonitoringAPI as monitoringAPI,
  pppoeDiagnosticAPI as api,
} from './presentation';
import type {
  PDDiagnostic,
  PDRow,
  PPResult,
  PPReadResult,
  DeviceMonitoringPoint,
  PPReadRow,
  PPRate,
} from './presentation';
import { formatPPUptime } from './pppoe-format';
import {
  formatLatencyRange,
  portLabel,
  formatDate,
  formatClockOffset,
  formatLatency,
  exactNumber,
  exactRate,
  exactVolume,
  formatAtScale,
  formatDuration,
  formatMetric,
  formatRate,
  formatVolume,
  formatVolumePair,
  networkScale,
  uptimeTitle,
} from './network-format';
import type { Device } from './equipment-api';
import { equipmentMessage } from './equipment-api';
import { pendingPPPoE as pending } from './equipment-receipts';

type Scope = 'catalog' | 'radius' | 'ping';
type Source = { observation: string; row: string };
const diagnosticMessage = (code: string) =>
  (
    ({
      PING_CONTEXT_UNAVAILABLE:
        'O contexto main contém VRF, regras ou mangle ativos incompatíveis com a sondagem.',
      PING_SESSION_CHANGED:
        'A sessão mudou no concentrador. Atualize as sessões antes de sondar.',
      PING_READING_STALE:
        'A leitura pertence a um contexto anterior do serviço. Atualize as sessões.',
      PING_OFFLINE: 'Ping indisponível: sessão OFFLINE.',
    }) as Record<string, string>
  )[code] ||
  (code === 'PARTIAL'
    ? 'O concentrador não devolveu uma leitura completa.'
    : code === 'OBSERVATION_INCOMPATIBLE'
      ? 'A leitura não permite uma sondagem segura. Confira a identidade da sessão.'
      : code === 'OBSERVATION_UNAVAILABLE'
        ? 'A leitura foi substituída. Reabra os dados armazenados.'
        : code === 'JOB_INTERRUPTED'
          ? 'A ação anterior foi interrompida. Ela não será reenviada automaticamente.'
          : equipmentMessage(code));

function useDiagnostic(
  profile: string,
  activation: string,
  device: Device,
  scope: Scope,
  source?: Source,
  kind = '',
) {
  const sourceObservation = source?.observation;
  const sourceRow = source?.row;
  const context = [
    profile,
    activation,
    device.id,
    device.revision,
    scope,
    source?.observation,
    source?.row,
  ].join('|');
  const key = [
    profile,
    activation,
    device.id,
    'diag',
    scope,
    device.revision,
    source?.observation || '',
    source?.row || '',
  ].join('|');
  const life = useRef({ generation: 0 });
  const [storedContext, setStoredContext] = useState(context);
  const [meta, setMeta] = useState<PDDiagnostic>();
  const [rows, setRows] = useState<PDRow[]>([]);
  const [attempt, setAttempt] =
    useState<NonNullable<PPResult['data']>['attempt']>();
  const [loading, setLoading] = useState(true),
    [pageLoading, setPageLoading] = useState(false);
  const [error, setError] = useState(''),
    [pageError, setPageError] = useState('');
  const [cursor, setCursor] = useState(''),
    [next, setNext] = useState('');
  const [job, setJob] = useState(''),
    [starting, setStarting] = useState(false);
  const [statusFailed, setStatusFailed] = useState(false),
    [statusRetry, retryStatus] = useState(0);
  const [refresh, reload] = useState(0);
  const [uncertain, setUncertain] = useState(false);
  const active = useCallback(
    (generation: number, result?: PPResult) =>
      generation === life.current.generation &&
      (!result ||
        (result.profile_id === profile && result.activation_id === activation)),
    [profile, activation],
  );
  const acknowledge = useCallback(
    (operation?: string) => {
      if (!operation) return;
      const prefix =
        [profile, activation, device.id, 'diag', scope].join('|') + '|';
      for (const [receiptKey, receipt] of pending)
        if (receiptKey.startsWith(prefix) && receipt.operation === operation)
          pending.delete(receiptKey);
    },
    [profile, activation, device.id, scope],
  );
  // Adjust presentation state during render so no stale surface is committed.
  // The guarded key converges before React paints; durable receipts stay intact.
  if (storedContext !== context) {
    setStoredContext(context);
    setMeta(undefined);
    setRows([]);
    setJob('');
    setCursor('');
    setNext('');
    setError('');
    setPageError('');
    setStarting(false);
    setStatusFailed(false);
    setUncertain(false);
    setAttempt(undefined);
    setLoading(true);
  }
  const readKey = context + '|' + refresh;
  const [storedReadKey, setStoredReadKey] = useState(readKey);
  if (storedReadKey !== readKey) {
    setStoredReadKey(readKey);
    setLoading(true);
  }
  useLayoutEffect(() => {
    const lifecycle = life.current;
    ++lifecycle.generation;
    return () => {
      ++lifecycle.generation;
    };
  }, [context]);
  useEffect(() => {
    const generation = life.current.generation;
    let stopped = false;
    void api
      .current(profile, activation, device.id, scope)
      .then((result) => {
        if (stopped || !active(generation, result)) return;
        if (result.code !== 'OK') {
          setError(diagnosticMessage(result.code));
          return;
        }
        setMeta(result.data?.diagnostic);
        const candidate = result.data?.attempt;
        const latest =
          !sourceObservation ||
          (candidate?.source_observation === sourceObservation &&
            candidate?.source_row === sourceRow)
            ? candidate
            : undefined;
        setAttempt(latest);
        if (latest?.state === 'RUNNING' && latest.job_id) setJob(latest.job_id);
        else acknowledge(latest?.operation_id);
      })
      .catch(() => {
        if (!stopped && active(generation))
          setError('Não foi possível abrir os dados armazenados.');
      })
      .finally(() => {
        if (!stopped && active(generation)) setLoading(false);
      });
    return () => {
      stopped = true;
    };
  }, [
    context,
    refresh,
    profile,
    activation,
    device.id,
    scope,
    active,
    acknowledge,
    sourceObservation,
    sourceRow,
  ]);
  const visibleMeta =
    storedContext === context &&
    (!source ||
      (meta?.source_observation === source.observation &&
        meta?.source_row === source.row))
      ? meta
      : undefined;
  const diagnosticID = visibleMeta?.diagnostic_id;
  const pageKey = [context, diagnosticID, cursor, refresh, kind].join('|');
  const [storedPageKey, setStoredPageKey] = useState(pageKey);
  if (storedPageKey !== pageKey) {
    setStoredPageKey(pageKey);
    setRows([]);
    setNext('');
    setPageError('');
    setPageLoading(!!diagnosticID);
  }
  useEffect(() => {
    let stopped = false;
    const generation = life.current.generation;
    if (!diagnosticID) return;
    void api
      .list(profile, activation, device.id, scope, diagnosticID, cursor, kind)
      .then((result) => {
        if (stopped || !active(generation, result)) return;
        if (result.code !== 'OK') {
          setPageError(diagnosticMessage(result.code));
          return;
        }
        setRows(result.data?.diagnostic_rows || []);
        setNext(result.data?.next_cursor || '');
      })
      .catch(() => {
        if (!stopped && active(generation))
          setPageError('Não foi possível carregar a página.');
      })
      .finally(() => {
        if (!stopped && active(generation)) setPageLoading(false);
      });
    return () => {
      stopped = true;
    };
  }, [
    context,
    diagnosticID,
    kind,
    cursor,
    refresh,
    profile,
    activation,
    device.id,
    scope,
    active,
  ]);
  useEffect(() => {
    if (!job) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const generation = life.current.generation;
    const poll = async () => {
      setStatusFailed(false);
      try {
        const result = await api.status(profile, activation, job);
        if (stopped || !active(generation, result)) return;
        if (result.code !== 'OK' || !result.data?.job) {
          setStatusFailed(true);
          setError(diagnosticMessage(result.code));
          return;
        }
        const current = result.data.job;
        if (current.state === 'RUNNING')
          timer = setTimeout(() => void poll(), 150);
        else {
          acknowledge(current.pppoe?.operation_id);
          setUncertain(false);
          setJob('');
          setCursor('');
          setError(
            current.code === 'OK' || current.code === 'CANCELLED'
              ? ''
              : diagnosticMessage(current.code),
          );
          reload((n) => n + 1);
        }
      } catch {
        if (!stopped && active(generation)) {
          setStatusFailed(true);
          setError(
            'Não foi possível consultar o estado. A ação não será reenviada.',
          );
        }
      }
    };
    void poll();
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [context, job, statusRetry, profile, activation, active, acknowledge]);
  async function start() {
    if (pending.has(key)) {
      const generation = life.current.generation;
      setStarting(true);
      setError('');
      try {
        const result = await api.current(profile, activation, device.id, scope);
        if (!active(generation, result)) return;
        if (result.code !== 'OK') {
          setError(diagnosticMessage(result.code));
          return;
        }
        const latest = result.data?.attempt;
        if (
          latest?.operation_id === pending.get(key)?.operation &&
          latest?.state === 'RUNNING' &&
          latest.job_id
        )
          setJob(latest.job_id);
        else {
          pending.delete(key);
          setUncertain(false);
          setError(
            'Não há trabalho ativo desta ação. Inicie uma nova leitura deliberadamente.',
          );
          reload((n) => n + 1);
        }
      } catch {
        if (active(generation))
          setError(
            'Não foi possível consultar o estado. Nenhuma sondagem foi reenviada.',
          );
      } finally {
        if (active(generation)) setStarting(false);
      }
      return;
    }
    if (!pending.has(key) && pending.size >= 32) {
      setError(
        'Há 32 ações incertas. Reconcilie uma delas no contexto original.',
      );
      return;
    }
    const receipt = pending.get(key) || {
      operation: crypto.randomUUID(),
      revision: device.revision,
    };
    pending.set(key, receipt);
    const generation = life.current.generation;
    setStarting(true);
    setError('');
    setUncertain(false);
    try {
      const result =
        scope === 'ping' && source
          ? await api.ping(
              profile,
              activation,
              device.id,
              source.observation,
              source.row,
              receipt.operation,
              device.revision,
            )
          : await api[scope === 'catalog' ? 'catalog' : 'radius'](
              profile,
              activation,
              device.id,
              receipt.operation,
              device.revision,
            );
      if (!active(generation, result)) return;
      if (result.code === 'OK' && result.data?.job) setJob(result.data.job.id);
      else {
        if (
          [
            'JOB_INTERRUPTED',
            'REVISION_CONFLICT',
            'DEVICE_INACTIVE',
            'TRUST_REQUIRED',
            'OBSERVATION_INCOMPATIBLE',
            'OBSERVATION_UNAVAILABLE',
            'PING_READING_STALE',
            'PING_CONTEXT_UNAVAILABLE',
            'PING_SESSION_CHANGED',
            'PING_OFFLINE',
            'BUSY',
            'LIMIT_EXCEEDED',
          ].includes(result.code)
        )
          acknowledge(receipt.operation);
        setError(diagnosticMessage(result.code));
        setUncertain(pending.has(key));
      }
    } catch {
      if (active(generation)) {
        setUncertain(true);
        setError(
          'Resposta incerta. Consulte a mesma ação antes de iniciar outra.',
        );
      }
    } finally {
      if (active(generation)) setStarting(false);
    }
  }
  async function cancel() {
    const generation = life.current.generation;
    try {
      const result = await api.cancel(profile, activation, job);
      if (active(generation, result) && result.code !== 'OK')
        setError(diagnosticMessage(result.code));
    } catch {
      if (active(generation))
        setError('Cancelamento incerto. Consulte o estado da ação.');
    }
  }
  return {
    meta: visibleMeta,
    rows: visibleMeta ? rows : [],
    attempt,
    loading,
    pageLoading,
    error,
    pageError,
    job,
    starting,
    statusFailed,
    uncertain: uncertain || pending.has(key),
    cursor,
    next,
    start,
    cancel,
    retryStatus: () => retryStatus((n) => n + 1),
    reopen: () => {
      setError('');
      setPageError('');
      setCursor('');
      reload((n) => n + 1);
    },
    first: () => setCursor(''),
    more: () => setCursor(next),
  };
}

const state = (value?: boolean) =>
  value === undefined ? 'Indisponível' : value ? 'Sim' : 'Não';
const relation: Record<string, string> = {
  resolved: 'Pai identificado',
  missing: 'Pai ausente',
  ambiguous: 'Vínculo ambíguo',
  cycle: 'Ciclo na configuração',
  depth_limited: 'Encadeamento excede o limite',
};

function DiagnosticRow({ row }: { row: PDRow }) {
  if (row.not_seen_since)
    return (
      <article className="pppoe-diagnostic-row">
        <h3>{row.name || 'Interface sem nome'}</h3>
        <p title={row.not_seen_since}>
          Não visto desde {formatDate(row.not_seen_since, true)}. Estado atual
          indisponível; última interface dinâmica observada.
        </p>
      </article>
    );
  if (row.kind === 'aaa')
    return (
      <article className="pppoe-diagnostic-row">
        <h3>PPP AAA</h3>
        <p>
          Usa RADIUS: {state(row.use_radius)} · Accounting:{' '}
          {state(row.accounting)}
        </p>
      </article>
    );
  if (row.kind === 'radius')
    return (
      <article className="pppoe-diagnostic-row">
        <h3>{row.address}</h3>
        <p>
          Serviços: {row.services} · Protocolo:{' '}
          {row.protocol === '?' ? 'Indisponível' : row.protocol} · Desativado:{' '}
          {state(row.disabled)}
        </p>
        <p>
          Autenticação: porta {row.auth_port} · Accounting: porta{' '}
          {row.accounting_port}
        </p>
        {row.counters && (
          <dl className="pppoe-diagnostic-values">
            {Object.entries({
              Pendentes: row.counters.pending,
              Solicitações: row.counters.requests,
              Aceitas: row.counters.accepts,
              Rejeitadas: row.counters.rejects,
              Reenvios: row.counters.resends,
              Timeouts: row.counters.timeouts,
              'Respostas inválidas': row.counters.bad_replies,
            }).map(([name, value]) => (
              <div key={name}>
                <dt>{name}</dt>
                <dd>{exactNumber(value)}</dd>
              </div>
            ))}
            <div>
              <dt>RTT da última solicitação</dt>
              <dd title={uptimeTitle(row.counters.last_rtt)}>
                {row.counters.last_rtt.known
                  ? formatLatencyRange(row.counters.last_rtt)
                  : 'Indisponível'}
              </dd>
            </div>
          </dl>
        )}
      </article>
    );
  return (
    <article className="pppoe-diagnostic-row">
      <h3>{row.name || 'Serviço sem nome'}</h3>
      <p>
        {row.kind === 'service'
          ? 'Serviço PPPoE'
          : row.kind === 'vlan'
            ? 'VLAN configurada'
            : `Interface ${row.type || ''}`}{' '}
        · Desativado: {state(row.disabled)}
        {row.running !== undefined
          ? ` · Em execução: ${state(row.running)}`
          : ''}
        {row.dynamic !== undefined ? ` · Dinâmica: ${state(row.dynamic)}` : ''}
      </p>
      {row.parent !== undefined && (
        <p>
          Interface pai: {row.parent} ·{' '}
          {relation[row.relation || ''] || 'Vínculo indisponível'}
        </p>
      )}
      {row.kind === 'vlan' && (
        <p>
          Tag: {row.vlan_id} · Service tag: {state(row.service_tag)}.{' '}
          {row.relation === 'resolved'
            ? 'A relação representa a configuração, sem comprovar tráfego individual.'
            : 'O vínculo não pôde ser comprovado.'}
        </p>
      )}
      {row.kind === 'service' && (
        <p>
          Faixas de VLAN:{' '}
          {row.range === '?'
            ? 'Indisponível neste firmware/permissão'
            : row.range || 'Sem faixa configurada'}{' '}
          · Aceita sem tag: {state(row.accept_untagged)}
        </p>
      )}
    </article>
  );
}

export function PPPoEDiagnostics({
  profile,
  activation,
  device,
  scope,
  blocked,
}: {
  profile: string;
  activation: string;
  device: Device;
  scope: 'catalog' | 'radius';
  blocked: boolean;
}) {
  const [kind, setKind] = useState('');
  const view = useDiagnostic(
    profile,
    activation,
    device,
    scope,
    undefined,
    kind,
  );
  const title =
    scope === 'catalog'
      ? 'Serviços, interfaces e VLANs'
      : 'RADIUS no concentrador';
  return (
    <section
      aria-label={title}
      className="pppoe-diagnostics"
      aria-busy={view.loading || view.pageLoading}
    >
      <h2>{title}</h2>
      <button
        className="primary"
        disabled={
          blocked ||
          view.loading ||
          view.starting ||
          !!view.job ||
          !device.active ||
          !device.fingerprint
        }
        onClick={() => void view.start()}
      >
        {view.starting || view.job
          ? 'Consultando…'
          : view.uncertain
            ? 'Consultar ação pendente'
            : scope === 'catalog'
              ? 'Atualizar serviços e VLANs'
              : 'Atualizar RADIUS'}
      </button>
      {view.job && (
        <button onClick={() => void view.cancel()}>Cancelar consulta</button>
      )}
      {view.statusFailed && (
        <button onClick={view.retryStatus}>Consultar estado novamente</button>
      )}
      {view.error && <p role="alert">{view.error}</p>}
      {view.loading && <p role="status">Carregando diagnóstico armazenado…</p>}
      {!view.loading && !view.error && !view.meta && (
        <p>
          Nenhuma leitura deste diagnóstico disponível. Atualize para consultar
          o concentrador.
        </p>
      )}
      {!view.error &&
        view.attempt &&
        view.attempt.state !== 'RUNNING' &&
        view.attempt.code !== 'OK' && (
          <p role="status">
            Tentativa{' '}
            {view.attempt.quality?.status === 'limited'
              ? 'limitada'
              : 'incompleta'}
            : {diagnosticMessage(view.attempt.code)}{' '}
            {view.meta
              ? 'A última leitura boa e sua data foram preservadas.'
              : 'Nenhuma leitura boa disponível.'}
          </p>
        )}
      {view.meta && (
        <>
          <p>
            Última leitura boa: {formatDate(view.meta.provenance.ended_at)} ·
            RouterOS {view.meta.provenance.version} · {view.meta.count} itens ·
            origem: concentrador via SSH.
          </p>
          {scope === 'catalog' && (
            <label>
              Filtrar por tipo
              <select
                aria-label="Filtrar por tipo"
                value={kind}
                onChange={(e) => {
                  setKind(e.target.value);
                  view.first();
                }}
              >
                <option value="">Todos</option>
                <option value="ether">Ethernet</option>
                <option value="bridge">Bridge</option>
                <option value="vlan">VLAN</option>
                <option value="service">Serviço PPPoE</option>
                <option value="wlan">Wireless</option>
              </select>
            </label>
          )}
          {scope === 'catalog' ? (
            <p>
              VLANs e faixas representam configuração. Não há vínculo individual
              de VLAN comprovado para as sessões.{' '}
              {view.meta.absent_interfaces !== undefined
                ? `${view.meta.absent_interfaces} interfaces dinâmicas não vistas, retidas por até 24 h e separadas das presentes.`
                : ''}
              {view.meta.absences_limited
                ? ' Retenção de ausentes limitada para preservar a leitura atual.'
                : ''}
            </p>
          ) : (
            <p>
              Contadores acumulados desde o boot, agregados por servidor. Eles
              não identificam motivo de rejeição ou autenticação de um
              assinante. Motivo individual indisponível; nenhum log bruto é
              coletado.
            </p>
          )}
          {view.pageLoading && (
            <p role="status">Carregando página armazenada…</p>
          )}
          {!view.pageLoading && !view.pageError && view.rows.length === 0 && (
            <p>Nenhum item configurado nesta leitura completa.</p>
          )}
          {scope === 'radius' &&
            view.meta.count === 1 &&
            view.rows[0]?.kind === 'aaa' && (
              <p>Nenhum servidor RADIUS configurado nesta leitura.</p>
            )}
          {view.rows
            .filter((row) => !row.not_seen_since)
            .map((row) => (
              <DiagnosticRow key={row.id} row={row} />
            ))}
          {view.rows.some((row) => row.not_seen_since) && (
            <section aria-label="Interfaces dinâmicas não vistas na leitura atual">
              <h3>Não vistas na leitura atual</h3>
              {view.rows
                .filter((row) => row.not_seen_since)
                .map((row) => (
                  <DiagnosticRow key={row.id} row={row} />
                ))}
            </section>
          )}
        </>
      )}
      {view.pageError && <p role="alert">{view.pageError}</p>}
      {(view.error || view.pageError) && (
        <button onClick={view.reopen}>Reabrir dados armazenados</button>
      )}
      <div className="pppoe-actions">
        {view.cursor && (
          <button onClick={view.first}>Início do diagnóstico</button>
        )}
        {!view.pageLoading && !view.pageError && view.next && (
          <button onClick={view.more}>Próxima página do diagnóstico</button>
        )}
      </div>
    </section>
  );
}

export function PPPoEPing({
  profile,
  activation,
  device,
  observation,
  row,
  blocked,
}: {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  row: string;
  blocked: boolean;
}) {
  const view = useDiagnostic(profile, activation, device, 'ping', {
    observation,
    row,
  });
  const result = view.rows[0]?.ping;
  return (
    <section
      aria-label="Sondagem da sessão pelo concentrador"
      className="pppoe-diagnostics"
    >
      <h3>Ping pelo concentrador</h3>
      <p>
        3 pacotes · intervalo de 200 ms · contexto main, quando comprovado. A
        sessão é revalidada antes da sondagem.
      </p>
      <button
        disabled={
          blocked ||
          view.loading ||
          view.starting ||
          !!view.job ||
          !device.active ||
          !device.fingerprint
        }
        onClick={() => void view.start()}
      >
        {view.starting || view.job
          ? 'Sondando…'
          : view.uncertain
            ? 'Consultar sondagem pendente'
            : 'Sondar IP pelo concentrador'}
      </button>
      {view.job && (
        <button onClick={() => void view.cancel()}>Cancelar sondagem</button>
      )}
      {view.statusFailed && (
        <button onClick={view.retryStatus}>Consultar estado da sondagem</button>
      )}
      {view.error && <p role="alert">{view.error}</p>}
      {view.pageError && <p role="alert">{view.pageError}</p>}
      {view.loading && (
        <p role="status">Carregando última sondagem armazenada…</p>
      )}
      {!view.error &&
        view.attempt?.source_observation === observation &&
        view.attempt?.source_row === row &&
        view.attempt.state !== 'RUNNING' &&
        view.attempt.code !== 'OK' && (
          <p role="status">
            Tentativa de sondagem incompleta:{' '}
            {diagnosticMessage(view.attempt.code)}{' '}
            {view.meta
              ? 'A última sondagem boa e sua data foram preservadas.'
              : 'Nenhuma sondagem boa disponível.'}
          </p>
        )}
      {!view.loading && !view.error && !view.meta && (
        <p>Nenhuma sondagem armazenada para esta sessão.</p>
      )}
      {(view.error || view.pageError) && (
        <button onClick={view.reopen}>Reabrir sondagem armazenada</button>
      )}
      {result && view.meta && (
        <>
          <p>
            Origem: {result.context} ·{' '}
            {formatDate(view.meta.provenance.ended_at)}
          </p>
          <p>
            Enviados: {result.sent} · Recebidos: {result.received} · Perda:{' '}
            {formatMetric(result.loss_percent, '%')}
          </p>
          <p>
            RTT mínimo:{' '}
            <span title={exactNumber(result.min_ns, 'ns')}>
              {formatLatency(result.min_ns)}
            </span>{' '}
            · médio:{' '}
            <span title={exactNumber(result.avg_ns, 'ns')}>
              {formatLatency(result.avg_ns)}
            </span>{' '}
            · máximo:{' '}
            <span title={exactNumber(result.max_ns, 'ns')}>
              {formatLatency(result.max_ns)}
            </span>
          </p>
          <p>
            Amostra de 3 probes · jitter:{' '}
            <span title={exactNumber(result.jitter_ns, 'ns')}>
              {formatLatency(result.jitter_ns)}
            </span>
            .
          </p>
          <p>
            Mediana dos 10 pings anteriores bem-sucedidos deste alvo e sessão:{' '}
            <span title={exactNumber(result.baseline_median_ns, 'ns')}>
              {formatLatency(result.baseline_median_ns)}
            </span>
            {result.baseline_median_ns !== undefined &&
            result.baseline_median_ns > 0 &&
            result.avg_ns !== undefined
              ? ` · RTT médio ${result.avg_ns >= result.baseline_median_ns ? '+' : '−'}${formatMetric((Math.abs(result.avg_ns - result.baseline_median_ns) * 100) / result.baseline_median_ns, '%')}`
              : ''}
            .{' '}
            {result.baseline_samples !== undefined
              ? `${result.baseline_samples}/10 amostras anteriores disponíveis.`
              : 'Histórico não registrado.'}{' '}
            Histórico em memória desde o início do serviço; perdas não entram na
            baseline.
          </p>
          <ol>
            {result.probes.map((probe) => (
              <li key={probe.sequence} value={probe.sequence + 1}>
                {probe.status || 'Resposta ICMP'}
                {probe.rtt_ns !== undefined
                  ? ` · ${formatLatency(probe.rtt_ns)}`
                  : ''}
              </li>
            ))}
          </ol>
        </>
      )}
      <p>
        Não responder não significa estar offline. A validação e a sondagem
        ocorrem em uma janela; resposta ICMP não comprova navegação saudável.
      </p>
    </section>
  );
}

function RateValue({ rate }: { rate?: PPRate }) {
  return <span title={exactRate(rate)}>{formatRate(rate)}</span>;
}

function ppReadMessage(code: string) {
  if (code === 'PARTIAL')
    return 'Não foi possível concluir esta leitura. Tente novamente.';
  return code === 'UNSUPPORTED'
    ? 'Esta leitura não está disponível neste formato, versão ou permissão.'
    : equipmentMessage(code);
}

type PPReadPanelProps = {
  profile: string;
  activation: string;
  device: Device;
  observation: string;
  rows: string[];
  blocked?: boolean;
  allowTop?: boolean;
  logRow?: string;
  health?: boolean;
  monitoring?: boolean;
  onRead?: () => void;
  summaryOnly?: boolean;
  traffic?: boolean;
};
export function PPPoEReadPanel(props: PPReadPanelProps) {
  const key = [
    props.profile,
    props.activation,
    props.device.id,
    props.device.revision,
    props.observation,
    props.rows.join(','),
    props.allowTop,
    props.logRow,
    props.health,
    props.monitoring,
    props.traffic,
  ].join('|');
  return <PPPoEReadContent key={key} {...props} />;
}
function PPPoEReadContent({
  profile,
  activation,
  device,
  observation,
  rows,
  blocked = false,
  allowTop = false,
  logRow,
  health = false,
  monitoring = false,
  onRead,
  summaryOnly = false,
  traffic = false,
}: PPReadPanelProps) {
  const [job, setJob] = useState(''),
    [starting, setStarting] = useState(false),
    [pending, setPending] = useState(false),
    [cancelling, setCancelling] = useState(false);
  const [pageCursor, setPageCursor] = useState(''),
    [next, setNext] = useState(''),
    [pageBusy, setPageBusy] = useState(false),
    [resultJob, setResultJob] = useState('');
  const [error, setError] = useState(''),
    [failureReason, setFailureReason] = useState(''),
    [result, setResult] = useState<PPReadResult>(),
    [scope, setScope] = useState('bandwidth');
  const [pollRetry, retryPoll] = useState(0),
    [statusLatency, setStatusLatency] = useState<number>(),
    [firstStatus, setFirstStatus] = useState<number>(),
    [deadlineReached, setDeadlineReached] = useState(false),
    [sampleAge, setSampleAge] = useState(0),
    [stoppedTraffic, setStoppedTraffic] = useState(false);
  const autoStarted = useRef(false);
  const generation = useRef(0),
    activeJob = useRef(''),
    privateResultJob = useRef(''),
    pendingOperation = useRef('');
  const liveStarted = useRef(0),
    requestedAt = useRef(0),
    sampleClock = useRef(0),
    lastSample = useRef('');
  useEffect(() => {
    generation.current += 1;
    return () => {
      generation.current += 1;
      const id = activeJob.current || privateResultJob.current,
        operation = pendingOperation.current;
      activeJob.current = '';
      privateResultJob.current = '';
      pendingOperation.current = '';
      if (id) void readAPI.cancel(profile, activation, id).catch(() => {});
      else if (operation)
        void readAPI
          .cancelOperation(profile, activation, device.id, operation)
          .catch(() => {});
    };
  }, [profile, activation, device.id]);
  const valid = (r: PPResult) =>
    r.profile_id === profile && r.activation_id === activation;
  async function start(mode: string) {
    requestedAt.current = performance.now();
    setStatusLatency(undefined);
    setFirstStatus(undefined);
    if (pendingOperation.current || activeJob.current) return;
    if (privateResultJob.current) {
      void readAPI
        .cancel(profile, activation, privateResultJob.current)
        .catch(() => {});
      privateResultJob.current = '';
    }
    const g = ++generation.current,
      operation = crypto.randomUUID();
    pendingOperation.current = operation;
    setPending(true);
    setScope(mode);
    setStoppedTraffic(false);
    setStarting(true);
    setError('');
    setFailureReason('');
    setResult(undefined);
    setNext('');
    setPageCursor('');
    setResultJob('');
    setDeadlineReached(false);
    liveStarted.current = performance.now();
    sampleClock.current = liveStarted.current;
    lastSample.current = '';
    setSampleAge(0);
    try {
      const r = await readAPI.start(
        profile,
        activation,
        device.id,
        operation,
        mode,
        mode === 'top' || mode === 'health' || mode === 'operations'
          ? ''
          : observation,
        mode === 'top' || mode === 'health' || mode === 'operations'
          ? []
          : mode === 'log' || mode === 'secret'
            ? logRow
              ? [logRow]
              : []
            : rows,
        device.revision,
      );
      if (g !== generation.current) {
        if (valid(r) && r.data?.job)
          void readAPI
            .cancel(profile, activation, r.data.job.id)
            .catch(() => {});
        return;
      }
      if (!valid(r) || r.code !== 'OK' || !r.data?.job) {
        if (
          [
            'BUSY',
            'REVISION_CONFLICT',
            'INVALID_ARGUMENT',
            'PROFILE_CHANGED',
            'UNAUTHENTICATED',
            'DEVICE_INACTIVE',
            'TRUST_REQUIRED',
            'VAULT_UNAVAILABLE',
            'OBSERVATION_UNAVAILABLE',
            'OBSERVATION_INCOMPATIBLE',
            'LIMIT_EXCEEDED',
            'UNSUPPORTED',
          ].includes(r.code)
        ) {
          pendingOperation.current = '';
          setPending(false);
        }
        setError(ppReadMessage(r.code));
        return;
      }
      pendingOperation.current = '';
      setPending(false);
      activeJob.current = r.data.job.id;
      if (mode === 'secret' || mode === 'traffic')
        privateResultJob.current = r.data.job.id;
      setJob(r.data.job.id);
      setResultJob(r.data.job.id);
    } catch {
      if (g === generation.current)
        setError(
          'Não foi possível confirmar a solicitação. Cancele a solicitação pendente antes de iniciar outra.',
        );
    } finally {
      if (g === generation.current) setStarting(false);
    }
  }
  const autoStartTraffic = useEffectEvent(() => {
    if (!autoStarted.current) {
      autoStarted.current = true;
      void start('traffic');
    }
  });
  useEffect(() => {
    if (!traffic || blocked || !rows.length) return;
    const timer = setTimeout(() => autoStartTraffic(), 0);
    return () => clearTimeout(timer);
  }, [traffic, blocked, rows.length]);
  useEffect(() => {
    if (!job) return;
    let alive = true;
    const g = generation.current;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      try {
        const beforeStatus = performance.now();
        const r = await readAPI.status(profile, activation, job);
        if (!alive || g !== generation.current) return;
        setStatusLatency(performance.now() - beforeStatus);
        if (r.code === 'OK' && r.data?.job)
          setFirstStatus(
            (known) => known ?? performance.now() - requestedAt.current,
          );
        if (
          r.profile_id !== profile ||
          r.activation_id !== activation ||
          r.code !== 'OK' ||
          !r.data?.job
        ) {
          setError(ppReadMessage(r.code));
          return;
        }
        setError('');
        if (r.data.read) {
          if (lastSample.current !== r.data.read.ended_at) {
            lastSample.current = r.data.read.ended_at;
            sampleClock.current = performance.now();
            setSampleAge(0);
          }
          setResult(r.data.read);
          setNext(r.data.next_cursor || '');
        }
        if (r.data.job.state === 'RUNNING') {
          timer = setTimeout(poll, 350);
          return;
        }
        activeJob.current = '';
        setJob('');
        setDeadlineReached(false);
        if (r.data.job.code === 'OK' && scope === 'operations') onRead?.();
        if (r.data.job.code !== 'OK') {
          setError(ppReadMessage(r.data.job.code));
          setFailureReason(r.error?.reason || '');
        }
      } catch {
        if (alive && g === generation.current)
          setError(
            'Estado da leitura indisponível. Consulte o estado ou cancele a solicitação.',
          );
      }
    };
    void poll();
    return () => {
      alive = false;
      clearTimeout(timer);
    };
  }, [job, profile, activation, pollRetry, scope, onRead]);
  useEffect(() => {
    if (scope !== 'live' && scope !== 'traffic') return;
    if (!job && scope === 'live') return;
    const tick = () => {
      const now = performance.now();
      setSampleAge(Math.max(0, now - sampleClock.current));
      if (job && now - liveStarted.current >= 60000) {
        setDeadlineReached(true);
        clearInterval(timer);
      }
    };
    const timer = setInterval(tick, 1000),
      initial = setTimeout(tick, 0);
    return () => {
      clearInterval(timer);
      clearTimeout(initial);
    };
  }, [job, scope]);
  async function cancel() {
    const id = activeJob.current,
      operation = pendingOperation.current;
    if (!id && !operation) return;
    const g = ++generation.current;
    setCancelling(true);
    try {
      const r = id
        ? await (traffic ? readAPI.stop : readAPI.cancel)(
            profile,
            activation,
            id,
          )
        : await readAPI.cancelOperation(
            profile,
            activation,
            device.id,
            operation,
          );
      if (g !== generation.current) return;
      if (valid(r) && r.code === 'OK') {
        activeJob.current = '';
        pendingOperation.current = '';
        setPending(false);
        setJob('');
        setStarting(false);
        setError('');
        if (traffic) {
          setStoppedTraffic(true);
          setDeadlineReached(false);
        }
      } else setError(ppReadMessage(r.code));
    } catch {
      if (g === generation.current)
        setError(
          'Cancelamento não confirmado; o prazo da leitura continua limitado.',
        );
    } finally {
      if (g === generation.current) setCancelling(false);
    }
  }
  async function page(cursor: string) {
    const g = generation.current;
    setPageBusy(true);
    try {
      const r = await readAPI.page(profile, activation, resultJob, cursor);
      if (g !== generation.current) return;
      if (!valid(r) || r.code !== 'OK' || !r.data?.read) {
        setError(ppReadMessage(r.code));
        return;
      }
      setResult(r.data.read);
      setPageCursor(cursor);
      setNext(r.data.next_cursor || '');
      setError('');
    } catch {
      if (g === generation.current) setError('Página da leitura indisponível.');
    } finally {
      if (g === generation.current) setPageBusy(false);
    }
  }
  if (!rows.length && !allowTop && !logRow && !health && !monitoring)
    return null;
  const locked =
    blocked || starting || pending || Boolean(job) || cancelling || pageBusy;
  const calculated =
    Boolean(job) &&
    (scope === 'live' || scope === 'traffic') &&
    !error &&
    !deadlineReached;
  return (
    <section
      className="pppoe-diagnostics pppoe-read-panel"
      aria-label={
        traffic
          ? 'Tráfego'
          : monitoring
            ? 'Leitura operacional'
            : health
              ? 'Pools e interfaces estáticas'
              : 'Banda por sessão'
      }
    >
      <h3>
        {traffic
          ? 'Tráfego'
          : monitoring
            ? 'Leitura operacional'
            : health
              ? 'Pools e interfaces estáticas'
              : 'Banda por sessão'}
      </h3>
      {monitoring && (
        <button
          className="primary"
          disabled={locked}
          onClick={() => void start('operations')}
        >
          Atualizar monitoramento
        </button>
      )}
      {health && (
        <button disabled={locked} onClick={() => void start('health')}>
          Consultar pools e interfaces
        </button>
      )}
      {rows.length > 0 && !traffic && (
        <button disabled={locked} onClick={() => void start('bandwidth')}>
          {rows.length === 1
            ? 'Medir banda do assinante'
            : 'Medir banda da página'}
        </button>
      )}
      {rows.length > 0 && !traffic && (
        <button disabled={locked} onClick={() => void start('live')}>
          Observar ao vivo por até 60 s
        </button>
      )}
      {traffic && !locked && (error || result || stoppedTraffic) && (
        <button onClick={() => void start('traffic')}>
          Iniciar nova observação
        </button>
      )}
      {allowTop && (
        <button disabled={locked} onClick={() => void start('top')}>
          Buscar top consumo
        </button>
      )}
      {logRow && (
        <>
          <button disabled={locked} onClick={() => void start('log')}>
            Consultar log PPP
          </button>
          <button disabled={locked} onClick={() => void start('secret')}>
            Ver evidência do secret
          </button>
        </>
      )}
      {(job || pending || starting) && (
        <button disabled={cancelling} onClick={() => void cancel()}>
          {traffic && job
            ? 'Parar'
            : !job
              ? 'Cancelar solicitação pendente'
              : scope === 'live' && !deadlineReached
                ? 'Encerrar ao vivo'
                : 'Cancelar medição'}
        </button>
      )}
      {(error || deadlineReached) && job && (
        <button onClick={() => retryPoll((n) => n + 1)}>
          Consultar estado da leitura
        </button>
      )}
      {deadlineReached ? (
        <p role="status">
          Prazo da observação encerrado. Consulte o estado para confirmar o
          término.
        </p>
      ) : (
        (starting || job) &&
        !error && (
          <p role="status">
            {scope === 'live' || scope === 'traffic'
              ? 'Observação limitada a 60 s; atualização aproximada a cada 2 s.'
              : scope === 'operations'
                ? 'Consultando projeções operacionais somente leitura…'
                : scope === 'log'
                  ? 'Consultando eventos PPP deste login…'
                  : scope === 'secret'
                    ? 'Consultando evidência efêmera do secret…'
                    : 'Aguardando duas amostras do concentrador…'}
          </p>
        )
      )}
      {traffic && (
        <p>
          {result
            ? `Última atualização há ${Math.floor(sampleAge / 1000)} s.`
            : job || starting
              ? 'Aguardando primeira amostra.'
              : 'Nenhuma amostra recebida.'}{' '}
          Observação de até 60 s; gráfico retido em memória por 5 min.
        </p>
      )}
      {stoppedTraffic && <p role="status">Observação parada.</p>}
      {error && <p role="alert">{error}</p>}
      {monitoring && firstStatus !== undefined && (
        <p>
          Estado do job consultável após {formatMetric(firstStatus, 'ms')} ·
          consulta de estado (UI → serviço): {formatMetric(statusLatency, 'ms')}
          .
        </p>
      )}
      {error && failureReason && (
        <details>
          <summary>Detalhe técnico</summary>
          <p>Motivo: {failureReason}</p>
        </details>
      )}
      {result && (
        <>
          <p>
            {error || deadlineReached ? 'Último resultado de ' : 'Leitura de '}
            {formatDate(result.ended_at)} · duração{' '}
            <span title={exactNumber(result.duration_ns / 1e9, 's')}>
              {formatDuration(result.duration_ns / 1e9)}
            </span>
          </p>
          {result.scope === 'operations' && (
            <dl className="pppoe-diagnostic-values">
              <dt>Handshake SSH</dt>
              <dd title={exactNumber(result.cost?.ssh_handshake_ns, 'ns')}>
                {formatLatency(result.cost?.ssh_handshake_ns)}
              </dd>
              <dt>Coletores</dt>
              <dd title={exactNumber(result.cost?.collector_ns, 'ns')}>
                {formatLatency(result.cost?.collector_ns)}
              </dd>
            </dl>
          )}
          {result.scope === 'top' && (
            <p>
              Consumo ordenado pelo limite inferior de download + upload.
              Valores indisponíveis aparecem ao final.
            </p>
          )}
          {result.scope !== 'log' &&
            result.scope !== 'secret' &&
            result.scope !== 'operations' &&
            result.cost && (
              <p>
                {result.cost.ssh_commands} comandos SSH ·{' '}
                {result.cost.first_tunnels}/{result.cost.last_tunnels}{' '}
                {result.scope === 'health' ? 'interfaces estáticas' : 'túneis'}{' '}
                nas amostras ·{' '}
                <span title={exactVolume(result.cost.ssh_bytes)}>
                  {formatVolume(result.cost.ssh_bytes)}
                </span>
                {result.cost.cpu_start !== undefined &&
                result.cost.cpu_end !== undefined
                  ? ` · CPU observada: ${formatMetric(result.cost.cpu_start, '%')} / ${formatMetric(result.cost.cpu_end, '%')}`
                  : ''}
              </p>
            )}
          {result.scope === 'log' && (
            <p>
              Até 50 eventos PPP, sem depuração. Horários informados pelo
              equipamento; categorias não comprovam a causa da falha.
            </p>
          )}
          {result.scope === 'operations' ? (
            summaryOnly ? null : (
              <DeviceOperationalTables result={result} />
            )
          ) : result.scope === 'traffic' ? (
            <>
              {result.rows.map((row) => (
                <PPTrafficChart key={row.row_id} row={row} />
              ))}
            </>
          ) : result.scope === 'secret' ? (
            <PPSecretEvidence result={result} />
          ) : result.scope === 'health' ? (
            <PPHealthTables result={result} />
          ) : result.rows.length === 0 ? (
            <p>
              {result.scope === 'operations'
                ? 'Consultando projeções operacionais somente leitura…'
                : scope === 'log'
                  ? 'Nenhum evento PPP reconhecido para este login.'
                  : 'Nenhum túnel ativo na leitura concluída.'}
            </p>
          ) : result.scope === 'log' ? (
            <div className="table-scroll" tabIndex={0} aria-label="Eventos PPP">
              <table className="app-table">
                <thead>
                  <tr>
                    <th>Horário no equipamento</th>
                    <th>Evento</th>
                  </tr>
                </thead>
                <tbody>
                  {result.rows.map((row, i) => (
                    <tr key={i}>
                      <td>{row.at || 'Horário indisponível'}</td>
                      <td>
                        {(
                          {
                            auth_failed: 'Falha de autenticação',
                            address_failed: 'Endereço IP indisponível',
                            radius_timeout: 'Servidor RADIUS sem resposta',
                            timeout: 'Tempo esgotado',
                            disconnected: 'Conexão encerrada',
                            connected: 'Conexão estabelecida',
                            other: 'Evento PPP sem classificação',
                          } as Record<string, string>
                        )[row.event || 'other'] ||
                          'Evento PPP sem classificação'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div
              className="table-scroll"
              tabIndex={0}
              aria-label="Tabela de banda e consumo"
            >
              <table className="app-table">
                <thead>
                  <tr>
                    <th>Login</th>
                    <th>Download do assinante (TX concentrador)</th>
                    <th>Upload do assinante (RX concentrador)</th>
                    <th>Tempo conectado</th>
                  </tr>
                </thead>
                <tbody>
                  {result.rows.map((r, i) => (
                    <tr key={r.row_id || i}>
                      <td>
                        {r.login || 'Indisponível'}
                        {r.name && <small className="cell-sub">{r.name}</small>}
                      </td>
                      <td>
                        <RateValue rate={r.download} />
                      </td>
                      <td>
                        <RateValue rate={r.upload} />
                      </td>
                      <td>
                        <span title={uptimeTitle(r.uptime)}>
                          {formatPPUptime(
                            r.uptime.known && calculated
                              ? {
                                  ...r.uptime,
                                  min: r.uptime.min + sampleAge * 1e6,
                                  max: r.uptime.max + sampleAge * 1e6,
                                }
                              : r.uptime,
                          )}
                        </span>
                        {calculated ? ' (calculado)' : ''}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {next && !summaryOnly && (
            <button disabled={pageBusy} onClick={() => void page(next)}>
              {result.scope === 'operations'
                ? 'Próxima página do monitoramento'
                : result.scope === 'health'
                  ? 'Próxima página de pools e interfaces'
                  : result.scope === 'log'
                    ? 'Próximos eventos PPP'
                    : 'Próxima página de consumo'}
            </button>
          )}
          {resultJob &&
            (result.scope === 'top' ||
              result.scope === 'operations' ||
              result.scope === 'log' ||
              result.scope === 'health') &&
            pageCursor && (
              <button disabled={pageBusy} onClick={() => void page('')}>
                {result.scope === 'health'
                  ? 'Início de pools e interfaces'
                  : result.scope === 'log'
                    ? 'Primeiros eventos PPP'
                    : 'Início do consumo'}
              </button>
            )}
        </>
      )}
    </section>
  );
}

function PPSecretEvidence({ result }: { result: PPReadResult }) {
  return (
    <section aria-label="Evidência efêmera do secret">
      <p>
        Texto livre do administrador no roteador; pode conter dado pessoal e não
        é motivo verificado
      </p>
      <p>Evidência apenas nesta consulta; não é salva nem exportada.</p>
      {result.rows.length === 0 && <p>Nenhum secret local para este login.</p>}
      {result.rows.map((row, index) => {
        const secret = row.secret;
        return (
          <section key={index} aria-label={`Secret ${index + 1}`}>
            <h4>Secret {index + 1}</h4>
            <dl className="pppoe-diagnostic-values pppoe-context-values">
              <dt>Desativado</dt>
              <dd>
                {secret?.disabled === undefined
                  ? 'Indisponível'
                  : secret.disabled
                    ? 'Sim'
                    : 'Não'}
              </dd>
              {(
                [
                  ['Serviço', secret?.service],
                  ['Perfil', secret?.profile],
                  ['Última saída', secret?.last_logged_out],
                  ['Último caller ID', secret?.last_caller_id],
                  [
                    'Último motivo de desconexão',
                    secret?.last_disconnect_reason,
                  ],
                  ['Comentário (até 128 caracteres)', secret?.comment],
                ] as const
              ).map(([label, value]) => (
                <Fragment key={label}>
                  <dt>{label}</dt>
                  <dd>
                    {value === undefined
                      ? 'Indisponível'
                      : value || 'Sem texto informado'}
                  </dd>
                </Fragment>
              ))}
            </dl>
          </section>
        );
      })}
    </section>
  );
}

function PPHealthTables({ result }: { result: PPReadResult }) {
  const pools = result.rows.filter((r) => r.kind === 'pool'),
    interfaces = result.rows.filter((r) => r.kind === 'uplink');
  const flag = (v?: boolean) =>
    v === undefined ? 'Indisponível' : v ? 'Sim' : 'Não';
  return (
    <>
      <h4>Pools IPv4</h4>
      {result.pools_available !== true ? (
        <p>Leitura de pools indisponível nesta versão ou permissão.</p>
      ) : !pools.length ? (
        <p>
          {result.pool_total === 0
            ? 'Nenhum pool configurado.'
            : 'Pools disponíveis em outra página.'}
        </p>
      ) : (
        <div className="table-scroll" tabIndex={0} aria-label="Pools IPv4">
          <table className="app-table">
            <thead>
              <tr>
                <th>Pool</th>
                <th>Endereços em uso</th>
                <th>Endereços configurados</th>
              </tr>
            </thead>
            <tbody>
              {pools.map((r, i) => (
                <tr key={i}>
                  <td>{r.name}</td>
                  <td>{r.used ?? 'Indisponível'}</td>
                  <td>{r.size ?? 'Indisponível'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <h4>Interfaces estáticas</h4>
      <p>
        TX e RX são vistos pela interface do concentrador. Tráfego e contadores
        não identificam a causa de uma falha.
      </p>
      {result.uplinks_available !== true ? (
        <p>Leitura de interfaces indisponível nesta versão ou permissão.</p>
      ) : !interfaces.length ? (
        <p>
          {result.uplink_total === 0
            ? 'Nenhuma interface estática disponível.'
            : 'Interfaces disponíveis em outra página.'}
        </p>
      ) : (
        <div
          className="table-scroll"
          tabIndex={0}
          aria-label="Interfaces estáticas"
        >
          <table className="app-table">
            <thead>
              <tr>
                <th>Interface</th>
                <th>TX da interface</th>
                <th>RX da interface</th>
                <th>Em execução</th>
                <th>Desativada</th>
                <th>Erros RX</th>
                <th>Erros TX</th>
                <th>Descartes RX</th>
                <th>Descartes TX</th>
              </tr>
            </thead>
            <tbody>
              {interfaces.map((r, i) => (
                <tr key={i}>
                  <td>{r.name}</td>
                  <td>
                    <RateValue rate={r.tx} />
                  </td>
                  <td>
                    <RateValue rate={r.rx} />
                  </td>
                  <td>{flag(r.running)}</td>
                  <td>{flag(r.disabled)}</td>
                  <td>{exactNumber(r.rx_errors)}</td>
                  <td>{exactNumber(r.tx_errors)}</td>
                  <td>{exactNumber(r.rx_drops)}</td>
                  <td>{exactNumber(r.tx_drops)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
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
  return (
    <DeviceMonitoringContent
      key={[profile, activation, device.id, device.revision].join('|')}
      profile={profile}
      activation={activation}
      device={device}
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
  const displayNow = useDisplayNow();
  const [scope, setScope] = useState('series'),
    [cursor, setCursor] = useState(''),
    [next, setNext] = useState('');
  const [rows, setRows] = useState<PPReadResult['rows']>([]),
    [state, setState] = useState<'loading' | 'ready' | 'error'>('loading'),
    [error, setError] = useState(''),
    [refresh, bump] = useState(0);
  const [comparison, setComparison] = useState<{
    version?: boolean;
    packages?: boolean;
    count?: number;
  }>({});
  const completed = useCallback(() => {
    setCursor('');
    bump((n) => n + 1);
  }, []);
  useEffect(() => {
    let alive = true;
    void (async () => {
      const r = await monitoringAPI.current(
        profile,
        activation,
        device.id,
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
            : ppReadMessage(r.code),
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
  }, [profile, activation, device.id, scope, cursor, refresh]);
  function change(value: string) {
    setScope(value);
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
        {[
          ['series', 'Curva de sessões'],
          ['flapping', 'Oscilação'],
          ['versions', 'Versões'],
          ['hygiene', 'Higiene de secrets'],
          ['duplicates', 'Entre concentradores'],
        ].map(([value, label]) => (
          <button
            key={value}
            role="tab"
            aria-selected={scope === value}
            onClick={() => change(value)}
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
          {scope === 'series' && (
            <>
              <p>
                Pontos por minuto durante 24 h; depois, mínimo, máximo e último
                por hora até 7 dias. Quebras de continuidade não comprovam
                queda. Queda brusca: pelo menos 5 sessões e 20% entre leituras
                compatíveis.
              </p>
              <SessionMonitoringCurve
                points={rows.flatMap((r) => (r.point ? [r.point] : []))}
              />
            </>
          )}
          {scope === 'flapping' && (
            <p>
              Transições ONLINE ↔ OFFLINE observadas nas últimas 24 h. Leituras
              duplicadas, falhas, reboot e contexto incompatível não contam como
              oscilação.
            </p>
          )}
          {scope === 'duplicates' && (
            <>
              <p>
                Última leitura deste concentrador e dos demais da empresa: ambas
                com idade de até 15 min e janelas de coleta sobrepostas. Sem
                essas condições: não comparável (leituras distantes).
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
                          {[r.duplicate!.a, r.duplicate!.b].map((peer, j) => (
                            <td key={j} title={uptimeTitle(peer.uptime)}>
                              {peer.name} ({peer.endpoint}) ·{' '}
                              {peer.address || 'IP indisponível'} ·{' '}
                              {peer.caller_id || 'Caller ID indisponível'} ·{' '}
                              {formatPPUptime(peer.uptime)} · leitura há{' '}
                              {formatDuration(
                                (displayNow - Date.parse(peer.at)) / 1000,
                              )}
                            </td>
                          ))}
                        </tr>
                      ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
          {scope === 'versions' && (
            <p>
              {comparison.version
                ? 'Divergência de versões nas leituras armazenadas.'
                : 'Versões das últimas leituras disponíveis.'}{' '}
              Equipamentos sem leitura não participam da comparação.{' '}
              {comparison.packages === undefined
                ? 'Comparação de pacotes indisponível: menos de dois conjuntos completos.'
                : comparison.packages
                  ? 'Divergência de pacotes nos conjuntos completos.'
                  : 'Pacotes iguais nos conjuntos completos comparados.'}{' '}
              Comparação considera todos os {comparison.count ?? 0} equipamentos
              com leitura, inclusive outras páginas.
            </p>
          )}
          {!rows.length && (
            <p>
              {scope === 'series'
                ? 'Nenhum ponto armazenado. Atualize as sessões ou o monitoramento.'
                : scope === 'flapping'
                  ? 'Nenhuma transição registrada na janela disponível.'
                  : 'Nenhuma leitura disponível para este grupo.'}
            </p>
          )}
          {rows
            .filter((r) => !r.point && !r.duplicate)
            .map((r, i) => (
              <div key={i}>
                {r.kind === 'flapping' ? (
                  <p>
                    {r.login}: {r.used} transições · primeira{' '}
                    {r.at ? formatDate(r.at) : 'Indisponível'} · última{' '}
                    {r.value ? formatDate(r.value) : 'Indisponível'}
                  </p>
                ) : r.kind === 'version' ? (
                  <p>
                    {r.name}: {r.value || 'Indisponível'} ·{' '}
                    {r.at ? formatDate(r.at) : 'Data indisponível'} · pacotes{' '}
                    {r.event === 'complete'
                      ? (r.used ?? 'Indisponível')
                      : 'Indisponível'}{' '}
                    · origem{' '}
                    {r.source_at ? formatDate(r.source_at) : 'Indisponível'}
                  </p>
                ) : r.kind === 'hygiene' ? (
                  <p>
                    {r.name}: {r.used ?? 'Indisponível'} ·{' '}
                    {r.size ?? 'Indisponível'} ONLINE sem evidência disponível ·
                    origem {r.at ? formatDate(r.at) : 'Indisponível'}
                  </p>
                ) : (
                  <p>
                    {r.name === 'retention'
                      ? 'Parte da série foi removida pelo limite de armazenamento.'
                      : scope === 'duplicates'
                        ? `${r.name}: ${r.value || 'não comparável (leitura indisponível)'}`
                        : `${r.name}: indisponível nesta fonte.`}
                  </p>
                )}
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
function SessionMonitoringCurve({
  points,
}: {
  points: DeviceMonitoringPoint[];
}) {
  const ordered = [...points].reverse();
  const maximum = Math.max(
    1,
    ...ordered.flatMap((p) =>
      p.online_max === undefined ? [] : [p.online_max],
    ),
  );
  const first = Math.min(...ordered.map((p) => Date.parse(p.at))),
    last = Math.max(...ordered.map((p) => Date.parse(p.at)));
  const x = (p: DeviceMonitoringPoint) =>
    ((Date.parse(p.at) - first) / Math.max(1, last - first)) * 600;
  const paths: string[] = [];
  let path = '';
  for (let i = 0; i < ordered.length; i++) {
    const p = ordered[i],
      previous = ordered[i - 1];
    if (
      p.gap ||
      p.online === undefined ||
      (previous &&
        Date.parse(p.at) - Date.parse(previous.at) >
          Math.max(p.resolution, previous.resolution) * 1000)
    ) {
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
    <>
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
        {ordered.map((p, i) =>
          p.online === undefined ? null : (
            <circle
              key={i}
              cx={x(p)}
              cy={135 - (p.online / maximum) * 125}
              r={3}
              fill={
                p.drop ? 'var(--danger, #b94b4b)' : 'var(--accent, #466bd2)'
              }
            />
          ),
        )}
      </svg>
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
            {points.map((p, i) => (
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
                    : [
                        p.gap ? 'Lacuna de continuidade' : '',
                        p.drop ? 'Queda brusca observada' : '',
                        !p.gap && !p.drop ? 'Leitura compatível' : '',
                      ]
                        .filter(Boolean)
                        .join(' · ')}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
function DeviceOperationalTables({ result }: { result: PPReadResult }) {
  const health = result.rows.find((r) => r.kind === 'resource')?.health;
  const authCount = result.rows.find(
    (r) => r.kind === 'availability' && r.name === 'auth',
  )?.used;
  const labels: Record<string, string> = {
    services: 'Serviços telnet/ftp/www',
    packages: 'Pacotes RouterOS',
    interfaces: 'Erros e quedas de interfaces',
    pools: 'Uso de pools',
    auth: 'Falhas de autenticação PPP',
    fasttrack: 'Regras FastTrack',
    ports: 'Negociação de portas físicas',
  };
  return (
    <section className="pppoe-diagnostics" aria-label="Resultado operacional">
      {health && (
        <p>
          Recurso observado entre{' '}
          {health.started_at
            ? formatDate(health.started_at)
            : 'Instante indisponível'}{' '}
          e {health.at ? formatDate(health.at) : 'Instante indisponível'}.
        </p>
      )}
      {health && (
        <dl className="pppoe-diagnostic-values pppoe-context-values">
          <dt>Regras FastTrack habilitadas</dt>
          <dd>{exactNumber(health.fasttrack_rules)}</dd>
          <dt>CPU</dt>
          <dd>{formatMetric(health.cpu, '%')}</dd>
          <dt>Memória livre / total</dt>
          <dd
            title={`${exactVolume(health.free_memory)} / ${exactVolume(health.total_memory)}`}
          >
            {formatVolumePair(health.free_memory, health.total_memory)}
          </dd>
          <dt>Uptime do equipamento</dt>
          <dd title={uptimeTitle(health.uptime)}>
            {formatPPUptime(health.uptime)}
          </dd>
          <dt>RouterOS</dt>
          <dd>{health.version || 'Indisponível'}</dd>
          <dt>Temperatura</dt>
          <dd>
            {health.temperature === undefined
              ? 'Indisponível'
              : formatMetric(health.temperature, '°C')}
          </dd>
          <dt>Diferença relógio roteador × PC</dt>
          <dd>
            {health.clock_offset.known
              ? formatClockOffset(health.clock_offset)
              : 'Indisponível'}
          </dd>
          <dt>NTP</dt>
          <dd>
            {health.ntp_enabled === undefined
              ? 'Indisponível'
              : health.ntp_enabled
                ? 'Ativado'
                : 'Desativado'}{' '}
            · {health.ntp_status || 'Estado indisponível'}
          </dd>
          <dt>Erros no buffer de log</dt>
          <dd>{health.error_count ?? 'Indisponível'}</dd>
        </dl>
      )}
      {health?.fasttrack_rules !== undefined && health.fasttrack_rules > 0 && (
        <p role="status" className="inventory-notice">
          Sem controle de banda: FastTrack ativo
        </p>
      )}
      <p>
        Somente leitura. A auditoria de serviços não verifica “Available From”
        nem altera a configuração.
      </p>
      {Object.entries(labels).map(([group, label]) => (
        <p key={group}>
          {label}:{' '}
          {result.availability?.[group] === 'complete'
            ? 'leitura completa'
            : result.availability?.[group] === 'partial'
              ? 'leitura parcial; há formatos não reconhecidos'
              : result.availability?.[group] === 'limited'
                ? 'limite atingido'
                : 'indisponível'}
          .
        </p>
      ))}
      <p>
        Contadores de interfaces são acumulados; ausência não é zero. Pools em
        80% ou mais são destacados apenas nesta leitura. Falhas de autenticação
        representam o buffer examinado, com os horários informados pelo
        roteador; não são uma janela de 24 h.
      </p>
      {authCount !== undefined && result.availability?.auth === 'complete' && (
        <p>
          {authCount === 0
            ? 'Nenhuma falha de autenticação reconhecida no buffer examinado.'
            : `${authCount} falhas de autenticação reconhecidas no buffer examinado.`}
        </p>
      )}
      <div
        className="table-scroll"
        tabIndex={0}
        aria-label="Tabela operacional"
      >
        <table className="app-table">
          <thead>
            <tr>
              <th>Grupo</th>
              <th>Nome / login</th>
              <th>Leitura</th>
              <th>Origem / estado</th>
            </tr>
          </thead>
          <tbody>
            {result.rows
              .filter((r) => r.kind !== 'resource' && r.kind !== 'availability')
              .map((r, i) => (
                <tr key={i}>
                  <td>
                    {(
                      {
                        service: 'Serviço',
                        package: 'Pacote',
                        interface: 'Interface',
                        port: 'Porta física',
                        pool: 'Pool',
                        auth_failure: 'Falha PPP',
                      } as Record<string, string>
                    )[r.kind] || r.kind}
                  </td>
                  <td>{r.name || r.login || 'Indisponível'}</td>
                  <td>
                    {r.kind === 'service'
                      ? `porta ${r.used ?? 'Indisponível'}`
                      : r.kind === 'port'
                        ? portLabel(r.port)
                        : r.kind === 'package'
                          ? r.value || 'Indisponível'
                          : r.kind === 'interface'
                            ? `RX erros ${exactNumber(r.rx_errors)} · TX erros ${exactNumber(r.tx_errors)} · quedas ${exactNumber(r.link_downs)}`
                            : r.kind === 'pool'
                              ? `${exactNumber(r.used)} / ${exactNumber(r.size)}${r.used !== undefined && r.size !== undefined && r.size > 0 && r.used / r.size >= 0.8 ? ' · ocupação ≥80%' : ''}`
                              : exactNumber(r.used)}
                  </td>
                  <td>
                    {r.kind === 'service' || r.kind === 'package'
                      ? r.disabled === undefined
                        ? 'Estado indisponível'
                        : r.disabled
                          ? 'Desativado'
                          : 'Ativo'
                      : r.kind === 'port'
                        ? `${exactNumber(r.port?.link_downs)} quedas acumuladas${r.port?.running && ((r.port.speed_bps !== undefined && r.port.speed_bps <= 100000000) || r.port.full_duplex === false) ? ' · atenção' : ''}`
                        : r.kind === 'auth_failure'
                          ? `${r.at || 'Horário indisponível'} a ${r.value || 'Horário indisponível'}`
                          : r.value || 'Contador acumulado'}
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function PPTrafficChart({ row }: { row: PPReadRow }) {
  const t = row.traffic;
  if (!t) return <p>Tráfego indisponível para esta sessão.</p>;
  const max = Math.max(
    1,
    ...t.points.flatMap((p) => (p.gap ? [] : [p.d || 0, p.u || 0])),
  );
  const end = Math.max(5000, ...t.points.map((p) => p.t));
  const scale = networkScale(max, 'rate');
  const paths = (field: 'd' | 'u') => {
    const paths: string[] = [];
    let current = '';
    for (const p of t.points) {
      if (p.gap || p[field] === undefined) {
        if (current) paths.push(current);
        current = '';
        continue;
      }
      current +=
        (current ? ' L ' : 'M ') +
        (78 + (p.t / end) * 522).toFixed(1) +
        ' ' +
        (135 - (p[field]! / max) * 125).toFixed(1);
    }
    if (current) paths.push(current);
    return paths;
  };
  return (
    <section aria-label={'Resumo de tráfego de ' + row.login}>
      <h4>{row.login}</h4>
      <p title={exactRate(max)}>
        Escala: 0–{formatAtScale(max, scale)} · download / upload
      </p>
      <svg
        viewBox="0 0 600 150"
        role="img"
        aria-label={'Gráfico de tráfego de ' + row.login}
        style={{ width: '100%', maxHeight: 180 }}
      >
        <title>
          Download TX e upload RX; interrupções são lacunas, sem interpolação.
        </title>
        {[1, 0.5, 0].map((part) => (
          <text
            key={part}
            x="0"
            y={part === 1 ? 12 : part === 0 ? 147 : 77}
            fill="var(--muted)"
            fontSize="11"
          >
            {formatAtScale(max * part, scale)}
          </text>
        ))}
        {paths('d').map((d, i) => (
          <path
            key={'d' + i}
            d={d}
            fill="none"
            stroke="var(--primary)"
            strokeWidth="2"
          />
        ))}
        {paths('u').map((d, i) => (
          <path
            key={'u' + i}
            d={d}
            fill="none"
            stroke="var(--muted)"
            strokeWidth="2"
            strokeDasharray="5 3"
          />
        ))}
      </svg>
      <p>
        Download (TX): <RateValue rate={row.download} /> · Upload (RX):{' '}
        <RateValue rate={row.upload} />
      </p>
      <p>
        Média da observação: download <RateValue rate={t.average_download} /> /
        upload <RateValue rate={t.average_upload} /> · pico amostrado: download{' '}
        <RateValue rate={t.peak_download} /> / upload{' '}
        <RateValue rate={t.peak_upload} />
      </p>
      <p>
        Total {t.gaps ? 'parcial ' : ''}da observação:{' '}
        <span
          title={`${exactVolume(t.download_bytes)} download / ${exactVolume(t.upload_bytes)} upload`}
        >
          {t.covered_ns > 0 && !t.overflow
            ? `${formatVolume(t.download_bytes)} download / ${formatVolume(t.upload_bytes)} upload`
            : 'Indisponível'}
        </span>{' '}
        · tempo coberto{' '}
        <span title={exactNumber(t.covered_ns / 1e9, 's')}>
          {formatDuration(t.covered_ns / 1e9)}
        </span>
        {t.gaps
          ? ` · ${t.gaps} lacuna(s); sem estimar os intervalos ausentes.`
          : ''}
      </p>
      {t.overflow && <p role="alert">Limite numérico: resumo indisponível.</p>}
    </section>
  );
}
