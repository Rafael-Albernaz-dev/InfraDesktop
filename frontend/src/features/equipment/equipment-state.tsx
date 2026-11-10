import { useCallback, useEffect, useEffectEvent } from 'react';
import type { Device, EquipmentResult, Job } from './equipment-api';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import { useCredentialsCatalogEffect } from './equipment-credential-effect';
import {
  useDetailDeviceSyncEffects,
  useDeviceListEffect,
  useJobPollEffect,
} from './equipment-effects';
import {
  useEquipmentRefs,
  useEquipmentSurfaceEffects,
} from './equipment-lifecycle';
import { useEquipmentNavigator } from './equipment-navigator';
import { retireOlderPendingRevisions } from './equipment-pending-revisions';
import { id } from './equipment-reading-format';
import { pendingStarts } from './equipment-receipts';
import { driverOwners } from './equipment-role-options';
import { useEquipmentStore } from './equipment-store';
import { fieldForEquipmentCode } from './equipment-validation';
import type { LinkedSessionTarget } from '../pppoe/linked-session-navigation';
import { RadioRoles } from './radio-contract';

export type EquipmentPerformFn = (
  call: () => Promise<EquipmentResult>,
  after?: (r: EquipmentResult) => void,
  refreshList?: boolean,
  inlineFields?: boolean,
) => Promise<EquipmentResult | undefined>;

type EquipmentPerformContext = {
  busy: boolean;
  setBusy: (v: boolean) => void;
  setError: (v: string) => void;
  surfaceRef: React.MutableRefObject<number>;
  mountedRef: React.MutableRefObject<boolean>;
  reload: React.Dispatch<React.SetStateAction<number>>;
};
function usePerformAction({
  busy,
  setBusy,
  setError,
  surfaceRef,
  mountedRef,
  reload,
}: EquipmentPerformContext): EquipmentPerformFn {
  return useCallback(
    async function perform(
      call: () => Promise<EquipmentResult>,
      after?: (r: EquipmentResult) => void,
      refreshList = true,
      inlineFields = false,
    ) {
      if (busy) return;
      setBusy(true);
      setError('');
      const intent = surfaceRef.current;
      try {
        const r = await call();
        if (!mountedRef.current) return;
        if (r.code === 'OK') {
          if (refreshList) reload((n) => n + 1);
          if (intent === surfaceRef.current) after?.(r);
        } else if (
          intent === surfaceRef.current &&
          !isInlineFieldError(inlineFields, r.code)
        )
          setError(equipmentMessage(r.code));
        return r;
      } catch {
        if (mountedRef.current && intent === surfaceRef.current)
          setError(equipmentMessage('SERVICE_UNAVAILABLE'));
      } finally {
        if (mountedRef.current) setBusy(false);
      }
    },
    [busy, setBusy, setError, surfaceRef, mountedRef, reload],
  );
}

type CreateStartOperationProps = {
  profile: string;
  activation: string;
  setError: (v: string) => void;
  setJobDevice: (d: Device | undefined) => void;
  setJob: (j: Job | undefined) => void;
  perform: EquipmentPerformFn;
};
function createStartOperation(p: CreateStartOperationProps) {
  return async function startOperation(
    kind: 'query' | 'inspect',
    device: Device,
  ) {
    const key = [
      p.profile,
      p.activation,
      kind,
      device.id,
      device.revision,
    ].join('|');
    retireOlderPendingRevisions(p.profile, p.activation, device);
    if (!pendingStarts.has(key) && pendingStarts.size >= 128) {
      p.setError(equipmentMessage('LIMIT_EXCEEDED'));
      return;
    }
    const operation = pendingStarts.get(key) || id();
    pendingStarts.set(key, operation);
    await p.perform(
      async () => {
        const r = await (kind === 'query' ? api.start : api.inspect)(
          p.profile,
          p.activation,
          device.id,
          operation,
          device.revision,
        );
        if (!['SERVICE_UNAVAILABLE', 'DEADLINE_EXCEEDED'].includes(r.code))
          if (r.code !== 'OK' || r.data?.job?.state !== 'RUNNING')
            pendingStarts.delete(key);
        return r;
      },
      (r: EquipmentResult) => {
        p.setJobDevice(device);
        p.setJob(r.data?.job);
        if (r.data?.job && r.data.job.code !== 'OK')
          p.setError(equipmentMessage(r.data.job.code));
      },
    );
  };
}

function useQueuedConnectEffect(
  connectRequest: Device | undefined,
  setConnectRequest: (d: Device | undefined) => void,
  busy: boolean,
  connected: boolean,
  startOperation: (kind: 'query' | 'inspect', d: Device) => Promise<void>,
) {
  const queuedConnect = useEffectEvent((device: Device) => {
    setConnectRequest(undefined);
    if (
      Boolean(driverOwners[device.role || '']) ||
      (device.fingerprint &&
        (device.role === RadioRoles.ap || device.role === RadioRoles.cpe))
    )
      return;
    void startOperation(device.fingerprint ? 'query' : 'inspect', device);
  });
  useEffect(() => {
    if (!connectRequest || busy || !connected) return;
    const device = connectRequest;
    let stopped = false;
    queueMicrotask(() => {
      if (!stopped) queuedConnect(device);
    });
    return () => {
      stopped = true;
    };
  }, [connectRequest, busy, connected]);
}

type EquipmentActionContext = {
  profile: string;
  activation: string;
  closeForm: () => void;
  perform: EquipmentPerformFn;
  onSelect?: (id: string) => void;
};
function createEquipmentActions(
  s: ReturnType<typeof useEquipmentStore>,
  p: EquipmentActionContext,
) {
  const { profile, activation, closeForm, perform, onSelect } = p;

  const openDevice = (device: Device, target?: LinkedSessionTarget) => {
    s.setSessionTarget(target);
    s.setSelected(device);
    s.setDetailOpen(true);
    s.setJob(undefined);
    s.setError('');
    onSelect?.(device.id);
  };
  const dismissTrust = () => {
    if (!s.job || s.busy) return;
    void perform(
      () => api.cancel(profile, activation, s.job!.id),
      () => s.setJob(undefined),
      false,
    );
  };
  const showSavedDevice = (r: EquipmentResult) => {
    closeForm();
    if (r.data?.device) openDevice(r.data.device);
  };
  return {
    openDevice,
    dismissTrust,
    showSavedDevice,
  };
}
export type UseEquipmentStateProps = {
  inventoryNode?: HTMLElement | null;
  profile: string;
  activation: string;
  connected: boolean;
  addVisible?: boolean;
  onTotal?: (context: string, total: number) => void;
  targetID?: string;
  onSelect?: (id: string) => void;
  onDetailChange?: (open: boolean) => void;
};
export function useEquipmentState(props: UseEquipmentStateProps) {
  const { profile, activation, connected, onTotal, targetID, onSelect } = props;
  const refs = useEquipmentRefs(profile, activation);
  const {
    defaultContextRef,
    surfaceRef,
    mountedRef,
    detailReadRef,
    generationRef,
  } = refs;
  const s = useEquipmentStore(profile, activation);
  useEquipmentSurfaceEffects(refs, s, props);
  const shownForm = s.form;
  const closeForm = () => {
    s.setError('');
    s.setForm(undefined);
  };
  useDeviceListEffect({
    ...s,
    inventoryNode: props.inventoryNode ?? null,
    generationRef,
    profile,
    activation,
    connected,
    onTotal,
  });
  useCredentialsCatalogEffect({ ...s, profile, activation, connected });
  useJobPollEffect({ ...s, surfaceRef, profile, activation });
  const perform = usePerformAction({
    busy: s.busy,
    setBusy: s.setBusy,
    setError: s.setError,
    surfaceRef,
    mountedRef,
    reload: s.reload,
  });
  const startOperation = createStartOperation({
    ...s,
    profile,
    activation,
    perform,
  });
  useDetailDeviceSyncEffects({
    ...s,
    detailReadRef,
    surfaceRef,
    targetID,
    profile,
    activation,
    connected,
    onSelect,
  });
  useQueuedConnectEffect(
    s.connectRequest,
    s.setConnectRequest,
    s.busy,
    connected,
    startOperation,
  );
  const actions = createEquipmentActions(s, {
    profile,
    activation,
    closeForm,
    perform,
    onSelect,
  });
  const openById = useEquipmentNavigator({
    profile,
    activation,
    scopeRef: defaultContextRef,
    surfaceRef,
    mountedRef,
    open: actions.openDevice,
    error: s.setError,
  });
  return {
    ...s,
    ...actions,
    openById,
    openSession: (target: LinkedSessionTarget) =>
      openById(target.deviceID, { ...target, originDevice: s.selected?.id }),
    shownForm,
    closeForm,
    perform,
    startOperation,
    ...refs,
    confirmOperation: refs.confirmOperationRef,
    resetOperation: refs.resetOperationRef,
  };
}

function isInlineFieldError(inlineFields: boolean, code: string) {
  return inlineFields && Boolean(fieldForEquipmentCode[code]);
}
