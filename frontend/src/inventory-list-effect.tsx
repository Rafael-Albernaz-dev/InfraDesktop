import { useEffect } from 'react';
import type { Device } from './equipment-api';
import { equipmentAPI as api, equipmentMessage } from './equipment-api';
import { useLatestRef } from './use-latest-ref';
type UseDeviceListEffectProps = {
  inventoryNode: HTMLElement | null;
  profile: string;
  activation: string;
  search: string;
  cursor: number;
  inactive: boolean;
  roleFilter: string;
  blockFilter: string;
  modelFilter: string;
  stateFilter: string;
  setPulseNetwork: (v: string) => void;
  setBlocks: (v: string[]) => void;
  setModels: (v: string[]) => void;
  connected: boolean;
  refresh: number;
  onTotal?: (context: string, total: number) => void;
  generationRef: React.MutableRefObject<number>;
  setListError: (v: string) => void;
  setDevices: (v: Device[]) => void;
  setTotal: (v: number) => void;
  setNext: (v: number) => void;
  setVault: (v: boolean) => void;
  setReady: (v: boolean) => void;
};

function listFilters(p: UseDeviceListEffectProps) {
  if (
    !p.stateFilter &&
    !p.roleFilter &&
    !p.blockFilter &&
    !p.modelFilter &&
    !p.inactive
  )
    return;
  return {
    ...(p.stateFilter ? { state: p.stateFilter } : {}),
    role: p.roleFilter,
    block: p.blockFilter.startsWith('b:') ? p.blockFilter.slice(2) : '',
    block_empty: p.blockFilter === 'none',
    model: p.modelFilter,
  };
}
async function readList(p: UseDeviceListEffectProps, current: () => boolean) {
  try {
    const filters = listFilters(p);
    const args = [
      p.profile,
      p.activation,
      p.search,
      p.cursor,
      p.inactive,
    ] as const;
    const r = filters
      ? await api.list(...args, filters)
      : await api.list(...args);
    if (!current()) return;
    if (r.code !== 'OK' || !r.data)
      return p.setListError(equipmentMessage(r.code));
    p.setListError('');
    const list = r.data.devices || [];
    p.setDevices(list);
    p.setPulseNetwork(r.data.pulse_network || '');
    p.setBlocks(r.data.blocks || []);
    p.setModels(r.data.models || []);
    const total = r.data.total_devices ?? list.length;
    p.setTotal(total);
    p.onTotal?.([p.profile, p.activation].join('|'), total);
    p.setNext(r.data.next_cursor);
    p.setVault(r.data.vault_available);
    p.setReady(true);
  } catch {
    if (current()) p.setListError(equipmentMessage('SERVICE_UNAVAILABLE'));
  }
}
export function useDeviceListEffect(p: UseDeviceListEffectProps) {
  const latest = useLatestRef(p);
  const { generationRef } = p;
  useEffect(() => {
    const p = latest.current;
    const doc = p.inventoryNode?.ownerDocument;
    if (!doc || !p.profile || !p.connected) return;
    let stopped = false;
    let pending = false;
    const read = () => {
      if (doc.visibilityState === 'hidden' || pending || stopped) return;
      pending = true;
      const generation = ++generationRef.current;
      const current = () => !stopped && generation === generationRef.current;
      void readList(p, current).finally(() => {
        if (generation === generationRef.current) pending = false;
      });
    };
    const visibility = () => {
      ++generationRef.current;
      if (doc.visibilityState !== 'hidden') {
        pending = false;
        read();
      }
    };
    read();
    const timer = setInterval(read, 5000);
    doc.addEventListener('visibilitychange', visibility);
    return () => {
      stopped = true;
      clearInterval(timer);
      doc.removeEventListener('visibilitychange', visibility);
    };
  }, [
    latest,
    generationRef,
    p.inventoryNode,
    p.profile,
    p.activation,
    p.search,
    p.cursor,
    p.inactive,
    p.roleFilter,
    p.blockFilter,
    p.modelFilter,
    p.stateFilter,
    p.connected,
    p.refresh,
    p.onTotal,
  ]);
}
