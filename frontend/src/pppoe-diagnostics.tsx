import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import { pppoeDiagnosticAPI as api } from './presentation';
import type { PDDiagnostic, PDRow, PPResult } from './presentation';
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
          latest?.state === 'RUNNING' && latest.job_id
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
