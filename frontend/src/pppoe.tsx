import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import { pppoeAPI as api } from './presentation';
import type {
  PPResult,
  PPRow,
  PPObservation,
  PPReference,
  PPDelta,
  PPQuery,
} from './presentation';
import type { Device } from './equipment-api';
import { equipmentMessage } from './equipment-api';
import './pppoe.css';
import { PPPoEDiagnostics, PPPoEPing } from './pppoe-diagnostics';

const freshQuery = (): PPQuery => ({
  search: '',
  duplicates_only: false,
  sort: 'login',
  desc: false,
  category: '',
});
const message = (r: PPResult) =>
  r.error?.reason === 'operation_receipts'
    ? 'Limite de 2048 operações atingido. Novas atualizações, referências e exclusões estão bloqueadas. Consultar, comparar e exportar continuam disponíveis.'
    : r.error?.reason?.startsWith('references')
      ? 'Limite de referências atingido. Exclua uma referência que não precisa mais.'
      : {
          OBSERVATION_UNAVAILABLE:
            'A leitura exibida não está mais disponível. Abra a leitura atual.',
          OBSERVATION_INCOMPATIBLE:
            'As leituras pertencem a contextos incompatíveis.',
          CURSOR_INVALID: 'A página expirou. Volte ao início da lista.',
        }[r.code] || equipmentMessage(r.code);
const category: Record<string, string> = {
  unchanged: 'Sem alteração',
  changed: 'Campos alterados',
  ambiguous: 'Ambígua / possível reconexão',
  new: 'Somente na atual',
  absent: 'Somente na referência',
};
import {
  pendingPPPoE as pending,
  acceptEquipmentAuthority,
} from './equipment-receipts';
export function PPPoE({
  profile,
  activation,
  device,
  back,
}: {
  profile: string;
  activation: string;
  device: Device;
  back: () => void;
}) {
  useLayoutEffect(() => {
    acceptEquipmentAuthority(profile, activation);
  }, [profile, activation]);
  const [pageState, setPageState] = useState<
    'idle' | 'loading' | 'ready' | 'error'
  >('loading');
  const [statusFailed, setStatusFailed] = useState(false),
    [statusRetry, retryStatus] = useState(0);
  const [pair, setPair] = useState<{ left?: PPRow; right?: PPRow }>();
  const detailTrigger = useRef<HTMLElement | null>(null);
  const detailHeading = useRef<HTMLHeadingElement>(null);
  const pairHeading = useRef<HTMLHeadingElement>(null);
  const focusedControl = useRef<HTMLElement | null>(null);
  const searchControl = useRef<HTMLInputElement>(null);
  const referencesHeading = useRef<HTMLHeadingElement>(null);
  const exportControl = useRef<HTMLButtonElement>(null);
  const removedFocus = useRef('');
  const exportFocus = useRef(false);
  const [referencesState, setReferencesState] = useState<
    'loading' | 'ready' | 'error'
  >('loading');
  const [initialFailed, setInitialFailed] = useState(false);
  const [current, setCurrent] = useState<PPObservation>();
  const [rows, setRows] = useState<PPRow[]>([]),
    [refs, setRefs] = useState<PPReference[]>([]),
    [deltas, setDeltas] = useState<PPDelta[]>([]);
  const [query, setQuery] = useState(freshQuery),
    [cursor, setCursor] = useState(''),
    [next, setNext] = useState('');
  const [selected, select] = useState<PPRow>(),
    [base, setBase] = useState(''),
    [tab, setTab] = useState<'sessions' | 'compare' | 'catalog' | 'radius'>(
      'sessions',
    );
  const [error, setError] = useState(''),
    [notice, setNotice] = useState(''),
    [attempt, setAttempt] = useState(''),
    [job, setJob] = useState('');
  const [busy, setBusy] = useState(false),
    [loading, setLoading] = useState(true),
    [total, setTotal] = useState(0),
    [refresh, bump] = useState(0);
  const life = useRef({ generation: 0 }),
    primary = useRef<HTMLButtonElement>(null);
  useLayoutEffect(() => {
    if (
      removedFocus.current &&
      !refs.some((r) => r.reference_id === removedFocus.current)
    ) {
      removedFocus.current = '';
      referencesHeading.current?.focus();
    }
  }, [refs]);
  useLayoutEffect(() => {
    if (!busy && exportFocus.current) {
      exportFocus.current = false;
      exportControl.current?.focus();
    }
  }, [busy]);
  const context = [profile, activation, device.id].join('|');
  useLayoutEffect(() => {
    if (selected) detailHeading.current?.focus();
  }, [selected]);
  useLayoutEffect(() => {
    if (pair) pairHeading.current?.focus();
  }, [pair]);
  const [cursorBase, setCursorBase] = useState(base);
  if (cursorBase !== base) {
    setCursorBase(base);
    setCursor('');
  }

  const detailIntent = useRef(0);
  const surface = [
    context,
    current?.observation_id,
    tab,
    base,
    JSON.stringify(query),
    cursor,
  ].join('|');
  const [acceptedSurface, setAcceptedSurface] = useState(surface);
  if (acceptedSurface !== surface) {
    setAcceptedSurface(surface);
    select(undefined);
    setPair(undefined);
  }
  useLayoutEffect(() => {
    ++detailIntent.current;
  }, [surface]);
  useEffect(() => {
    const lifecycle = life.current;
    ++lifecycle.generation;
    return () => {
      ++lifecycle.generation;
    };
  }, [context]);
  const valid = useCallback(
    (generation: number, r: PPResult) =>
      generation === life.current.generation &&
      r.profile_id === profile &&
      r.activation_id === activation,
    [profile, activation],
  );
  useEffect(() => {
    const generation = life.current.generation;
    let stopped = false;
    void (async () => {
      setLoading(true);
      setReferencesState('loading');
      setInitialFailed(false);
      const r = await api.current(profile, activation, device.id);
      if (stopped || !valid(generation, r)) return;
      if (r.code !== 'OK' || !r.data) {
        setError(message(r));
        setInitialFailed(true);
        setReferencesState('error');
        setLoading(false);
        return;
      }
      setCurrent(r.data.current);
      const a = r.data.attempt;
      setAttempt(
        a &&
          (a.quality?.status !== 'complete' ||
            a.state === 'FAILED' ||
            a.state === 'CANCELLED')
          ? `Tentativa ${a.quality?.status === 'limited' ? 'limitada' : 'incompleta'}: ${a.quality?.counts_known ? a.quality.eligible + ' elegíveis; ' : ''}${a.code}. ${r.data.current ? 'A tabela continua sendo a última leitura boa, na data indicada.' : 'Nenhuma leitura boa disponível.'}`
          : '',
      );
      const rr = await api.references(profile, activation, device.id, '');
      if (stopped || !valid(generation, rr)) return;
      if (rr.code === 'OK') {
        setRefs(rr.data?.references || []);
        setReferencesState('ready');
      } else {
        setError(message(rr));
        setReferencesState('error');
      }
      setLoading(false);
    })().catch(() => {
      if (!stopped) {
        setError('Serviço indisponível. Tente abrir novamente.');
        setInitialFailed(true);
        setReferencesState('error');
        setLoading(false);
      }
    });
    return () => {
      stopped = true;
    };
  }, [context, refresh, profile, activation, device.id, valid]);
  useEffect(() => {
    const generation = life.current.generation;
    let stopped = false;
    void (async () => {
      setRows([]);
      setDeltas([]);
      setNext('');
      setTotal(0);
      if (
        !current ||
        (tab !== 'sessions' && tab !== 'compare') ||
        (tab === 'compare' && !base)
      ) {
        setPageState('idle');
        return;
      }
      setPageState('loading');
      const r =
        tab === 'sessions'
          ? await api.list(
              profile,
              activation,
              device.id,
              current.observation_id,
              query,
              cursor,
            )
          : await api.compare(
              profile,
              activation,
              device.id,
              base,
              current.observation_id,
              query,
              cursor,
            );
      if (stopped || !valid(generation, r)) return;
      if (r.code !== 'OK') {
        setError(message(r));
        setPageState('error');
        setRows([]);
        setDeltas([]);
        setNext('');
        return;
      }
      setPageState('ready');
      setRows(r.data?.rows || []);
      setDeltas(r.data?.deltas || []);
      setNext(r.data?.next_cursor || '');
      setTotal(r.data?.total || 0);
    })().catch(() => {
      if (!stopped) {
        setPageState('error');
        setNext('');
        setTotal(0);
        setError('Não foi possível carregar a página.');
      }
    });
    return () => {
      stopped = true;
    };
  }, [
    context,
    current,
    query,
    cursor,
    tab,
    base,
    profile,
    activation,
    device.id,
    valid,
  ]);
  async function run(
    action: () => Promise<PPResult>,
    done?: (r: PPResult) => void,
  ) {
    const generation = life.current.generation;
    const trigger = focusedControl.current;
    setBusy(true);
    setError('');
    setNotice('');
    try {
      const r = await action();
      if (!valid(generation, r)) return;
      if (r.code !== 'OK') {
        setError(message(r));
        if (
          [
            'JOB_INTERRUPTED',
            'REVISION_CONFLICT',
            'DEVICE_INACTIVE',
            'TRUST_REQUIRED',
          ].includes(r.code)
        )
          pending.delete(context + '|collect|' + device.revision);
      } else done?.(r);
    } catch {
      if (generation === life.current.generation)
        setError(
          'Resposta incerta. A ação mantém o mesmo identificador para reconciliação.',
        );
    } finally {
      if (generation === life.current.generation) {
        setBusy(false);
        trigger?.focus();
      }
    }
  }
  // Detail queries are disposable intents. Durable mutations keep their own
  // receipts and lifecycle; changing a filter must never discard those receipts.
  async function runDetail(
    action: (accept: () => boolean) => Promise<PPResult>,
    done: (r: PPResult) => void,
  ) {
    const generation = life.current.generation;
    const intent = ++detailIntent.current;
    const accept = () =>
      generation === life.current.generation && intent === detailIntent.current;
    setError('');
    try {
      const r = await action(accept);
      if (!accept() || !valid(generation, r)) return;
      if (r.code !== 'OK') setError(message(r));
      else done(r);
    } catch {
      if (accept())
        setError('Não foi possível carregar o detalhe. Tente novamente.');
    }
  }
  function update() {
    const k = context + '|collect|' + device.revision;
    for (const [key, receipt] of pending)
      if (
        key.startsWith(context + '|collect|') &&
        receipt.revision !== undefined &&
        Number.isSafeInteger(receipt.revision) &&
        Number.isSafeInteger(device.revision) &&
        receipt.revision < device.revision
      )
        pending.delete(key);
    if (!pending.has(k) && pending.size >= 32) {
      setError(
        'Há 32 ações com resposta incerta. Reconcilie uma delas no contexto original.',
      );
      return;
    }
    const p = pending.get(k) || {
      operation: crypto.randomUUID(),
      revision: device.revision,
    };
    pending.set(k, p);
    void run(
      () =>
        api.start(profile, activation, device.id, p.operation, device.revision),
      (r) => {
        if (r.data?.job) setJob(r.data.job.id);
      },
    );
  }
  useEffect(() => {
    if (!job) return;
    let stopped = false;
    const generation = life.current.generation;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      setStatusFailed(false);
      try {
        const r = await api.status(profile, activation, job);
        if (stopped || !valid(generation, r)) return;
        if (r.code !== 'OK') {
          setError(message(r));
          setStatusFailed(true);
          return;
        }
        if (r.data?.job?.state === 'RUNNING')
          timer = setTimeout(() => void poll(), 150);
        else {
          pending.delete(context + '|collect|' + device.revision);
          setJob('');
          setError(
            r.data?.job?.code === 'OK'
              ? ''
              : equipmentMessage(r.data?.job?.code || 'SERVICE_UNAVAILABLE'),
          );
          setCursor('');
          select(undefined);
          bump((n) => n + 1);
          primary.current?.focus();
        }
      } catch {
        if (!stopped) {
          setStatusFailed(true);
          setError(
            'Não foi possível consultar o estado. A atualização não será repetida automaticamente.',
          );
        }
      }
    };
    void poll();
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [job, context, statusRetry, profile, activation, device.revision, valid]);
  function save() {
    if (!current) return;
    const k = context + '|save|' + current.observation_id;
    if (!pending.has(k) && pending.size >= 32) {
      setError('Limite de ações incertas; reconcilie no contexto original.');
      return;
    }
    const p = pending.get(k) || {
      operation: crypto.randomUUID(),
      reference: crypto.randomUUID(),
    };
    pending.set(k, p);
    void run(
      () =>
        api.save(
          profile,
          activation,
          device.id,
          current.observation_id,
          p.reference!,
          p.operation,
        ),
      (r) => {
        pending.delete(k);
        setNotice(
          r.data?.effect_state === 'removed'
            ? 'Esta referência já foi removida. O replay não a recriou.'
            : 'Referência salva da leitura exibida.',
        );
        bump((n) => n + 1);
      },
    );
  }
  function remove(ref: PPReference) {
    const k = context + '|delete|' + ref.reference_id;
    if (!pending.has(k) && pending.size >= 32) {
      setError('Limite de ações incertas; reconcilie no contexto original.');
      return;
    }
    const p = pending.get(k) || { operation: crypto.randomUUID() };
    pending.set(k, p);
    void run(
      () =>
        api.remove(
          profile,
          activation,
          device.id,
          ref.reference_id,
          p.operation,
        ),
      () => {
        pending.delete(k);
        removedFocus.current = ref.reference_id;
        setBase((selectedBase) =>
          selectedBase === ref.observation_id ? '' : selectedBase,
        );
        bump((n) => n + 1);
      },
    );
  }
  function change(q: Partial<PPQuery>) {
    setQuery((v) => ({ ...v, ...q }));
    setCursor('');
    select(undefined);
    setPair(undefined);
  }
  async function openPair(d: PPDelta, trigger: HTMLElement) {
    detailTrigger.current = trigger;
    let left: PPRow | undefined;
    let right: PPRow | undefined;
    await runDetail(
      async (accept) => {
        let r: PPResult = {
          code: 'OK',
          profile_id: profile,
          activation_id: activation,
        };
        if (d.base_row) {
          r = await api.detail(
            profile,
            activation,
            device.id,
            base,
            d.base_row,
          );
          if (!accept() || r.code !== 'OK') return r;
          left = r.data?.rows?.[0];
        }
        if (d.target_row) {
          r = await api.detail(
            profile,
            activation,
            device.id,
            current!.observation_id,
            d.target_row,
          );
          if (r.code !== 'OK') return r;
          right = r.data?.rows?.[0];
        }
        return r;
      },
      () => {
        select(undefined);
        setPair({ left, right });
      },
    );
  }
  const disabled = busy || Boolean(job);
  return (
    <section
      className="pppoe-workspace"
      onFocusCapture={(event) => {
        focusedControl.current = event.target;
      }}
      onKeyDown={(event) => {
        if (event.key === 'Escape' && (selected || pair)) {
          ++detailIntent.current;
          select(undefined);
          setPair(undefined);
          if (detailTrigger.current?.isConnected) detailTrigger.current.focus();
          event.preventDefault();
        }
      }}
      aria-label="Diagnóstico PPPoE"
      tabIndex={0}
    >
      <button onClick={back}>Voltar ao equipamento</button>
      <div className="pppoe-head">
        <div>
          <h1>Sessões PPPoE</h1>
          <p>{device.name}</p>
        </div>
        {!initialFailed && (tab === 'sessions' || tab === 'compare') && (
          <button
            ref={primary}
            className="primary"
            disabled={
              disabled || loading || !device.active || !device.fingerprint
            }
            onClick={update}
          >
            {job ? 'Atualizando…' : 'Atualizar sessões'}
          </button>
        )}
      </div>
      {job && (
        <button
          onClick={() => void run(() => api.cancel(profile, activation, job))}
        >
          Cancelar atualização
        </button>
      )}
      {statusFailed && job && (
        <button onClick={() => retryStatus((n) => n + 1)}>
          Consultar estado novamente
        </button>
      )}
      {error && (tab === 'sessions' || tab === 'compare') && (
        <p role="alert">{error}</p>
      )}
      {notice && <p role="status">{notice}</p>}
      {attempt && (tab === 'sessions' || tab === 'compare') && (
        <p role="status">{attempt}</p>
      )}
      {initialFailed &&
        !loading &&
        (tab === 'sessions' || tab === 'compare') && (
          <button
            className="primary"
            onClick={() => {
              setError('');
              bump((n) => n + 1);
            }}
          >
            Reabrir leitura armazenada
          </button>
        )}
      {loading && (tab === 'sessions' || tab === 'compare') && (
        <p role="status">Carregando leitura armazenada…</p>
      )}
      {!loading &&
        !initialFailed &&
        !current &&
        (tab === 'sessions' || tab === 'compare') && (
          <p>
            Nenhuma leitura de sessões disponível. Atualize para consultar este
            concentrador.
          </p>
        )}
      {current && (tab === 'sessions' || tab === 'compare') && (
        <>
          <p>
            Última leitura boa:{' '}
            {new Date(current.provenance.ended_at).toLocaleString('pt-BR')} ·{' '}
            {current.count} sessões · SSH · {current.quality.status}. Leitura
            por janela, sem snapshot remoto atômico.
          </p>
          <div className="pppoe-actions">
            <button disabled={disabled} onClick={save}>
              Salvar referência desta leitura
            </button>
            <button
              ref={exportControl}
              disabled={busy}
              onClick={() => {
                exportFocus.current = true;
                setNotice('');
                setError('');
                const g = life.current.generation;
                setBusy(true);
                void api
                  .export(
                    profile,
                    activation,
                    device.id,
                    current.observation_id,
                    tab === 'compare' ? base : '',
                  )
                  .then((code) => {
                    if (g === life.current.generation) {
                      setNotice(
                        code === 'OK'
                          ? 'Arquivo sanitizado salvo.'
                          : code === 'EXPORT_CANCELLED'
                            ? ''
                            : 'Exportação: ' + code,
                      );
                    }
                  })
                  .catch(() => {
                    if (g === life.current.generation)
                      setError('Falha na exportação.');
                  })
                  .finally(() => {
                    if (g === life.current.generation) setBusy(false);
                  });
              }}
            >
              Exportar JSON sanitizado
            </button>
          </div>
        </>
      )}
      <div role="tablist" aria-label="Leitura PPPoE">
        {(
          [
            ['sessions', 'Sessões'],
            ['compare', 'Comparação'],
            ['catalog', 'VLANs e serviços'],
            ['radius', 'RADIUS'],
          ] as const
        ).map(([id, label]) => (
          <button
            key={id}
            role="tab"
            aria-selected={tab === id}
            onClick={() => {
              setTab(id);
              setCursor('');
            }}
          >
            {label}
          </button>
        ))}
      </div>
      {(tab === 'catalog' || tab === 'radius') && (
        <PPPoEDiagnostics
          key={context + '|' + tab}
          profile={profile}
          activation={activation}
          device={device}
          scope={tab}
          blocked={disabled}
        />
      )}
      {current && (tab === 'sessions' || tab === 'compare') && (
        <>
          {tab === 'sessions' ? (
            <>
              <div className="pppoe-filters">
                <label>
                  Buscar sessões
                  <input
                    ref={searchControl}
                    value={query.search}
                    onChange={(e) => change({ search: e.target.value })}
                  />
                </label>
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
                    onChange={(e) =>
                      change({ duplicates_only: e.target.checked })
                    }
                  />
                  Somente logins duplicados
                </label>
                {(query.search || query.duplicates_only) && (
                  <button
                    onClick={() => {
                      setQuery(freshQuery());
                      setCursor('');
                      searchControl.current?.focus();
                    }}
                  >
                    Limpar filtros
                  </button>
                )}
              </div>
              {pageState === 'loading' && (
                <p role="status">Carregando página armazenada…</p>
              )}
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
                      <th>Duplicidade</th>
                      <th>Detalhe</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((row) => (
                      <tr key={row.row_id}>
                        <td>{row.login || 'Indisponível'}</td>
                        <td>{row.address || 'Indisponível'}</td>
                        <td>
                          {row.duplicates > 1
                            ? row.duplicates + ' sessões do mesmo login'
                            : '—'}
                        </td>
                        <td>
                          <button
                            aria-label={
                              'Detalhar ' +
                              row.login +
                              ' ' +
                              row.row_id.slice(0, 8)
                            }
                            onClick={(event) => {
                              detailTrigger.current = event.currentTarget;
                              void runDetail(
                                () =>
                                  api.detail(
                                    profile,
                                    activation,
                                    device.id,
                                    current.observation_id,
                                    row.row_id,
                                  ),
                                (r) => {
                                  setPair(undefined);
                                  select(r.data?.rows?.[0]);
                                },
                              );
                            }}
                          >
                            Detalhar
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {pageState === 'ready' && !rows.length && (
                <p>
                  {current.count === 0
                    ? 'Nenhuma sessão ativa nesta leitura completa.'
                    : 'Nenhuma sessão corresponde aos filtros.'}
                </p>
              )}
              {selected && (
                <aside aria-label="Detalhes da sessão">
                  <h2 tabIndex={-1} ref={detailHeading}>
                    {selected.login}
                  </h2>
                  <p>IP: {selected.address || 'Indisponível'}</p>
                  <p>Caller ID: {selected.caller_id || 'Indisponível'}</p>
                  <p>
                    Tempo conectado:{' '}
                    {selected.uptime.known
                      ? Math.floor(selected.uptime.min / 1e9) + ' s'
                      : 'Indisponível'}
                  </p>
                  <p>Identidade local da linha: {selected.row_id}</p>
                  <PPPoEPing
                    key={current.observation_id + '|' + selected.row_id}
                    profile={profile}
                    activation={activation}
                    device={device}
                    observation={current.observation_id}
                    row={selected.row_id}
                    blocked={disabled}
                  />
                  <button
                    onClick={() => {
                      ++detailIntent.current;
                      select(undefined);
                      detailTrigger.current?.focus();
                    }}
                  >
                    Fechar detalhe
                  </button>
                </aside>
              )}
            </>
          ) : (
            <>
              <label>
                Referência para comparar
                <select
                  value={base}
                  onChange={(e) => {
                    setBase(e.target.value);
                    setCursor('');
                  }}
                >
                  <option value="">Selecione uma referência</option>
                  {refs.map((ref) => (
                    <option key={ref.reference_id} value={ref.observation_id}>
                      {new Date(ref.created_at).toLocaleString('pt-BR')} ·{' '}
                      {ref.count} sessões
                    </option>
                  ))}
                </select>
              </label>
              <p>
                Ausência nesta leitura não comprova instante ou causa de queda.
                IDs e logins iguais podem ser ambíguos após reboot, reuso ou
                lacuna.
              </p>
              <label>
                Categoria
                <select
                  value={query.category}
                  onChange={(e) => change({ category: e.target.value })}
                >
                  <option value="">Todas</option>
                  {Object.entries(category).map(([id, label]) => (
                    <option key={id} value={id}>
                      {label}
                    </option>
                  ))}
                </select>
              </label>
              {pageState === 'loading' && (
                <p role="status">Carregando comparação…</p>
              )}
              {deltas.map((d, i) => (
                <div key={d.base_row + '|' + d.target_row + '|' + i}>
                  <span>
                    {d.base_row?.slice(0, 8) || '—'} →{' '}
                    {d.target_row?.slice(0, 8) || '—'} · {category[d.category]}
                  </span>
                  {d.address_changed ? ' · IP alterado' : ''}
                  {d.caller_changed ? ' · Caller ID alterado' : ''}
                  <button onClick={(e) => void openPair(d, e.currentTarget)}>
                    Detalhar par
                  </button>
                </div>
              ))}
            </>
          )}
          {pair && (
            <aside aria-label="Detalhes da comparação">
              <h2 tabIndex={-1} ref={pairHeading}>
                Sessões do par
              </h2>
              {(['left', 'right'] as const).map((side) => (
                <div key={side}>
                  <h3>{side === 'left' ? 'Referência' : 'Atual'}</h3>
                  {pair[side] ? (
                    <>
                      <p>Login: {pair[side]!.login}</p>
                      <p>IP: {pair[side]!.address || 'Indisponível'}</p>
                      <p>
                        Caller ID: {pair[side]!.caller_id || 'Indisponível'}
                      </p>
                      <p>Linha: {pair[side]!.row_id}</p>
                    </>
                  ) : (
                    <p>Ausente neste conjunto.</p>
                  )}
                </div>
              ))}
              <button
                onClick={() => {
                  ++detailIntent.current;
                  setPair(undefined);
                  detailTrigger.current?.focus();
                }}
              >
                Fechar par
              </button>
            </aside>
          )}
          <div className="pppoe-actions">
            {pageState === 'ready' && (
              <span role="status" aria-label={`${total} resultados`}>
                {total} resultados
              </span>
            )}
            {cursor && (
              <button
                onClick={() => {
                  setCursor('');
                  searchControl.current?.focus();
                }}
              >
                Início da lista
              </button>
            )}
            {pageState === 'ready' && next && (
              <button
                onClick={() => {
                  setCursor(next);
                  select(undefined);
                  searchControl.current?.focus();
                }}
              >
                Próxima página
              </button>
            )}
          </div>
        </>
      )}
      {(tab === 'sessions' || tab === 'compare') && (
        <>
          <h2 tabIndex={-1} ref={referencesHeading}>
            Referências preservadas
          </h2>
          {referencesState === 'loading' && (
            <p role="status">Carregando referências…</p>
          )}
          {referencesState === 'error' && !initialFailed && (
            <button
              onClick={() => {
                setError('');
                bump((n) => n + 1);
              }}
            >
              Recarregar referências
            </button>
          )}
          {referencesState === 'ready' && !refs.length && (
            <p>Nenhuma referência salva.</p>
          )}
          {refs.map((ref) => (
            <div key={ref.reference_id} className="pppoe-reference">
              <span>
                {new Date(ref.created_at).toLocaleString('pt-BR')} · {ref.count}{' '}
                sessões
              </span>
              <button
                disabled={disabled}
                aria-label={
                  'Excluir referência ' + ref.reference_id.slice(0, 8)
                }
                onClick={() => remove(ref)}
              >
                Excluir referência
              </button>
            </div>
          ))}
        </>
      )}
    </section>
  );
}
