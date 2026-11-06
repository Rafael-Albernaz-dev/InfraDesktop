import { useEffect } from 'react';
import { pppoeReadAPI as readAPI } from './presentation';

export function usePPReadCleanup(p: {
  profile: string;
  activation: string;
  deviceId: string;
  generationRef: React.MutableRefObject<number>;
  activeJobRef: React.MutableRefObject<string>;
  privateResultJobRef: React.MutableRefObject<string>;
  pendingOperationRef: React.MutableRefObject<string>;
  renewalTimerRef: React.MutableRefObject<ReturnType<typeof setTimeout> | undefined>;
}) {
  const {
    profile,
    activation,
    deviceId,
    generationRef,
    activeJobRef,
    privateResultJobRef,
    pendingOperationRef,
    renewalTimerRef,
  } = p;
  useEffect(() => {
    return () => {
      clearTimeout(renewalTimerRef.current);
      generationRef.current += 1;
      const id = activeJobRef.current || privateResultJobRef.current;
      const op = pendingOperationRef.current;
      activeJobRef.current = '';
      privateResultJobRef.current = '';
      pendingOperationRef.current = '';
      if (id) void readAPI.cancel(profile, activation, id).catch(() => {});
      else if (op)
        void readAPI
          .cancelOperation(profile, activation, deviceId, op)
          .catch(() => {});
    };
  }, [
    profile,
    activation,
    deviceId,
    generationRef,
    activeJobRef,
    privateResultJobRef,
    pendingOperationRef,
    renewalTimerRef,
  ]);
}

