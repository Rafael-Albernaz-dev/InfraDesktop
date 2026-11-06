import { useSessionView } from './use-session-view';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useSessionRefs } from './access-session-refs';
import { useLatestRef } from './use-latest-ref';
import type { Page } from './desktop';
import type { Branding } from './desktop-model';
import { acceptEquipmentAuthority } from './equipment-receipts';
import type { Intent } from './microcopy';
import { message } from './microcopy';
import type { Result } from './presentation';
import { access } from './presentation';

export const initial: Result = {
  code: 'SERVICE_UNAVAILABLE',
  request_id: '',
  instance_id: '',
  activation_id: '',
  profile_id: '',
  heartbeat: 0,
  setup: false,
  authenticated: false,
  name: '',
  theme: 'light',
  recovery: '',
  companies: [],
};

export function useBrandState(view: Result, connected: boolean) {
  const [brandState, setBrandState] = useState<{
    key: string;
    value: Branding;
  }>({ key: '', value: { name: '', logo: '', accent: 'teal' } });
  const brandRead = useRef(0);
  const [brandRetry, retryBrand] = useState(0);
  const [brandError, setBrandError] = useState('');
  const brandKey = [
    view.instance_id,
    view.profile_id,
    view.activation_id,
    view.authenticated,
  ].join('|');
  useEffect(() => {
    let cancelled = false;
    const read = ++brandRead.current;
    if (!view.authenticated || !view.profile_id || !connected) return;
    void access
      .getBrand(view.profile_id, view.activation_id)
      .then((r) => {
        if (cancelled || read !== brandRead.current) return;
        if (r.code === 'OK' && r.brand) {
          setBrandState({ key: brandKey, value: r.brand });
          setBrandError('');
        } else setBrandError(message(r.code, 'brand'));
      })
      .catch(() => {
        if (!cancelled && read === brandRead.current)
          setBrandError(message('SERVICE_UNAVAILABLE', 'brand'));
      });
    return () => {
      cancelled = true;
    };
  }, [
    brandKey,
    view.authenticated,
    view.profile_id,
    view.activation_id,
    connected,
    brandRetry,
  ]);
  const savedBrand =
    brandState.key === brandKey
      ? brandState.value
      : { name: '', logo: '', accent: 'teal' };
  return {
    brandKey,
    brandState,
    setBrandState,
    brandRead,
    brandError,
    setBrandError,
    retryBrand,
    savedBrand,
  };
}

export function useStatusPoll(
  working: React.RefObject<boolean>,
  epoch: React.RefObject<number>,
  accept: (r: Result) => void,
  setConnected: (v: boolean) => void,
  invalidate: () => void,
) {
  useEffect(() => {
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    let failures = 0;
    const request = async () => {
      const generation = epoch.current;
      if (working.current) return;
      try {
        const r = await access.status();
        if (!stopped && generation === epoch.current) {
          if (r.code === 'OK') {
            failures = 0;
            accept(r);
          } else {
            failures++;
            setConnected(false);
            if (r.code === 'UNAUTHENTICATED') accept(r);
          }
        }
      } catch {
        if (!stopped && generation === epoch.current) {
          failures++;
          setConnected(false);
        }
      }
    };
    const poll = async () => {
      await request();
      if (!stopped)
        timer = setTimeout(
          () => void poll(),
          Math.min(10000, 1000 * 2 ** Math.min(failures, 4)),
        );
    };
    void poll();
    return () => {
      stopped = true;
      clearTimeout(timer);
      invalidate();
    };
  }, [accept, epoch, invalidate, setConnected, working]);
}

export type PerformFn = (
  call: () => Promise<Result>,
  after?: (r: Result) => void,
  applyResult?: boolean,
  intent?: Intent,
) => Promise<boolean>;

export function syncAuthority(r: Result) {
  if (r.code === 'UNAUTHENTICATED') acceptEquipmentAuthority('', '');
  else if (r.code === 'OK')
    acceptEquipmentAuthority(
      r.authenticated ? r.profile_id : '',
      r.authenticated ? r.activation_id : '',
    );
}

export function hasScopeChanged(previous: Result, r: Result): boolean {
  const reconnected = previous.instance_id !== r.instance_id;
  return (
    previous.authenticated !== r.authenticated ||
    previous.profile_id !== r.profile_id ||
    previous.name !== r.name ||
    (!reconnected && previous.activation_id !== r.activation_id)
  );
}

export type SessionRefs = {
  securityLockedRef: React.MutableRefObject<boolean>;
  epochRef: React.MutableRefObject<number>;
  hadAuthRef: React.MutableRefObject<boolean>;
  contextKeyRef: React.MutableRefObject<string>;
  surfaceRef: React.MutableRefObject<number>;
  previousViewRef: React.MutableRefObject<Result>;
  setError: (v: string) => void;
  setRecovery: (v: string) => void;
  setDialog: (v: string) => void;
  setRecoveryUncertain: (v: boolean) => void;
  setView: (v: Result) => void;
  setConnected: (v: boolean) => void;
};

export function applySessionResult(
  r: Result,
  refs: SessionRefs,
  setPage: (p: Page) => void,
  setTheme: (t: string) => void,
) {
  syncAuthority(r);
  const key = [
    r.instance_id,
    r.authenticated,
    r.profile_id,
    r.activation_id,
    r.name,
  ].join('|');
  const previous = refs.previousViewRef.current;
  if (refs.contextKeyRef.current && refs.contextKeyRef.current !== key)
    refs.epochRef.current++;
  if (hasScopeChanged(previous, r)) {
    refs.setError('');
    refs.surfaceRef.current++;
    if (!r.authenticated && previous.authenticated) {
      refs.securityLockedRef.current = false;
      refs.setRecoveryUncertain(false);
      refs.setRecovery('');
    }
    if (!refs.securityLockedRef.current) {
      refs.setDialog('');
      if (previous.authenticated || r.authenticated) refs.setRecovery('');
    }
  }
  refs.previousViewRef.current = r;
  refs.contextKeyRef.current = key;
  refs.setView({ ...r, recovery: '' });
  refs.setConnected(r.code === 'OK');
  if (r.authenticated) {
    setTheme(r.theme);
    if (!refs.hadAuthRef.current) setPage('companies');
  }
  refs.hadAuthRef.current = r.authenticated;
}

export function useAccessSession(
  setPage: (p: Page) => void,
  setTheme: (t: string) => void,
) {
  const state = useSessionView(initial);
  const refs = useSessionRefs(initial),
    stateRef = useLatestRef(state);
  const { securityLockedRef, epochRef, workingRef, surfaceRef } = refs;
  const { setConnected } = state;
  const bumpSurface = useCallback(() => {
    surfaceRef.current++;
  }, [surfaceRef]);
  const accept = useCallback(
    (r: Result) =>
      applySessionResult(
        r,
        { ...refs, ...stateRef.current },
        setPage,
        setTheme,
      ),
    [refs, stateRef, setPage, setTheme],
  );
  const invalidate = useCallback(() => {
    epochRef.current++;
  }, [epochRef]);
  useStatusPoll(workingRef, epochRef, accept, setConnected, invalidate);
  return {
    ...state,
    securityLockedRef,
    epochRef,
    workingRef,
    surfaceRef,
    bumpSurface,
    accept,
    invalidate,
  };
}

export function createBrandSaver(
  view: Result,
  brand: ReturnType<typeof useBrandState>,
  perform: PerformFn,
) {
  if (!view.profile_id || brand.brandState.key !== brand.brandKey)
    return undefined;
  return async (nextBrand: Branding) => {
    ++brand.brandRead.current;
    const expected = brand.brandKey;
    let success = false;
    let committed = false;
    await perform(
      async () => {
        const result = await access.saveBrand(
          view.profile_id,
          view.activation_id,
          nextBrand,
        );
        committed = result.code === 'OK';
        return result;
      },
      (r) => {
        if (r.brand && expected === brand.brandKey) {
          brand.setBrandState({ key: expected, value: r.brand });
          brand.setBrandError('');
          success = true;
        }
      },
      false,
      'brand',
    );
    if (committed && !success) brand.retryBrand((n) => n + 1);
    return success;
  };
}
