import { useEffect } from 'react';
import { useLatestRef } from '../../shared/hooks/use-latest-ref';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import type { Device, Job } from './equipment-api';
type UseJobCompletionSyncEffectProps = {
  job?: Job;
  profile: string;
  activation: string;
  onSelect?: (id: string) => void;
  surfaceRef: React.MutableRefObject<number>;
  setSelected: (d: Device | undefined) => void;
  setDetailOpen: (v: boolean) => void;
  setError: (v: string) => void;
};
export function useJobCompletionSyncEffect(p: UseJobCompletionSyncEffectProps) {
  const latestRef = useLatestRef(p);
  useEffect(() => {
    const p = latestRef.current;

    if (
      !p.job ||
      !['device.query-start', 'trust.inspect'].includes(p.job.kind) ||
      p.job.state === 'RUNNING'
    )
      return;
    const jobDeviceId = p.job.device_id;
    let cancelled = false;
    const intent = p.surfaceRef.current;
    void (async () => {
      await Promise.resolve();
      if (cancelled || intent !== p.surfaceRef.current) return;
      try {
        const r = await api.get(p.profile, p.activation, jobDeviceId);
        if (cancelled || intent !== p.surfaceRef.current) return;
        if (r.code === 'OK' && r.data?.device?.id === jobDeviceId) {
          p.setSelected(r.data.device);
          p.setDetailOpen(true);
          p.onSelect?.(r.data.device.id);
        } else p.setError(equipmentMessage(r.code));
      } catch {
        if (!cancelled && intent === p.surfaceRef.current)
          p.setError(equipmentMessage('SERVICE_UNAVAILABLE'));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [
    latestRef,
    p.job,
    p.profile,
    p.activation,
    p.onSelect,
    p.surfaceRef,
    p.setSelected,
    p.setDetailOpen,
    p.setError,
  ]);
}
