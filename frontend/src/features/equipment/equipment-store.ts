import { useCallback, useState } from 'react';
import type { Credential, Device, Job } from './equipment-api';
import { useDisplayNow } from './equipment-snapshot';
import { useInventoryFilterState } from '../inventory/inventory-filter-state';
import type { LinkedSessionTarget } from '../pppoe/linked-session-navigation';
function useCredentialStore(profile: string, activation: string) {
  const [credentialRefresh, reloadCredentials] = useState(0);
  const [credentialDefaults, setCredentialDefaults] = useState<
    Record<string, string>
  >({});
  const [defaultBusy, setDefaultBusy] = useState(false);
  const [credentialError, setCredentialError] = useState('');
  const [credentialCatalog, setCredentialCatalog] = useState<{
    key: string;
    value: Credential[];
  }>({ key: '', value: [] });
  const credentials =
    credentialCatalog.key === [profile, activation].join('|')
      ? credentialCatalog.value
      : [];
  return {
    credentialRefresh,
    reloadCredentials,
    credentialDefaults,
    setCredentialDefaults,
    defaultBusy,
    setDefaultBusy,
    credentialError,
    setCredentialError,
    setCredentialCatalog,
    credentials,
  };
}
function useEquipmentListStore() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [search, setSearch] = useState('');
  const [inactive, setInactive] = useState(false);
  const [cursor, setCursor] = useState(0);
  const [next, setNext] = useState(0);
  const [refresh, reload] = useState(0);
  const [total, setTotal] = useState(0);
  const [ready, setReady] = useState(false);
  return {
    devices,
    setDevices,
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
  };
}
function useEquipmentSelectionStore() {
  const [sessionTarget, setSessionTarget] = useState<LinkedSessionTarget>();
  const [selected, setSelected] = useState<Device>();
  const [form, setForm] = useState<Device>();
  const [credentialForm, setCredentialForm] = useState<Credential | 'new'>();
  const [resetTrust, setResetTrust] = useState(false);
  const setDetailOpen = useCallback((open: boolean) => {
    if (!open) setSelected(undefined);
  }, []);
  return {
    sessionTarget,
    setSessionTarget,
    selected,
    setSelected,
    form,
    setForm,
    credentialForm,
    setCredentialForm,
    resetTrust,
    setResetTrust,
    detailOpen: Boolean(selected),
    setDetailOpen,
  };
}
function useEquipmentJobStore() {
  const [vault, setVault] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [listError, setListError] = useState('');
  const [job, setJob] = useState<Job>();
  const [connectRequest, setConnectRequest] = useState<Device>();
  const [jobDevice, setJobDevice] = useState<Device>();
  return {
    vault,
    setVault,
    busy,
    setBusy,
    error,
    setError,
    listError,
    setListError,
    job,
    setJob,
    connectRequest,
    setConnectRequest,
    jobDevice,
    setJobDevice,
  };
}
export function useEquipmentStore(profile: string, activation: string) {
  const inventoryFilters = useInventoryFilterState();
  const displayNow = useDisplayNow();
  return {
    ...inventoryFilters,
    displayNow,
    ...useCredentialStore(profile, activation),
    ...useEquipmentListStore(),
    ...useEquipmentSelectionStore(),
    ...useEquipmentJobStore(),
  };
}
