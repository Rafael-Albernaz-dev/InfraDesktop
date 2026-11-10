import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import type { Device } from '../equipment/equipment-api';
import { pendingPPPoE as pending } from '../access/equipment-receipts';
import type {
  UseDiagLoadEffectProps,
  UseDiagPageEffectProps,
  UseDiagPollEffectProps,
} from './pppoe-diagnostic-effect-props';
import { diagnosticMessage } from './pppoe-diagnostic-message';
import type {
  PDDiagnostic,
  PDRow,
  PPResult,
} from '../../platform/wails/presentation';
import { pppoeDiagnosticAPI as api } from '../../platform/wails/presentation';
import { useLatestRef } from '../../shared/hooks/use-latest-ref';

export type Scope = 'catalog' | 'radius' | 'ping';
export type Source = { observation: string; row: string };

function useDiagnosticStateVars(context: string, readKey: string) {
  const [storedContext, setStoredContext] = useState(context);
  const [meta, setMeta] = useState<PDDiagnostic>();
  const [rows, setRows] = useState<PDRow[]>([]);
  const [attempt, setAttempt] =
    useState<NonNullable<PPResult['data']>['attempt']>();
  const [loading, setLoading] = useState(true);
  const [pageLoading, setPageLoading] = useState(false);
  const [error, setError] = useState('');
  const [pageError, setPageError] = useState('');
  const [cursor, setCursor] = useState('');
  const [next, setNext] = useState('');
  const [job, setJob] = useState('');
  const [starting, setStarting] = useState(false);
  const [statusFailed, setStatusFailed] = useState(false);
  const [statusRetry, retryStatus] = useState(0);
  const [refresh, reload] = useState(0);
  const [uncertain, setUncertain] = useState(false);
  const [metaReadKey, setMetaReadKey] = useState('');
  const [storedReadKey, setStoredReadKey] = useState(readKey);
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
  if (storedReadKey !== readKey) {
    setStoredReadKey(readKey);
    setLoading(true);
  }
  return {
    storedContext,
    meta,
    setMeta,
    rows,
    setRows,
    attempt,
    setAttempt,
    loading,
    setLoading,
    pageLoading,
    setPageLoading,
    error,
    setError,
    pageError,
    setPageError,
    cursor,
    setCursor,
    next,
    setNext,
    job,
    setJob,
    starting,
    setStarting,
    statusFailed,
    setStatusFailed,
    statusRetry,
    retryStatus,
    refresh,
    reload,
    uncertain,
    setUncertain,
    metaReadKey,
    setMetaReadKey,
  };
}

export type DiagState = ReturnType<typeof useDiagnosticStateVars>;

function useDiagLoadEffect(p: UseDiagLoadEffectProps) {
  const {
    s,
    context,
    readKey,
    profile,
    activation,
    deviceId,
    scope,
    sourceObservation,
    sourceRow,
    lifeRef,
    readSequenceRef,
    active,
    acknowledge,
  } = p;
  const latestRef = useLatestRef(s);
  useEffect(() => {
    const s = latestRef.current;

    const generation = lifeRef.current.generation;
    const request = ++readSequenceRef.current;
    let stopped = false;
    void api
      .current(profile, activation, deviceId, scope)
      .then((result) => {
        if (
          stopped ||
          request !== readSequenceRef.current ||
          !active(generation, result)
        )
          return;
        if (result.code !== 'OK') {
          s.setError(diagnosticMessage(result.code));
          return;
        }
        s.setMeta(result.data?.diagnostic);
        s.setMetaReadKey(readKey);
        const cand = result.data?.attempt;
        const latest =
          !sourceObservation ||
          (cand?.source_observation === sourceObservation &&
            cand?.source_row === sourceRow)
            ? cand
            : undefined;
        s.setAttempt(latest);
        if (latest?.state === 'RUNNING' && latest.job_id)
          s.setJob(latest.job_id);
        else acknowledge(latest?.operation_id);
      })
      .catch(() => {
        if (!stopped && active(generation))
          s.setError('Não foi possível abrir os dados armazenados.');
      })
      .finally(() => {
        if (!stopped && active(generation)) s.setLoading(false);
      });
    return () => {
      stopped = true;
    };
  }, [
    latestRef,
    lifeRef,
    readSequenceRef,
    context,
    s.refresh,
    profile,
    activation,
    deviceId,
    scope,
    active,
    acknowledge,
    sourceObservation,
    sourceRow,
    readKey,
  ]);
}

function useDiagPageEffect(p: UseDiagPageEffectProps) {
  const {
    s,
    context,
    diagnosticID,
    kind,
    profile,
    activation,
    deviceId,
    scope,
    readKey,
    lifeRef,
    active,
  } = p;
  const pageKey = [context, diagnosticID, s.cursor, s.refresh, kind].join('|');
  const [storedPageKey, setStoredPageKey] = useState(pageKey);
  if (storedPageKey !== pageKey) {
    setStoredPageKey(pageKey);
    s.setRows([]);
    s.setNext('');
    s.setPageError('');
    s.setPageLoading(!!diagnosticID);
  }
  const latestRef = useLatestRef(s);
  useEffect(() => {
    const s = latestRef.current;

    let stopped = false;
    const generation = lifeRef.current.generation;
    if (!diagnosticID || s.metaReadKey !== readKey) return;
    void api
      .list(profile, activation, deviceId, scope, diagnosticID, s.cursor, kind)
      .then((result) => {
        if (stopped || !active(generation, result)) return;
        if (result.code !== 'OK') {
          s.setPageError(diagnosticMessage(result.code));
          return;
        }
        s.setRows(result.data?.diagnostic_rows || []);
        s.setNext(result.data?.next_cursor || '');
      })
      .catch(() => {
        if (!stopped && active(generation))
          s.setPageError('Não foi possível carregar a página.');
      })
      .finally(() => {
        if (!stopped && active(generation)) s.setPageLoading(false);
      });
    return () => {
      stopped = true;
    };
  }, [
    latestRef,
    lifeRef,
    context,
    diagnosticID,
    kind,
    s.cursor,
    s.refresh,
    profile,
    activation,
    deviceId,
    scope,
    active,
    s.metaReadKey,
    readKey,
  ]);
}

function useDiagPollEffect(p: UseDiagPollEffectProps) {
  const { s, context, profile, activation, lifeRef, active, acknowledge } = p;
  const latestRef = useLatestRef(s);
  useEffect(() => {
    const s = latestRef.current;

    if (!s.job) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const generation = lifeRef.current.generation;
    const poll = async () => {
      s.setStatusFailed(false);
      try {
        const result = await api.status(profile, activation, s.job);
        if (stopped || !active(generation, result)) return;
        if (result.code !== 'OK' || !result.data?.job) {
          s.setStatusFailed(true);
          s.setError(diagnosticMessage(result.code));
          return;
        }
        const current = result.data.job;
        if (current.state === 'RUNNING')
          timer = setTimeout(() => void poll(), 150);
        else {
          acknowledge(current.pppoe?.operation_id);
          s.setUncertain(false);
          s.setJob('');
          s.setCursor('');
          s.setError(
            current.code === 'OK' || current.code === 'CANCELLED'
              ? ''
              : diagnosticMessage(current.code),
          );
          s.reload((n) => n + 1);
        }
      } catch {
        if (!stopped && active(generation)) {
          s.setStatusFailed(true);
          s.setError(
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
  }, [
    latestRef,
    lifeRef,
    context,
    s.job,
    s.statusRetry,
    profile,
    activation,
    active,
    acknowledge,
  ]);
}

import { useDiagActions } from './pppoe-diag-actions';

export function useDiagnostic(
  profile: string,
  activation: string,
  device: Device,
  scope: Scope,
  source?: Source,
  kind = '',
) {
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
  const lifeRef = useRef({ generation: 0 });
  const readSequenceRef = useRef(0);
  const readKey = context + '|' + 0;
  const s = useDiagnosticStateVars(context, readKey);
  const active = useCallback(
    (gen: number, res?: PPResult) =>
      gen === lifeRef.current.generation &&
      (!res ||
        (res.profile_id === profile && res.activation_id === activation)),
    [profile, activation],
  );
  const acknowledge = useCallback(
    (operation?: string) => {
      if (!operation) return;
      const prefix =
        [profile, activation, device.id, 'diag', scope].join('|') + '|';
      for (const [receiptKey, receipt] of pending) {
        if (receiptKey.startsWith(prefix) && receipt.operation === operation)
          pending.delete(receiptKey);
      }
    },
    [profile, activation, device.id, scope],
  );
  useLayoutEffect(() => {
    const lifecycle = lifeRef.current;
    ++lifecycle.generation;
    return () => {
      ++lifecycle.generation;
    };
  }, [context]);
  useDiagLoadEffect({
    s,
    context,
    readKey: context + '|' + s.refresh,
    profile,
    activation,
    deviceId: device.id,
    scope,
    sourceObservation: source?.observation,
    sourceRow: source?.row,
    lifeRef,
    readSequenceRef,
    active,
    acknowledge,
  });
  const visibleMeta =
    s.storedContext === context &&
    (!source ||
      (s.meta?.source_observation === source.observation &&
        s.meta?.source_row === source.row))
      ? s.meta
      : undefined;
  useDiagPageEffect({
    s,
    context,
    diagnosticID: visibleMeta?.diagnostic_id,
    kind,
    profile,
    activation,
    deviceId: device.id,
    scope,
    readKey: context + '|' + s.refresh,
    lifeRef,
    active,
  });
  useDiagPollEffect({
    s,
    context,
    profile,
    activation,
    lifeRef,
    active,
    acknowledge,
  });
  const { start, cancel } = useDiagActions({
    s,
    key,
    profile,
    activation,
    device,
    scope,
    source,
    lifeRef,
    active,
    acknowledge,
  });
  return {
    meta: visibleMeta,
    rows: visibleMeta ? s.rows : [],
    attempt: s.attempt,
    loading: s.loading,
    pageLoading: s.pageLoading,
    error: s.error,
    pageError: s.pageError,
    job: s.job,
    starting: s.starting,
    statusFailed: s.statusFailed,
    uncertain: s.uncertain || pending.has(key),
    cursor: s.cursor,
    next: s.next,
    start,
    cancel,
    retryStatus: () => s.retryStatus((n) => n + 1),
    reopen: () => {
      s.setError('');
      s.setPageError('');
      s.setCursor('');
      s.reload((n) => n + 1);
    },
    first: () => s.setCursor(''),
    more: () => s.setCursor(s.next),
  };
}
