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
  generation: React.MutableRefObject<number>;
  setListError: (v: string) => void;
  setDevices: (v: Device[]) => void;
  setTotal: (v: number) => void;
  setNext: (v: number) => void;
  setVault: (v: boolean) => void;
  setReady: (v: boolean) => void;
  setSelected: React.Dispatch<React.SetStateAction<Device | undefined>>;
}) {
  useEffect(() => {
    let stopped = false;
    const read = ++p.generation.current;
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
        if (stopped || read !== p.generation.current) return;
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
        if (!stopped && read === p.generation.current)
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
    p.generation,
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
  surface: React.MutableRefObject<number>;
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
    const intent = p.surface.current;
    const failed = (code: string) => {
      if (intent === p.surface.current)
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
        if (failures && intent === p.surface.current) p.setError('');
        failures = 0;
        if (current.state !== 'RUNNING') {
          clearPendingStartsForJob(p.profile, p.activation, current);
          p.setJob(intent === p.surface.current ? current : undefined);
          if (current.code !== 'OK' && intent === p.surface.current)
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
    p.surface,
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
  detailRead: React.MutableRefObject<number>,
  surface: React.MutableRefObject<number>,
  setSelected: (d: Device | undefined) => void,
  setDetailOpen: (v: boolean) => void,
  setError: (v: string) => void,
) {
  let cancelled = false;
  const read = ++detailRead.current;
  const intent = surface.current;
  void api
    .get(profile, activation, requested)
    .then((r) => {
      if (
        cancelled ||
        read !== detailRead.current ||
        intent !== surface.current
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
        read === detailRead.current &&
        intent === surface.current
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
  surface: React.MutableRefObject<number>;
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
    const intent = p.surface.current;
    void api
      .get(p.profile, p.activation, jobDeviceId)
      .then((r) => {
        if (cancelled || intent !== p.surface.current) return;
        if (r.code === 'OK' && r.data?.device?.id === jobDeviceId) {
          p.setSelected(r.data.device);
          p.setDetailOpen(true);
          p.onSelect?.(r.data.device.id);
        } else p.setError(equipmentMessage(r.code));
      })
      .catch(() => {
        if (!cancelled && intent === p.surface.current)
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
    p.surface,
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
  detailRead: React.MutableRefObject<number>;
  surface: React.MutableRefObject<number>;
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
      p.detailRead,
      p.surface,
      p.setSelected,
      p.setDetailOpen,
      p.setError,
    );
  }, [
    p.targetID,
    p.profile,
    p.activation,
    p.connected,
    p.detailRead,
    p.surface,
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
      p.detailRead,
      p.surface,
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
    p.detailRead,
    p.surface,
    p.setSelected,
    p.setDetailOpen,
    p.setError,
  ]);
  useJobCompletionSyncEffect(p);
}
