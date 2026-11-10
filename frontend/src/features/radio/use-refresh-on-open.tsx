import { refreshAttemptInProgress } from '../../shared/hooks/refresh-attempt-order';
import { RadioCodes } from './radio-contract';
import { useEffect, useEffectEvent, useRef } from 'react';
import type { MutableRefObject } from 'react';
import type { Device } from '../equipment/equipment-api';

export type RefreshOnOpenInput = {
  now: number;
  observedAt?: string;
  device: Pick<Device, 'active' | 'fingerprint' | 'credential_id'> & {
    last_attempt?: { state: string; code: string; at: string };
  };
  vaultAvailable: boolean;
  serviceAvailable: boolean;
  reading: boolean;
};

export function shouldRefreshOnOpen(p: RefreshOnOpenInput): boolean {
  const observed = Date.parse(p.observedAt || '');
  if (!Number.isFinite(observed) || p.now - observed <= 60_000) return false;
  if (!p.device.active || !p.device.fingerprint || !p.device.credential_id)
    return false;
  if (!p.vaultAvailable || !p.serviceAvailable || p.reading) return false;
  const attempt = p.device.last_attempt;
  if (
    refreshAttemptInProgress(attempt) ||
    attempt?.code === RadioCodes.REMOTE_AUTH_FAILED
  )
    return false;
  if (attempt && attempt.code !== 'OK') {
    const attempted = Date.parse(attempt.at);
    // Unknown failure age is not permission to retry a credential.
    if (!Number.isFinite(attempted) || p.now - attempted < 300_000)
      return false;
  }
  return true;
}

// The opening owns its authority and one decision. Later data/revision/activation
// reconciliation cannot renew that decision; loading saved data may finish first.
export type RefreshOpening = { key: string; scope: string; decided: boolean };
export type RefreshOpeningRef = MutableRefObject<RefreshOpening | undefined>;
export function useRefreshOnOpen(p: {
  decision?: RefreshOpeningRef;
  opening: string;
  scope: string;
  ready: boolean;
  input: Omit<RefreshOnOpenInput, 'now'> & { now?: number };
  refresh: () => void;
}) {
  const ownOpeningRef = useRef<RefreshOpening | undefined>(undefined);
  const openingRef = p.decision || ownOpeningRef;
  const decide = useEffectEvent(() => {
    if (shouldRefreshOnOpen({ ...p.input, now: p.input.now ?? Date.now() }))
      p.refresh();
  });
  useEffect(() => {
    if (!p.opening) {
      openingRef.current = undefined;
      return;
    }
    if (openingRef.current?.key !== p.opening)
      openingRef.current = { key: p.opening, scope: p.scope, decided: false };
    const opened = openingRef.current;
    if (opened.scope !== p.scope) {
      opened.decided = true;
      return;
    }
    if (!p.ready || opened.decided) return;
    opened.decided = true;
    decide();
  }, [p.opening, p.scope, p.ready, openingRef]);
}
