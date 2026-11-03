import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import type { Device } from './equipment-api';
import { equipmentMessage } from './equipment-api';
import {
  acceptEquipmentAuthority,
  pendingPPPoE as pending,
} from './equipment-receipts';
import { useEquipmentSnapshot } from './equipment-snapshot';
import {
  createOpenPairHandler,
  createRemoveHandler,
  createSaveHandler,
  createUpdateHandler,
  FRESH_QUERY,
  pppoeErrorMessage,
} from './pppoe-context';
import type {
  PPDelta,
  PPObservation,
  PPQuery,
  PPReference,
  PPResult,
  PPRow,
} from './presentation';
import { pppoeAPI as api } from './presentation';

export type PPPoETab =
  'sessions' | 'compare' | 'references' | 'catalog' | 'radius' | 'monitoring';

function formatAttemptStatus(r: PPResult): string {
  const a = r.data?.attempt;
  if (
    !a ||
    (a.quality?.status === 'complete' &&
      a.state !== 'FAILED' &&
      a.state !== 'CANCELLED')
  ) {
    return '';
  }
  const kind = a.quality?.status === 'limited' ? 'limitada' : 'incompleta';
  const eligible = a.quality?.counts_known
    ? `${a.quality.eligible} elegíveis; `
    : '';
  const suffix = r.data?.current
    ? 'A tabela continua sendo a última leitura boa, na data indicada.'
    : 'Nenhuma leitura boa disponível.';
  return `Tentativa ${kind}: ${eligible}${equipmentMessage(a.code || 'SERVICE_UNAVAILABLE')} ${suffix}`;
}

function usePPPoERefs() {
  const detailTrigger = useRef<HTMLElement | null>(null);
  const detailHeading = useRef<HTMLHeadingElement>(null);
  const pairHeading = useRef<HTMLHeadingElement>(null);
  const focusedControl = useRef<HTMLElement | null>(null);
  const searchControl = useRef<HTMLInputElement>(null);
  const referencesHeading = useRef<HTMLHeadingElement>(null);
  const exportControl = useRef<HTMLButtonElement>(null);
  const csvExportControl = useRef<HTMLButtonElement>(null);
  const removedFocus = useRef('');
  const exportFocus = useRef<'sanitized' | 'operational' | ''>('');
  const life = useRef({ generation: 0 });
  const primary = useRef<HTMLButtonElement>(null);
  const detailIntent = useRef(0);
  return {
    detailTrigger,
    detailHeading,
    pairHeading,
    focusedControl,
    searchControl,
    referencesHeading,
    exportControl,
    csvExportControl,
    removedFocus,
    exportFocus,
    life,
    primary,
    detailIntent,
  };
}

function usePPPoEViewState(profile: string, activation: string) {
  useLayoutEffect(() => {
    acceptEquipmentAuthority(profile, activation);
  }, [profile, activation]);
  const r = usePPPoERefs();
  const [pageState, setPageState] = useState<
    'idle' | 'loading' | 'ready' | 'error'
  >('loading');
  const [statusFailed, setStatusFailed] = useState(false);
  const [statusRetry, retryStatus] = useState(0);
  const [detailTab, setDetailTab] = useState<'traffic' | 'information'>(
    'traffic',
  );
  const [pair, setPair] = useState<{ left?: PPRow; right?: PPRow }>();
  const [referencesState, setReferencesState] = useState<
    'loading' | 'ready' | 'error'
  >('loading');
  const [initialFailed, setInitialFailed] = useState(false);
  const [current, setCurrent] = useState<PPObservation>();
  const [rows, setRows] = useState<PPRow[]>([]);
  const [refs, setRefs] = useState<PPReference[]>([]);
  const [deltas, setDeltas] = useState<PPDelta[]>([]);
  const [query, setQuery] = useState(FRESH_QUERY);
  const [cursor, setCursor] = useState('');
  const [next, setNext] = useState('');
  const [showPlan, setShowPlan] = useState(false);
  const [history, setHistory] = useState<string[]>([]);
  const [selected, select] = useState<PPRow>();
  const [base, setBase] = useState('');
  const [tab, setTab] = useState<PPPoETab>('sessions');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [attempt, setAttempt] = useState('');
  const [job, setJob] = useState('');
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [refresh, bump] = useState(0);
  return {
    ...r,
    pageState,
    setPageState,
    statusFailed,
    setStatusFailed,
    statusRetry,
    retryStatus,
    detailTab,
    setDetailTab,
    pair,
    setPair,
    referencesState,
    setReferencesState,
    initialFailed,
    setInitialFailed,
    current,
    setCurrent,
    rows,
    setRows,
    refs,
    setRefs,
    deltas,
    setDeltas,
    query,
    setQuery,
    cursor,
    setCursor,
    next,
    setNext,
    showPlan,
    setShowPlan,
    history,
    setHistory,
    selected,
    select,
    base,
    setBase,
    tab,
    setTab,
    error,
    setError,
    notice,
    setNotice,
    attempt,
    setAttempt,
    job,
    setJob,
    busy,
    setBusy,
    loading,
    setLoading,
    total,
    setTotal,
    refresh,
    bump,
  };
}

type ViewState = ReturnType<typeof usePPPoEViewState>;
type SharedCtx = ViewState & {
  context: string;
  profile: string;
  activation: string;
  device: Device;
  valid: (g: number, r: PPResult) => boolean;
};

function usePPPoEInitialLoad(p: SharedCtx) {
  const { context, refresh, profile, activation, device, valid } = p;
  useEffect(() => {
    const generation = p.life.current.generation;
    let stopped = false;
    void (async () => {
      p.setLoading(true);
      p.setReferencesState('loading');
      p.setInitialFailed(false);
      const r = await api.current(profile, activation, device.id);
      if (stopped || !valid(generation, r)) return;
      if (r.code !== 'OK' || !r.data) {
        p.setError(pppoeErrorMessage(r));
        p.setInitialFailed(true);
        p.setReferencesState('error');
        p.setLoading(false);
        return;
      }
      p.setCurrent(r.data.current);
      p.setAttempt(formatAttemptStatus(r));
      const rr = await api.references(profile, activation, device.id, '');
      if (stopped || !valid(generation, rr)) return;
      if (rr.code === 'OK') {
        p.setRefs(rr.data?.references || []);
        p.setReferencesState('ready');
      } else {
        p.setError(pppoeErrorMessage(rr));
        p.setReferencesState('error');
      }
      p.setLoading(false);
    })().catch(() => {
      if (!stopped) {
        p.setError('Serviço indisponível. Tente abrir novamente.');
        p.setInitialFailed(true);
        p.setReferencesState('error');
        p.setLoading(false);
      }
    });
    return () => {
      stopped = true;
    };
  }, [context, refresh, profile, activation, device.id, valid]);
}

function usePPPoEPageLoad(p: SharedCtx) {
  const { context, current, query, cursor, tab, base, profile, activation } = p;
  useEffect(() => {
    const generation = p.life.current.generation;
    let stopped = false;
    void (async () => {
      p.setRows([]);
      p.setDeltas([]);
      p.setNext('');
      p.setTotal(0);
      if (
        !current ||
        (tab !== 'sessions' && tab !== 'compare') ||
        (tab === 'compare' && !base)
      ) {
        p.setPageState('idle');
        return;
      }
      p.setPageState('loading');
      const r =
        tab === 'sessions'
          ? await api.list(
              profile,
              activation,
              p.device.id,
              current.observation_id,
              query,
              cursor,
            )
          : await api.compare(
              profile,
              activation,
              p.device.id,
              base,
              current.observation_id,
              query,
              cursor,
            );
      if (stopped || !p.valid(generation, r)) return;
      if (r.code !== 'OK') {
        p.setError(pppoeErrorMessage(r));
        p.setPageState('error');
        return;
      }
      p.setPageState('ready');
      p.setRows(r.data?.rows || []);
      p.setDeltas(r.data?.deltas || []);
      p.setNext(r.data?.next_cursor || '');
      p.setTotal(r.data?.total || 0);
    })().catch(() => {
      if (!stopped) {
        p.setPageState('error');
        p.setError('Não foi possível carregar a página.');
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
    p.device.id,
    p.valid,
  ]);
}

function usePPPoEJobPoll(p: SharedCtx) {
  const { job, context, statusRetry, profile, activation, device } = p;
  useEffect(() => {
    if (!job) return;
    let stopped = false;
    const generation = p.life.current.generation;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      p.setStatusFailed(false);
      try {
        const r = await api.status(profile, activation, job);
        if (stopped || !p.valid(generation, r)) return;
        if (r.code !== 'OK') {
          p.setError(pppoeErrorMessage(r));
          p.setStatusFailed(true);
          return;
        }
        if (r.data?.job?.state === 'RUNNING') {
          timer = setTimeout(() => void poll(), 150);
        } else {
          pending.delete(context + '|collect|' + device.revision);
          p.setJob('');
          p.setError(
            r.data?.job?.code === 'OK'
              ? ''
              : equipmentMessage(r.data?.job?.code || 'SERVICE_UNAVAILABLE'),
          );
          p.setCursor('');
          p.setHistory([]);
          p.select(undefined);
          p.bump((n) => n + 1);
          p.primary.current?.focus();
        }
      } catch {
        if (!stopped) {
          p.setStatusFailed(true);
          p.setError(
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
  }, [
    job,
    context,
    statusRetry,
    profile,
    activation,
    device.revision,
    p.valid,
  ]);
}

function usePPPoEFocusAndSurface(p: SharedCtx) {
  useLayoutEffect(() => {
    if (
      p.removedFocus.current &&
      !p.refs.some((r) => r.reference_id === p.removedFocus.current)
    ) {
      p.removedFocus.current = '';
      p.referencesHeading.current?.focus();
    }
  }, [p.refs]);
  useLayoutEffect(() => {
    if (!p.busy && p.exportFocus.current) {
      const control =
        p.exportFocus.current === 'operational'
          ? p.csvExportControl
          : p.exportControl;
      p.exportFocus.current = '';
      control.current?.focus();
    }
  }, [p.busy]);
  useLayoutEffect(() => {
    if (p.selected) p.detailHeading.current?.focus();
  }, [p.selected]);
  useLayoutEffect(() => {
    if (p.pair) p.pairHeading.current?.focus();
  }, [p.pair]);
  const [cursorBase, setCursorBase] = useState(p.base);
  if (cursorBase !== p.base) {
    setCursorBase(p.base);
    p.setCursor('');
    p.setHistory([]);
  }
  const surface = [
    p.context,
    p.current?.observation_id,
    p.tab,
    p.base,
    JSON.stringify(p.query),
    p.cursor,
  ].join('|');
  const [acceptedSurface, setAcceptedSurface] = useState(surface);
  if (acceptedSurface !== surface) {
    setAcceptedSurface(surface);
    p.select(undefined);
    p.setPair(undefined);
  }
  useLayoutEffect(() => {
    ++p.detailIntent.current;
  }, [surface]);
  useEffect(() => {
    const lifecycle = p.life.current;
    ++lifecycle.generation;
    return () => {
      ++lifecycle.generation;
    };
  }, [p.context]);
}

function usePPPoERunners(p: SharedCtx) {
  async function run(
    action: () => Promise<PPResult>,
    done?: (r: PPResult) => void,
  ) {
    const generation = p.life.current.generation;
    const trigger = p.focusedControl.current;
    p.setBusy(true);
    p.setError('');
    p.setNotice('');
    try {
      const r = await action();
      if (!p.valid(generation, r)) return;
      if (r.code !== 'OK') {
        p.setError(pppoeErrorMessage(r));
        if (
          [
            'JOB_INTERRUPTED',
            'REVISION_CONFLICT',
            'DEVICE_INACTIVE',
            'TRUST_REQUIRED',
          ].includes(r.code)
        )
          pending.delete(p.context + '|collect|' + p.device.revision);
      } else done?.(r);
    } catch {
      if (generation === p.life.current.generation)
        p.setError(
          'Resposta incerta. A ação mantém o mesmo identificador para reconciliação.',
        );
    } finally {
      if (generation === p.life.current.generation) {
        p.setBusy(false);
        trigger?.focus();
      }
    }
  }
  async function runDetail(
    action: (accept: () => boolean) => Promise<PPResult>,
    done: (r: PPResult) => void,
  ) {
    const generation = p.life.current.generation;
    const intent = ++p.detailIntent.current;
    const accept = () =>
      generation === p.life.current.generation &&
      intent === p.detailIntent.current;
    p.setError('');
    try {
      const r = await action(accept);
      if (!accept() || !p.valid(generation, r)) return;
      if (r.code !== 'OK') p.setError(pppoeErrorMessage(r));
      else done(r);
    } catch {
      if (accept())
        p.setError('Não foi possível carregar o detalhe. Tente novamente.');
    }
  }
  return { run, runDetail };
}

export function usePPPoEState(
  profile: string,
  activation: string,
  device: Device,
) {
  const s = usePPPoEViewState(profile, activation);
  const context = [profile, activation, device.id].join('|');
  const valid = useCallback(
    (generation: number, r: PPResult) =>
      generation === s.life.current.generation &&
      r.profile_id === profile &&
      r.activation_id === activation,
    [profile, activation, s.life],
  );
  const ctx: SharedCtx = { ...s, context, profile, activation, device, valid };
  const operationalSnapshot = useEquipmentSnapshot(
    profile,
    activation,
    device.id,
    device.revision,
    `${s.tab}|${s.selected?.row_id || ''}|${s.refresh}`,
  );
  const fastTrack = operationalSnapshot?.rows.find((r) => r.kind === 'resource')
    ?.health?.fasttrack_rules;
  usePPPoEFocusAndSurface(ctx);
  usePPPoEInitialLoad(ctx);
  usePPPoEPageLoad(ctx);
  usePPPoEJobPoll(ctx);
  const { run, runDetail } = usePPPoERunners(ctx);
  function change(q: Partial<PPQuery>) {
    s.setQuery((v) => ({ ...v, ...q }));
    s.setCursor('');
    s.setHistory([]);
    s.select(undefined);
    s.setPair(undefined);
  }
  return {
    ...ctx,
    fastTrack,
    run,
    runDetail,
    update: createUpdateHandler(ctx, run),
    save: createSaveHandler(ctx, run),
    remove: createRemoveHandler(ctx, run),
    change,
    openPair: createOpenPairHandler(ctx, runDetail),
  };
}
