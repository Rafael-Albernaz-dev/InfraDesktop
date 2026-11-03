import { useCallback, useEffect, useLayoutEffect, useState } from 'react';
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

export type PPPoERefs = {
  detailTriggerRef: React.MutableRefObject<HTMLElement | null>;
  detailHeadingRef: React.RefObject<HTMLHeadingElement | null>;
  pairHeadingRef: React.RefObject<HTMLHeadingElement | null>;
  focusedControlRef: React.MutableRefObject<HTMLElement | null>;
  searchControlRef: React.RefObject<HTMLInputElement | null>;
  referencesHeadingRef: React.RefObject<HTMLHeadingElement | null>;
  exportControlRef: React.RefObject<HTMLButtonElement | null>;
  csvExportControlRef: React.RefObject<HTMLButtonElement | null>;
  removedFocusRef: React.MutableRefObject<string>;
  exportFocusRef: React.MutableRefObject<'sanitized' | 'operational' | ''>;
  lifeRef: React.MutableRefObject<{ generation: number }>;
  primaryRef: React.RefObject<HTMLButtonElement | null>;
  detailIntentRef: React.MutableRefObject<number>;
};

function usePPPoEViewState(profile: string, activation: string) {
  useLayoutEffect(() => {
    acceptEquipmentAuthority(profile, activation);
  }, [profile, activation]);
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
  const [savedReferences, setSavedReferences] = useState<PPReference[]>([]);
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
    savedReferences,
    setSavedReferences,
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
type SharedCtx = ViewState &
  PPPoERefs & {
    context: string;
    profile: string;
    activation: string;
    device: Device;
    valid: (g: number, r: PPResult) => boolean;
    // Legacy shim aliases for action creators:
    primary: React.RefObject<HTMLButtonElement | null>;
    removedFocus: React.MutableRefObject<string>;
    exportFocus: React.MutableRefObject<'sanitized' | 'operational' | ''>;
    life: React.MutableRefObject<{ generation: number }>;
    focusedControl: React.MutableRefObject<HTMLElement | null>;
    detailTrigger: React.MutableRefObject<HTMLElement | null>;
    detailIntent: React.MutableRefObject<number>;
    refs: PPReference[];
  };

function usePPPoEInitialLoad(p: SharedCtx) {
  const { context, refresh, profile, activation, device, valid, lifeRef } = p;
  useEffect(() => {
    const generation = lifeRef.current.generation;
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
        p.setSavedReferences(rr.data?.references || []);
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
  }, [context, refresh, profile, activation, device.id, valid, lifeRef]);
}

function usePPPoEPageLoad(p: SharedCtx) {
  const {
    context,
    current,
    query,
    cursor,
    tab,
    base,
    profile,
    activation,
    lifeRef,
  } = p;
  useEffect(() => {
    const generation = lifeRef.current.generation;
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
  const {
    job,
    context,
    statusRetry,
    profile,
    activation,
    device,
    lifeRef,
    primaryRef,
    valid,
  } = p;
  useEffect(() => {
    if (!job) return;
    let stopped = false;
    const generation = lifeRef.current.generation;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      p.setStatusFailed(false);
      try {
        const r = await api.status(profile, activation, job);
        if (stopped || !valid(generation, r)) return;
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
          primaryRef.current?.focus();
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
    valid,
    lifeRef,
    primaryRef,
  ]);
}

function usePPPoEFocusAndSurface(p: SharedCtx) {
  const {
    removedFocusRef,
    exportFocusRef,
    detailIntentRef,
    lifeRef,
    referencesHeadingRef,
    csvExportControlRef,
    exportControlRef,
    detailHeadingRef,
    pairHeadingRef,
    savedReferences,
    busy,
    selected,
    pair,
    base,
    setCursor,
    setHistory,
    select,
    setPair,
    context,
    current,
    tab,
    query,
    cursor,
  } = p;
  useLayoutEffect(() => {
    if (
      removedFocusRef.current &&
      !savedReferences.some((r) => r.reference_id === removedFocusRef.current)
    ) {
      removedFocusRef.current = '';
      referencesHeadingRef.current?.focus();
    }
  }, [savedReferences, removedFocusRef, referencesHeadingRef]);
  useLayoutEffect(() => {
    if (!busy && exportFocusRef.current) {
      const control =
        exportFocusRef.current === 'operational'
          ? csvExportControlRef
          : exportControlRef;
      exportFocusRef.current = '';
      control.current?.focus();
    }
  }, [busy, exportFocusRef, csvExportControlRef, exportControlRef]);
  useLayoutEffect(() => {
    if (selected) detailHeadingRef.current?.focus();
  }, [selected, detailHeadingRef]);
  useLayoutEffect(() => {
    if (pair) pairHeadingRef.current?.focus();
  }, [pair, pairHeadingRef]);
  const [cursorBase, setCursorBase] = useState(base);
  if (cursorBase !== base) {
    setCursorBase(base);
    setCursor('');
    setHistory([]);
  }
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
    ++detailIntentRef.current;
  }, [surface, detailIntentRef]);
  useEffect(() => {
    const lifecycle = lifeRef.current;
    ++lifecycle.generation;
    return () => {
      ++lifecycle.generation;
    };
  }, [context, lifeRef]);
}

function usePPPoERunners(p: SharedCtx) {
  const { lifeRef, focusedControlRef, detailIntentRef, valid } = p;
  async function run(
    action: () => Promise<PPResult>,
    done?: (r: PPResult) => void,
  ) {
    const generation = lifeRef.current.generation;
    const trigger = focusedControlRef.current;
    p.setBusy(true);
    p.setError('');
    p.setNotice('');
    try {
      const r = await action();
      if (!valid(generation, r)) return;
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
      if (generation === lifeRef.current.generation)
        p.setError(
          'Resposta incerta. A ação mantém o mesmo identificador para reconciliação.',
        );
    } finally {
      if (generation === lifeRef.current.generation) {
        p.setBusy(false);
        trigger?.focus();
      }
    }
  }
  async function runDetail(
    action: (accept: () => boolean) => Promise<PPResult>,
    done: (r: PPResult) => void,
  ) {
    const generation = lifeRef.current.generation;
    const intent = ++detailIntentRef.current;
    const accept = () =>
      generation === lifeRef.current.generation &&
      intent === detailIntentRef.current;
    p.setError('');
    try {
      const r = await action(accept);
      if (!accept() || !valid(generation, r)) return;
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
  refs: PPPoERefs,
) {
  const s = usePPPoEViewState(profile, activation);
  const context = [profile, activation, device.id].join('|');
  const valid = useCallback(
    (generation: number, r: PPResult) =>
      generation === refs.lifeRef.current.generation &&
      r.profile_id === profile &&
      r.activation_id === activation,
    [profile, activation, refs.lifeRef],
  );
  const ctx: SharedCtx = {
    ...s,
    ...refs,
    primary: refs.primaryRef,
    removedFocus: refs.removedFocusRef,
    exportFocus: refs.exportFocusRef,
    life: refs.lifeRef,
    focusedControl: refs.focusedControlRef,
    detailTrigger: refs.detailTriggerRef,
    detailIntent: refs.detailIntentRef,
    refs: s.savedReferences,
    context,
    profile,
    activation,
    device,
    valid,
  };
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
    ...s,
    context,
    profile,
    activation,
    device,
    refs: s.savedReferences,
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
