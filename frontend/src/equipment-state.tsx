import {
  useEffect,
  useEffectEvent,
  useLayoutEffect,
  useRef,
  useState,
} from 'react';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import type { Credential, Device, EquipmentResult, Job } from './equipment-api';
import {
  useCredentialsCatalogEffect,
  useDetailDeviceSyncEffects,
  useDeviceListEffect,
  useJobPollEffect,
} from './equipment-effects';
import { acceptEquipmentAuthority, pendingStarts } from './equipment-receipts';
import { id } from './equipment-reading';
import { useDisplayNow } from './equipment-snapshot';
import { fieldForEquipmentCode } from './equipment-validation';
import { RadioRoles } from './radio-contract';

function retireOlderPendingRevisions(
  profile: string,
  activation: string,
  device: Device,
) {
  for (const operationKind of ['query', 'inspect']) {
    const prefix =
      [profile, activation, operationKind, device.id].join('|') + '|';
    for (const old of pendingStarts.keys()) {
      const oldRevision = Number(old.slice(prefix.length));
      if (
        old.startsWith(prefix) &&
        Number.isSafeInteger(oldRevision) &&
        Number.isSafeInteger(device.revision) &&
        oldRevision < device.revision
      )
        pendingStarts.delete(old);
    }
  }
}

function createPerformAction(p: {
  busy: boolean;
  setBusy: (v: boolean) => void;
  setError: (v: string) => void;
  surface: React.MutableRefObject<number>;
  mounted: React.MutableRefObject<boolean>;
  reload: React.Dispatch<React.SetStateAction<number>>;
}) {
  return async function perform(
    call: () => Promise<EquipmentResult>,
    after?: (r: EquipmentResult) => void,
    refreshList = true,
    inlineFields = false,
  ) {
    if (p.busy) return;
    p.setBusy(true);
    p.setError('');
    const intent = p.surface.current;
    try {
      const r = await call();
      if (!p.mounted.current) return;
      if (r.code === 'OK') {
        if (refreshList) p.reload((n) => n + 1);
        if (intent === p.surface.current) after?.(r);
      } else if (
        intent === p.surface.current &&
        !(inlineFields && fieldForEquipmentCode[r.code])
      )
        p.setError(equipmentMessage(r.code));
      return r;
    } catch {
      if (p.mounted.current && intent === p.surface.current)
        p.setError(equipmentMessage('SERVICE_UNAVAILABLE'));
    } finally {
      if (p.mounted.current) p.setBusy(false);
    }
  };
}

function createStartOperation(p: {
  profile: string;
  activation: string;
  setError: (v: string) => void;
  setJobDevice: (d: Device | undefined) => void;
  setJob: (j: Job | undefined) => void;
  perform: ReturnType<typeof createPerformAction>;
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
      (r) => {
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
      device.fingerprint &&
      (device.role === RadioRoles.ap || device.role === RadioRoles.cpe)
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

function useEquipmentStore(
  profile: string,
  activation: string,
  targetID?: string,
) {
  const [credentialRefresh, reloadCredentials] = useState(0);
  const defaultContext = useRef([profile, activation].join('|'));
  const [credentialDefaults, setCredentialDefaults] = useState<
    Record<string, string>
  >({});
  const [defaultBusy, setDefaultBusy] = useState(false);
  const [credentialError, setCredentialError] = useState('');
  const [ppOpen, setPPOpen] = useState(false);
  const detailSurface = useRef<HTMLElement>(null);
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
  const [detailOpen, setDetailOpen] = useState(Boolean(targetID)),
    [connectRequest, setConnectRequest] = useState<Device>(),
    [resetTrust, setResetTrust] = useState(false),
    [jobDevice, setJobDevice] = useState<Device>(),
    [consumedCreate, consumeCreate] = useState(0);
  const resetOperation = useRef(''),
    confirmOperation = useRef({ job: '', operation: '' }),
    detailRead = useRef(0),
    surface = useRef(0),
    mounted = useRef(true),
    generation = useRef(0);
  return {
    credentialRefresh,
    reloadCredentials,
    defaultContext,
    credentialDefaults,
    setCredentialDefaults,
    defaultBusy,
    setDefaultBusy,
    credentialError,
    setCredentialError,
    ppOpen,
    setPPOpen,
    detailSurface,
    displayNow,
    devices,
    setDevices,
    setCredentialCatalog,
    credentials,
    selected,
    setSelected,
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
    detailOpen,
    setDetailOpen,
    connectRequest,
    setConnectRequest,
    resetTrust,
    setResetTrust,
    resetOperation,
    jobDevice,
    setJobDevice,
    confirmOperation,
    detailRead,
    surface,
    mounted,
    generation,
    consumedCreate,
    consumeCreate,
  };
}

function createEquipmentActions(
  s: ReturnType<typeof useEquipmentStore>,
  profile: string,
  activation: string,
  closeForm: () => void,
  perform: ReturnType<typeof createPerformAction>,
  onSelect?: (id: string) => void,
) {
  const openDevice = (device: Device) => {
    s.setSelected(device);
    s.setDetailOpen(true);
    s.setJob(undefined);
    s.setError('');
    onSelect?.(device.id);
  };
  const openById = (id: string) => {
    const dev = s.devices.find((d) => d.id === id);
    if (dev) {
      openDevice(dev);
      return;
    }
    void api.get(profile, activation, id).then((r) => {
      if (r.code === 'OK' && r.data?.device) openDevice(r.data.device);
    });
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
    openById,
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
    onTotal,
    targetID,
    onSelect,
    onDetailChange,
  } = props;
  const s = useEquipmentStore(profile, activation, targetID);
  useLayoutEffect(() => {
    acceptEquipmentAuthority(profile, activation);
    s.defaultContext.current = [profile, activation].join('|');
  }, [profile, activation, s.defaultContext]);
  useLayoutEffect(() => {
    if (s.detailOpen && s.selected?.id && !s.ppOpen) {
      s.detailSurface.current?.focus({ preventScroll: true });
      s.detailSurface.current?.scrollIntoView?.({ block: 'start' });
    }
  }, [s.detailOpen, s.selected?.id, s.ppOpen, s.detailSurface]);
  useEffect(() => {
    s.mounted.current = true;
    return () => {
      s.mounted.current = false;
    };
  }, [s.mounted]);
  const shownForm =
    createRequest > 0 && createRequest !== s.consumedCreate ? 'new' : s.form;
  const closeForm = () => {
    s.setError('');
    s.setForm(undefined);
    s.consumeCreate(createRequest);
  };
  useLayoutEffect(() => {
    s.surface.current += 1;
  }, [
    s.selected?.id,
    shownForm,
    s.credentialForm,
    profile,
    activation,
    s.surface,
  ]);
  useDeviceListEffect({ ...s, profile, activation, connected, onTotal });
  useCredentialsCatalogEffect({ ...s, profile, activation, connected });
  useJobPollEffect({ ...s, profile, activation });
  const perform = createPerformAction(s);
  const startOperation = createStartOperation({
    ...s,
    profile,
    activation,
    perform,
  });
  useDetailDeviceSyncEffects({
    ...s,
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
  return { ...s, ...actions, shownForm, closeForm, perform, startOperation };
}
