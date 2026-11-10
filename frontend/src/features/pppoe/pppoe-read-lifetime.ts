import { useEffect } from 'react';
import type { ReadContext } from './pppoe-read-lifecycle';
import type { PPReadPanelProps } from './pppoe-read-props';
import { pppoeReadAPI as api } from '../../platform/wails/presentation';
function readLifetime(
  c: ReadContext,
  profile: string,
  activation: string,
  deviceId: string,
) {
  const l = c.life.current;
  l.disposed = false;
  return () => {
    l.disposed = true;
    l.automaticStarted = false;
    clearTimeout(l.renewalTimer);
    clearTimeout(l.pollTimer);
    l.generation++;
    const id = l.activeJob || l.privateJob,
      op = l.operation;
    l.activeJob = '';
    l.privateJob = '';
    l.operation = '';
    if (id) void api.cancel(profile, activation, id).catch(() => {});
    else if (op)
      void api
        .cancelOperation(profile, activation, deviceId, op)
        .catch(() => {});
  };
}
export function useReadLifetime(c: ReadContext, p: PPReadPanelProps) {
  const {
    profile,
    activation,
    device: { id: deviceId },
  } = p;
  useEffect(
    () => readLifetime(c, profile, activation, deviceId),
    [c, profile, activation, deviceId],
  );
}
