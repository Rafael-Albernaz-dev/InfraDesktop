import { latestRefreshAttempt } from './refresh-attempt-order';
import { useCallback, useEffect, useSyncExternalStore } from 'react';
import type { Device } from './equipment-api';
type Attempt = { code: string; state: string; at: string };
type Recorded = { key: string; jobID: string; attempt: Attempt };
// Only status/age are retained, scoped to the authenticated account and device.
// A refused credential stays blocked when the sheet is reopened.
const history = new Map<string, Recorded>();
const listeners = new Set<() => void>();
function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
export function useRefreshAttempt(
  scope: string,
  device: Device,
  job?: { id: string; state: string; code: string },
) {
  const key = `${scope}|${device.id}|${device.credential_id}|${device.fingerprint}`;
  const snapshot = useCallback(() => history.get(key), [key]);
  const recorded = useSyncExternalStore(subscribe, snapshot);
  const terminal = Boolean(job && job.state !== 'RUNNING');
  const jobID = job?.id || '',
    state = job?.state || '',
    code = job?.code || '';
  useEffect(() => {
    if (!terminal || history.get(key)?.jobID === jobID) return;
    history.set(key, {
      key,
      jobID,
      attempt: { code, state, at: new Date().toISOString() },
    });
    for (const listener of listeners) listener();
  }, [key, terminal, jobID, state, code]);
  const attempt = latestRefreshAttempt(device.last_attempt, recorded?.attempt);
  return { attempt, ready: !terminal || recorded?.jobID === jobID };
}
