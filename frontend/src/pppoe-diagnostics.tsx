import {
  Fragment,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import {
  pppoeReadAPI as readAPI,
  pppoeDiagnosticAPI as api,
} from './presentation';
import type {
  PDDiagnostic,
  PDRow,
  PPResult,
  PPReadResult,
  PPRate,
} from './presentation';
import { formatPPUptime } from './pppoe-format';
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
const latency = (ns?: number) =>
  ns === undefined
    ? 'Indisponível'
    : (ns / 1e6).toLocaleString('pt-BR', { maximumFractionDigits: 6 }) + ' ms';
const relation: Record<string, string> = {
  resolved: 'Pai identificado',
  missing: 'Pai ausente',
  ambiguous: 'Vínculo ambíguo',
  cycle: 'Ciclo na configuração',
  depth_limited: 'Encadeamento excede o limite',
};

function DiagnosticRow({ row }: { row: PDRow }) {
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
                <dd>{value}</dd>
              </div>
            ))}
            <div>
              <dt>RTT da última solicitação</dt>
              <dd>
                {row.counters.last_rtt.known
                  ? latency(row.counters.last_rtt.min)
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
            Última leitura boa:{' '}
            {new Date(view.meta.provenance.ended_at).toLocaleString('pt-BR')} ·
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
              de VLAN comprovado para as sessões.
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
          {view.rows.map((row) => (
            <DiagnosticRow key={row.id} row={row} />
          ))}
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
            {new Date(view.meta.provenance.ended_at).toLocaleString('pt-BR')}
          </p>
          <p>
            Enviados: {result.sent} · Recebidos: {result.received} · Perda:{' '}
            {result.loss_percent.toLocaleString('pt-BR', {
              maximumFractionDigits: 2,
            })}
            %
          </p>
          <p>
            RTT mínimo: {latency(result.min_ns)} · médio:{' '}
            {latency(result.avg_ns)} · máximo: {latency(result.max_ns)}
          </p>
          <ol>
            {result.probes.map((probe) => (
              <li key={probe.sequence} value={probe.sequence + 1}>
                {probe.status || 'Resposta ICMP'}
                {probe.rtt_ns !== undefined
                  ? ` · ${latency(probe.rtt_ns)}`
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

const formatRate = (rate?: PPRate) => {
  if (!rate) return 'Indisponível';
  const scale = rate.max >= 1e6 ? 1e6 : rate.max >= 1e3 ? 1e3 : 1;
  const unit = scale === 1e6 ? 'Mbit/s' : scale === 1e3 ? 'kbit/s' : 'bit/s';
  const f = (n: number) =>
    new Intl.NumberFormat('pt-BR', { maximumFractionDigits: 2 }).format(n);
  const low = Math.floor((rate.min / scale) * 100) / 100,
    high = Math.ceil((rate.max / scale) * 100) / 100;
  return low === high ? `${f(low)} ${unit}` : `${f(low)}–${f(high)} ${unit}`;
};

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
    [result, setResult] = useState<PPReadResult>(),
    [scope, setScope] = useState('bandwidth');
  const [pollRetry, retryPoll] = useState(0),
    [deadlineReached, setDeadlineReached] = useState(false),
    [sampleAge, setSampleAge] = useState(0);
  const generation = useRef(0),
    activeJob = useRef(''),
    privateResultJob = useRef(''),
    pendingOperation = useRef('');
  const liveStarted = useRef(0),
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
    setStarting(true);
    setError('');
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
        mode === 'top' || mode === 'health' ? '' : observation,
        mode === 'top' || mode === 'health'
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
      if (mode === 'secret') privateResultJob.current = r.data.job.id;
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
  useEffect(() => {
    if (!job) return;
    let alive = true;
    const g = generation.current;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      try {
        const r = await readAPI.status(profile, activation, job);
        if (!alive || g !== generation.current) return;
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
        if (r.data.job.code !== 'OK') setError(ppReadMessage(r.data.job.code));
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
  }, [job, profile, activation, pollRetry]);
  useEffect(() => {
    if (!job || scope !== 'live') return;
    const tick = () => {
      const now = performance.now();
      setSampleAge(Math.max(0, now - sampleClock.current));
      if (now - liveStarted.current >= 60000) {
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
        ? await readAPI.cancel(profile, activation, id)
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
  if (!rows.length && !allowTop && !logRow && !health) return null;
  const locked =
    blocked || starting || pending || Boolean(job) || cancelling || pageBusy;
  const calculated =
    Boolean(job) && scope === 'live' && !error && !deadlineReached;
  return (
    <section
      className="pppoe-diagnostics pppoe-read-panel"
      aria-label={health ? 'Pools e interfaces estáticas' : 'Banda por sessão'}
    >
      <h3>{health ? 'Pools e interfaces estáticas' : 'Banda por sessão'}</h3>
      {health && (
        <button disabled={locked} onClick={() => void start('health')}>
          Consultar pools e interfaces
        </button>
      )}
      {rows.length > 0 && (
        <button disabled={locked} onClick={() => void start('bandwidth')}>
          {rows.length === 1
            ? 'Medir banda do assinante'
            : 'Medir banda da página'}
        </button>
      )}
      {rows.length > 0 && (
        <button disabled={locked} onClick={() => void start('live')}>
          Observar ao vivo por até 60 s
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
          {!job
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
            {scope === 'live'
              ? 'Observação limitada a 60 s; atualização aproximada a cada 2 s.'
              : scope === 'log'
                ? 'Consultando eventos PPP deste login…'
                : scope === 'secret'
                  ? 'Consultando evidência efêmera do secret…'
                  : 'Aguardando duas amostras do concentrador…'}
          </p>
        )
      )}
      {error && <p role="alert">{error}</p>}
      {result && (
        <>
          <p>
            {error || deadlineReached ? 'Último resultado de ' : 'Leitura de '}
            {new Date(result.ended_at).toLocaleString('pt-BR')} · duração{' '}
            {(result.duration_ns / 1e9).toFixed(2)} s
          </p>
          {result.scope === 'top' && (
            <p>
              Consumo ordenado pelo limite inferior de download + upload.
              Valores indisponíveis aparecem ao final.
            </p>
          )}
          {result.scope !== 'log' &&
            result.scope !== 'secret' &&
            result.cost && (
              <p>
                {result.cost.ssh_commands} comandos SSH ·{' '}
                {result.cost.first_tunnels}/{result.cost.last_tunnels}{' '}
                {result.scope === 'health' ? 'interfaces estáticas' : 'túneis'}{' '}
                nas amostras · {result.cost.ssh_bytes} bytes
                {result.cost.cpu_start !== undefined &&
                result.cost.cpu_end !== undefined
                  ? ` · CPU observada: ${result.cost.cpu_start}% / ${result.cost.cpu_end}%`
                  : ''}
              </p>
            )}
          {result.scope === 'log' && (
            <p>
              Até 50 eventos PPP, sem depuração. Horários informados pelo
              equipamento; categorias não comprovam a causa da falha.
            </p>
          )}
          {result.scope === 'secret' ? (
            <PPSecretEvidence result={result} />
          ) : result.scope === 'health' ? (
            <PPHealthTables result={result} />
          ) : result.rows.length === 0 ? (
            <p>
              {result.scope === 'log'
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
                      <td>{formatRate(r.download)}</td>
                      <td>{formatRate(r.upload)}</td>
                      <td>
                        {formatPPUptime(
                          r.uptime.known && calculated
                            ? {
                                ...r.uptime,
                                min: r.uptime.min + sampleAge * 1e6,
                                max: r.uptime.max + sampleAge * 1e6,
                              }
                            : r.uptime,
                        )}
                        {calculated ? ' (calculado)' : ''}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          {next && (
            <button disabled={pageBusy} onClick={() => void page(next)}>
              {result.scope === 'health'
                ? 'Próxima página de pools e interfaces'
                : result.scope === 'log'
                  ? 'Próximos eventos PPP'
                  : 'Próxima página de consumo'}
            </button>
          )}
          {resultJob &&
            (result.scope === 'top' ||
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
                  <td>{formatRate(r.tx)}</td>
                  <td>{formatRate(r.rx)}</td>
                  <td>{flag(r.running)}</td>
                  <td>{flag(r.disabled)}</td>
                  <td>{r.rx_errors ?? 'Indisponível'}</td>
                  <td>{r.tx_errors ?? 'Indisponível'}</td>
                  <td>{r.rx_drops ?? 'Indisponível'}</td>
                  <td>{r.tx_drops ?? 'Indisponível'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
