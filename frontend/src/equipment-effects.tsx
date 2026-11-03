import { useEffect } from 'react';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import type { Credential, Device, Job } from './equipment-api';
import { pendingStarts } from './equipment-receipts';

function reconcileListedSelection(
  old: Device | undefined,
  devices: Device[],
  detailOpen: boolean,
): Device | undefined {
  if (detailOpen) return old;
  const current =
    devices.find((d) => d.id === old?.id) ||
    (old && detailOpen ? old : devices[0]);
  return old &&
    current &&
    old.id === current.id &&
    old.revision === current.revision &&
    old.result?.observed_at === current.result?.observed_at &&
    (old.result?.interfaces?.length || detailOpen)
    ? old
    : current;
}

export function useDeviceListEffect(p: {
  profile: string;
  activation: string;
  search: string;
  cursor: number;
  inactive: boolean;
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
}) {
  const { generationRef } = p;
  useEffect(() => {
    let stopped = false;
    const read = ++generationRef.current;
    if (!p.profile || !p.connected) return;
    void (async () => {
      try {
        const r = await api.list(
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
        const tot = r.data.total_devices ?? list.length;
        p.setTotal(tot);
        p.onTotal?.([p.profile, p.activation].join('|'), tot);
        p.setNext(r.data.next_cursor);
        p.setVault(r.data.vault_available);
        p.setReady(true);
        p.setSelected((old) =>
          reconcileListedSelection(old, list, p.detailOpen),
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
    p.profile,
    p.activation,
    p.search,
    p.cursor,
    p.inactive,
    p.connected,
    p.refresh,
    p.detailOpen,
    p.onTotal,
    generationRef,
    p.setListError,
    p.setDevices,
    p.setTotal,
    p.setNext,
    p.setVault,
    p.setReady,
    p.setSelected,
  ]);
}

export function useCredentialsCatalogEffect(p: {
  profile: string;
  activation: string;
  connected: boolean;
  credentialRefresh: number;
  setCredentialDefaults: (v: Record<string, string>) => void;
  setCredentialCatalog: (v: { key: string; value: Credential[] }) => void;
  setCredentialError: (v: string) => void;
}) {
  useEffect(() => {
    if (!p.profile || !p.connected) return;
    let stopped = false;
    void (async () => {
      const all: Credential[] = [];
      let page = 0;
      do {
        const r = await api.credentials(p.profile, p.activation, page);
        if (stopped) return;
        if (r.code !== 'OK' || !r.data) throw new Error(r.code);
        all.push(...(r.data.credentials || []));
        p.setCredentialDefaults(r.data.credential_defaults || {});
        page = r.data.next_cursor;
      } while (page);
      p.setCredentialCatalog({
        key: [p.profile, p.activation].join('|'),
        value: all,
      });
      p.setCredentialError('');
    })().catch(() => {
      if (!stopped)
        p.setCredentialError(equipmentMessage('SERVICE_UNAVAILABLE'));
    });
    return () => {
      stopped = true;
    };
  }, [
    p.profile,
    p.activation,
    p.connected,
    p.credentialRefresh,
    p.setCredentialDefaults,
    p.setCredentialCatalog,
    p.setCredentialError,
  ]);
}

function clearPendingStartsForJob(
  profile: string,
  activation: string,
  current: Job,
) {
  const prefix =
    [
      profile,
      activation,
      current.kind === 'trust.inspect' ? 'inspect' : 'query',
      current.device_id,
    ].join('|') + '|';
  for (const [key, operation] of pendingStarts) {
    if (key.startsWith(prefix) && operation) pendingStarts.delete(key);
  }
}

export function useJobPollEffect(p: {
  job?: Job;
  profile: string;
  activation: string;
  surfaceRef: React.MutableRefObject<number>;
  setError: (v: string) => void;
  setJob: (v: Job | undefined) => void;
  reload: React.Dispatch<React.SetStateAction<number>>;
}) {
  useEffect(() => {
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

function useJobCompletionSyncEffect(p: {
  job?: Job;
  profile: string;
  activation: string;
  onSelect?: (id: string) => void;
  surfaceRef: React.MutableRefObject<number>;
  setSelected: (d: Device | undefined) => void;
  setDetailOpen: (v: boolean) => void;
  setError: (v: string) => void;
}) {
  useEffect(() => {
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

export function useDetailDeviceSyncEffects(p: {
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
}) {
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
