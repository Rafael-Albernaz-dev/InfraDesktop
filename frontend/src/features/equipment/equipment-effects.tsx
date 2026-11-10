import { useJobCompletionSyncEffect } from './equipment-completion-sync';
import { useEffect } from 'react';
import type { Device, Job } from './equipment-api';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import { clearPendingStartsForJob } from './equipment-pending-revisions';
import { useLatestRef } from '../../shared/hooks/use-latest-ref';
export { useDeviceListEffect } from './inventory-list-effect';
type UseJobPollEffectProps = {
  job?: Job;
  profile: string;
  activation: string;
  surfaceRef: React.MutableRefObject<number>;
  setError: (v: string) => void;
  setJob: (v: Job | undefined) => void;
  reload: React.Dispatch<React.SetStateAction<number>>;
};
export function useJobPollEffect(p: UseJobPollEffectProps) {
  const latestRef = useLatestRef(p);
  useEffect(() => {
    const p = latestRef.current;

    if (!p.job || p.job.state !== 'RUNNING') return;
    const jobId = p.job.id;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    let failures = 0;
    const intent = p.surfaceRef.current;
    const failed = (code: string) => {
      if (intent === p.surfaceRef.current)
        p.setError(
          equipmentMessage(code) +
            ' Estado da consulta ainda não confirmado; retome pelo mesmo botão, sem nova autenticação automática.',
        );
      if (++failures < 3) timer = setTimeout(() => void poll(), 250);
      else p.setJob(undefined);
    };
    const poll = async () => {
      try {
        const r = await api.status(p.profile, p.activation, jobId);
        if (stopped) return;
        if (r.code !== 'OK' || !r.data?.job) return failed(r.code);
        const current = r.data.job;
        if (failures && intent === p.surfaceRef.current) p.setError('');
        failures = 0;
        if (current.state !== 'RUNNING') {
          clearPendingStartsForJob(p.profile, p.activation, current);
          p.setJob(intent === p.surfaceRef.current ? current : undefined);
          if (current.code !== 'OK' && intent === p.surfaceRef.current)
            p.setError(equipmentMessage(current.code));
          if (current.kind === 'device.query-start') p.reload((n) => n + 1);
        } else {
          timer = setTimeout(() => void poll(), 250);
        }
      } catch {
        if (!stopped) failed('SERVICE_UNAVAILABLE');
      }
    };
    timer = setTimeout(() => void poll(), 150);
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [
    latestRef,
    p.job,
    p.profile,
    p.activation,
    p.surfaceRef,
    p.setError,
    p.setJob,
    p.reload,
  ]);
}

function fetchTargetDetail(
  profile: string,
  activation: string,
  requested: string,
  openOnSuccess: boolean,
  detailReadRef: React.MutableRefObject<number>,
  surfaceRef: React.MutableRefObject<number>,
  setSelected: (d: Device | undefined) => void,
  setDetailOpen: (v: boolean) => void,
  setError: (v: string) => void,
) {
  let cancelled = false;
  const read = ++detailReadRef.current;
  const intent = surfaceRef.current;
  void api
    .get(profile, activation, requested)
    .then((r) => {
      if (
        cancelled ||
        read !== detailReadRef.current ||
        intent !== surfaceRef.current
      )
        return;
      if (r.code === 'OK' && r.data?.device?.id === requested) {
        setSelected(r.data.device);
        if (openOnSuccess) setDetailOpen(true);
      } else
        setError(
          equipmentMessage(r.code === 'OK' ? 'REVISION_CONFLICT' : r.code),
        );
    })
    .catch(() => {
      if (
        !cancelled &&
        read === detailReadRef.current &&
        intent === surfaceRef.current
      )
        setError(equipmentMessage('SERVICE_UNAVAILABLE'));
    });
  return () => {
    cancelled = true;
  };
}

type UseDetailDeviceSyncEffectsProps = {
  targetID?: string;
  profile: string;
  activation: string;
  connected: boolean;
  detailOpen: boolean;
  selected?: Device;
  job?: Job;
  onSelect?: (id: string) => void;
  detailReadRef: React.MutableRefObject<number>;
  surfaceRef: React.MutableRefObject<number>;
  setSelected: (d: Device | undefined) => void;
  setDetailOpen: (v: boolean) => void;
  setError: (v: string) => void;
};
export function useDetailDeviceSyncEffects(p: UseDetailDeviceSyncEffectsProps) {
  useEffect(() => {
    if (!p.targetID || !p.connected) return;
    return fetchTargetDetail(
      p.profile,
      p.activation,
      p.targetID,
      true,
      p.detailReadRef,
      p.surfaceRef,
      p.setSelected,
      p.setDetailOpen,
      p.setError,
    );
  }, [
    p.targetID,
    p.profile,
    p.activation,
    p.connected,
    p.detailReadRef,
    p.surfaceRef,
    p.setSelected,
    p.setDetailOpen,
    p.setError,
  ]);
  useEffect(() => {
    if (!p.detailOpen || !p.selected?.id || !p.connected) return;
    return fetchTargetDetail(
      p.profile,
      p.activation,
      p.selected.id,
      false,
      p.detailReadRef,
      p.surfaceRef,
      p.setSelected,
      p.setDetailOpen,
      p.setError,
    );
  }, [
    p.detailOpen,
    p.selected?.id,
    p.selected?.revision,
    p.profile,
    p.activation,
    p.connected,
    p.detailReadRef,
    p.surfaceRef,
    p.setSelected,
    p.setDetailOpen,
    p.setError,
  ]);
  useJobCompletionSyncEffect(p);
}
