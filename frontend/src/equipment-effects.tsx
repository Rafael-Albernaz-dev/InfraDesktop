import { useEffect, useRef } from 'react';
import type { Device, Job } from './equipment-api';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import { clearPendingStartsForJob } from './equipment-pending-revisions';
import { useLatestRef } from './use-latest-ref';
function reconcileListedSelection(
  old: Device | undefined,
  devices: Device[],
  detailOpen: boolean,
): Device | undefined {
  if (detailOpen) return old;
  const current =
    devices.find((d) => d.id === old?.id) ||
    (old && detailOpen ? old : undefined);
  return old &&
    current &&
    old.id === current.id &&
    old.revision === current.revision &&
    old.result?.observed_at === current.result?.observed_at &&
    (old.result?.interfaces?.length || detailOpen)
    ? old
    : current;
}
type UseDeviceListEffectProps = {
  profile: string;
  activation: string;
  search: string;
  cursor: number;
  inactive: boolean;
  roleFilter: string;
  blockFilter: string;
  modelFilter: string;
  setBlocks: (v: string[]) => void;
  setModels: (v: string[]) => void;
  connected: boolean;
  refresh: number;
  detailOpen: boolean;
  onTotal?: (context: string, total: number) => void;
  generationRef: React.MutableRefObject<number>;
  setListError: (v: string) => void;
  setDevices: (v: Device[]) => void;
  setTotal: (v: number) => void;
  setNext: (v: number) => void;
  setVault: (v: boolean) => void;
  setReady: (v: boolean) => void;
  setSelected: React.Dispatch<React.SetStateAction<Device | undefined>>;
};
export function useDeviceListEffect(p: UseDeviceListEffectProps) {
  const { generationRef } = p;
  const detailOpen = useRef(p.detailOpen);
  const latestRef = useLatestRef(p);
  useEffect(() => {
    const p = latestRef.current;

    detailOpen.current = p.detailOpen;
  }, [latestRef, p.detailOpen]);
  useEffect(() => {
    const p = latestRef.current;

    let stopped = false;
    const read = ++generationRef.current;
    if (!p.profile || !p.connected) return;
    void (async () => {
      try {
        const filters =
          p.roleFilter || p.blockFilter || p.modelFilter || p.inactive
            ? {
                role: p.roleFilter,
                block: p.blockFilter.startsWith('b:')
                  ? p.blockFilter.slice(2)
                  : '',
                block_empty: p.blockFilter === 'none',
                model: p.modelFilter,
              }
            : undefined;
        const r = filters
          ? await api.list(
              p.profile,
              p.activation,
              p.search,
              p.cursor,
              p.inactive,
              filters,
            )
          : await api.list(
              p.profile,
              p.activation,
              p.search,
              p.cursor,
              p.inactive,
            );
        if (stopped || read !== generationRef.current) return;
        if (r.code !== 'OK' || !r.data)
          return p.setListError(equipmentMessage(r.code));
        p.setListError('');
        const list = r.data.devices || [];
        p.setDevices(list);
        p.setBlocks(r.data.blocks || []);
        p.setModels(r.data.models || []);
        const tot = r.data.total_devices ?? list.length;
        p.setTotal(tot);
        p.onTotal?.([p.profile, p.activation].join('|'), tot);
        p.setNext(r.data.next_cursor);
        p.setVault(r.data.vault_available);
        p.setReady(true);
        p.setSelected((old) =>
          reconcileListedSelection(old, list, detailOpen.current),
        );
      } catch {
        if (!stopped && read === generationRef.current)
          p.setListError(equipmentMessage('SERVICE_UNAVAILABLE'));
      }
    })();
    return () => {
      stopped = true;
    };
  }, [
    latestRef,
    p.profile,
    p.activation,
    p.search,
    p.cursor,
    p.inactive,
    p.roleFilter,
    p.blockFilter,
    p.modelFilter,
    p.connected,
    p.refresh,
    p.onTotal,
    generationRef,
    p.setListError,
    p.setDevices,
    p.setBlocks,
    p.setModels,
    p.setTotal,
    p.setNext,
    p.setVault,
    p.setReady,
    p.setSelected,
  ]);
}
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
function useJobCompletionSyncEffect(p: UseJobCompletionSyncEffectProps) {
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
    void api
      .get(p.profile, p.activation, jobDeviceId)
      .then((r) => {
        if (cancelled || intent !== p.surfaceRef.current) return;
        if (r.code === 'OK' && r.data?.device?.id === jobDeviceId) {
          p.setSelected(r.data.device);
          p.setDetailOpen(true);
          p.onSelect?.(r.data.device.id);
        } else p.setError(equipmentMessage(r.code));
      })
      .catch(() => {
        if (!cancelled && intent === p.surfaceRef.current)
          p.setError(equipmentMessage('SERVICE_UNAVAILABLE'));
      });
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
