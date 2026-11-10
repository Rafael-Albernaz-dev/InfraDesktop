import { useLayoutEffect, useState } from 'react';
import { acceptEquipmentAuthority } from '../equipment/equipment-receipts';
import { FRESH_QUERY } from './pppoe-context-actions';
import type {
  PPResult,
  PPDelta,
  PPRow,
} from '../../platform/wails/presentation';
import type {
  PPObservation,
  PPReference,
} from '../../platform/wails/presentation-pppoe-types';
export type PPPoETab =
  'sessions' | 'compare' | 'references' | 'catalog' | 'radius' | 'monitoring';
function usePPPoEPageStatus() {
  const [pageState, setPageState] = useState<
    'idle' | 'loading' | 'ready' | 'error'
  >('loading');
  const [statusFailed, setStatusFailed] = useState(false);
  const [statusRetry, retryStatus] = useState(0);
  const [detailTab, setDetailTab] = useState<
    'traffic' | 'information' | 'link' | 'history'
  >('traffic');
  const [pair, setPair] = useState<{ left?: PPRow; right?: PPRow }>();
  const [referencesState, setReferencesState] = useState<
    'loading' | 'ready' | 'error'
  >('loading');
  const [initialFailed, setInitialFailed] = useState(false);
  const [current, setCurrent] = useState<PPObservation>();
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
  };
}
function usePPPoEListState() {
  const [rows, setRows] = useState<PPRow[]>([]);
  const [savedReferences, setSavedReferences] = useState<PPReference[]>([]);
  const [deltas, setDeltas] = useState<PPDelta[]>([]);
  const [query, setQuery] = useState(FRESH_QUERY);
  const [cursor, setCursor] = useState('');
  const [next, setNext] = useState('');
  const [showPlan, setShowPlan] = useState(false);
  const [history, setHistory] = useState<string[]>([]);
  return {
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
  };
}
function usePPPoESelectionState(initialTab: PPPoETab) {
  const [selected, select] = useState<PPRow>();
  const [base, setBase] = useState('');
  const [tab, setTab] = useState<PPPoETab>(initialTab);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [collectionFailure, setCollectionFailure] = useState<{
    code: string;
    reason: string;
  }>();
  const [lastAttempt, setLastAttempt] =
    useState<NonNullable<PPResult['data']>['attempt']>();
  const [attempt, setAttempt] = useState('');
  return {
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
    collectionFailure,
    setCollectionFailure,
    lastAttempt,
    setLastAttempt,
    attempt,
    setAttempt,
  };
}
function usePPPoEJobState() {
  const [job, setJob] = useState('');
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [refresh, bump] = useState(0);
  return {
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

export function usePPPoEViewState(
  profile: string,
  activation: string,
  initialTab: PPPoETab,
) {
  useLayoutEffect(() => {
    acceptEquipmentAuthority(profile, activation);
  }, [profile, activation]);
  return {
    ...usePPPoEPageStatus(),
    ...usePPPoEListState(),
    ...usePPPoESelectionState(initialTab),
    ...usePPPoEJobState(),
  };
}
