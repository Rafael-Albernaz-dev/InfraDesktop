import { useCredentialsCatalogEffect } from './equipment-credential-effect';
import { retireOlderPendingRevisions } from './equipment-pending-revisions';
import { useInventoryFilterState } from './inventory-filter-state';
import { useEquipmentNavigator } from './equipment-navigator';
import type { LinkedSessionTarget } from './linked-session-navigation';
import {
  useCallback,
  useEffect,
  useEffectEvent,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import type { Credential, Device, EquipmentResult, Job } from './equipment-api';
import {
  useDetailDeviceSyncEffects,
  useDeviceListEffect,
  useJobPollEffect,
} from './equipment-effects';
import { acceptEquipmentAuthority, pendingStarts } from './equipment-receipts';
import { id } from './equipment-reading';
import { useDisplayNow } from './equipment-snapshot';
import { fieldForEquipmentCode } from './equipment-validation';
import { RadioRoles } from './radio-contract';
import { driverOwners } from './equipment-roles';

export type EquipmentPerformFn = (
  call: () => Promise<EquipmentResult>,
  after?: (r: EquipmentResult) => void,
  refreshList?: boolean,
  inlineFields?: boolean,
) => Promise<EquipmentResult | undefined>;

function usePerformAction(
  busy: boolean,
  setBusy: (v: boolean) => void,
  setError: (v: string) => void,
  surfaceRef: React.MutableRefObject<number>,
  mountedRef: React.MutableRefObject<boolean>,
  reload: React.Dispatch<React.SetStateAction<number>>,
): EquipmentPerformFn {
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
          !(inlineFields && fieldForEquipmentCode[r.code])
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

function createStartOperation(p: {
  profile: string;
  activation: string;
  setError: (v: string) => void;
  setJobDevice: (d: Device | undefined) => void;
  setJob: (j: Job | undefined) => void;
  perform: EquipmentPerformFn;
}) {
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
      (device.fingerprint && (device.role === RadioRoles.ap || device.role === RadioRoles.cpe))
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

function useEquipmentStore(profile: string, activation: string) {
  const inventoryFilters = useInventoryFilterState();
  const [credentialRefresh, reloadCredentials] = useState(0);
  const [credentialDefaults, setCredentialDefaults] = useState<
    Record<string, string>
  >({});
  const [defaultBusy, setDefaultBusy] = useState(false);
  const [credentialError, setCredentialError] = useState('');
  const displayNow = useDisplayNow();
  const [devices, setDevices] = useState<Device[]>([]);
  const [credentialCatalog, setCredentialCatalog] = useState<{
    key: string;
    value: Credential[];
  }>({ key: '', value: [] });
  const credentials =
    credentialCatalog.key === [profile, activation].join('|')
      ? credentialCatalog.value
      : [];
  const [sessionTarget, setSessionTarget] = useState<LinkedSessionTarget>();
  const [selected, setSelected] = useState<Device>(),
    [search, setSearch] = useState(''),
    [inactive, setInactive] = useState(false);
  const [cursor, setCursor] = useState(0),
    [next, setNext] = useState(0),
    [refresh, reload] = useState(0),
    [total, setTotal] = useState(0);
  const [ready, setReady] = useState(false),
    [vault, setVault] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [listError, setListError] = useState('');
  const [form, setForm] = useState<Device | 'new'>(),
    [credentialForm, setCredentialForm] = useState<Credential | 'new'>(),
    [job, setJob] = useState<Job>();
  const [connectRequest, setConnectRequest] = useState<Device>(),
    [resetTrust, setResetTrust] = useState(false),
    [jobDevice, setJobDevice] = useState<Device>(),
    [consumedCreate, consumeCreate] = useState(0);
  return {
    ...inventoryFilters,
    credentialRefresh,
    reloadCredentials,
    credentialDefaults,
    setCredentialDefaults,
    defaultBusy,
    setDefaultBusy,
    credentialError,
    setCredentialError,
    displayNow,
    devices,
    setDevices,
    setCredentialCatalog,
    credentials,
    selected,
    setSelected,
    sessionTarget,
    setSessionTarget,
    search,
    setSearch,
    inactive,
    setInactive,
    cursor,
    setCursor,
    next,
    setNext,
    refresh,
    reload,
    total,
    setTotal,
    ready,
    setReady,
    vault,
    setVault,
    busy,
    setBusy,
    error,
    setError,
    listError,
    setListError,
    form,
    setForm,
    credentialForm,
    setCredentialForm,
    job,
    setJob,
    detailOpen: Boolean(selected),
    setDetailOpen: useCallback((open: boolean) => {
      if (!open) setSelected(undefined);
    }, []),
    connectRequest,
    setConnectRequest,
    resetTrust,
    setResetTrust,
    jobDevice,
    setJobDevice,
    consumedCreate,
    consumeCreate,
  };
}

function createEquipmentActions(
  s: ReturnType<typeof useEquipmentStore>,
  profile: string,
  activation: string,
  closeForm: () => void,
  perform: EquipmentPerformFn,
  onSelect?: (id: string) => void,
) {
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
  const continueEnrollment = (r: EquipmentResult) => {
    showSavedDevice(r);
    if (r.data?.device) s.setConnectRequest(r.data.device);
  };
  return {
    openDevice,
    dismissTrust,
    showSavedDevice,
    continueEnrollment,
  };
}
export function useEquipmentState(props: {
  profile: string;
  activation: string;
  connected: boolean;
  createRequest: number;
  addVisible?: boolean;
  onTotal?: (context: string, total: number) => void;
  targetID?: string;
  onSelect?: (id: string) => void;
  onDetailChange?: (open: boolean) => void;
}) {
  const {
    profile,
    activation,
    connected,
    createRequest,
    addVisible,
    onTotal,
    targetID,
    onSelect,
    onDetailChange,
  } = props;
  const defaultContextRef = useRef([profile, activation].join('|'));
  const detailSurfaceRef = useRef<HTMLElement>(null);
  const resetOperationRef = useRef('');
  const confirmOperationRef = useRef({ job: '', operation: '' });
  const detailReadRef = useRef(0);
  const surfaceRef = useRef(0);
  const mountedRef = useRef(true);
  const generationRef = useRef(0);
  const s = useEquipmentStore(profile, activation);
  useLayoutEffect(() => {
    acceptEquipmentAuthority(profile, activation);
    defaultContextRef.current = [profile, activation].join('|');
  }, [profile, activation]);
  useLayoutEffect(() => {
    if (s.detailOpen && s.selected?.id) {
      detailSurfaceRef.current?.focus({ preventScroll: true });
      detailSurfaceRef.current?.scrollIntoView?.({ block: 'start' });
    }
  }, [s.detailOpen, s.selected?.id]);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);
  const shownForm =
    createRequest > 0 && createRequest !== s.consumedCreate ? 'new' : s.form;
  const closeForm = () => {
    s.setError('');
    s.setForm(undefined);
    s.consumeCreate(createRequest);
  };
  useLayoutEffect(() => {
    surfaceRef.current += 1;
  }, [s.selected?.id, shownForm, s.credentialForm, profile, activation, addVisible]);
  useDeviceListEffect({
    ...s,
    generationRef,
    profile,
    activation,
    connected,
    onTotal,
  });
  useCredentialsCatalogEffect({ ...s, profile, activation, connected });
  useJobPollEffect({ ...s, surfaceRef, profile, activation });
  const perform = usePerformAction(
    s.busy,
    s.setBusy,
    s.setError,
    surfaceRef,
    mountedRef,
    s.reload,
  );
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
  useEffect(() => {
    onDetailChange?.(s.detailOpen);
  }, [s.detailOpen, onDetailChange]);
  const actions = createEquipmentActions(
    s,
    profile,
    activation,
    closeForm,
    perform,
    onSelect,
  );
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
      openById(target.deviceID, target),
    shownForm,
    closeForm,
    perform,
    startOperation,
    defaultContextRef,
    detailSurfaceRef,
    resetOperationRef,
    confirmOperationRef,
    confirmOperation: confirmOperationRef,
    resetOperation: resetOperationRef,
    detailReadRef,
    surfaceRef,
    mountedRef,
    generationRef,
  };
}
