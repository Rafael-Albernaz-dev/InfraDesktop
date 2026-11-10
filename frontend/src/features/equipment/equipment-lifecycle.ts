import { useEffect, useLayoutEffect, useRef } from 'react';
import { acceptEquipmentAuthority } from '../access/equipment-receipts';
import type { UseEquipmentStateProps } from './equipment-state';
import type { useEquipmentStore } from './equipment-store';
function useEquipmentAuthorityRefs(profile: string, activation: string) {
  const defaultContextRef = useRef([profile, activation].join('|'));
  const detailSurfaceRef = useRef<HTMLElement>(null);
  const surfaceRef = useRef(0);
  const mountedRef = useRef(true);
  return { defaultContextRef, detailSurfaceRef, surfaceRef, mountedRef };
}
function useEquipmentOperationRefs() {
  const resetOperationRef = useRef('');
  const confirmOperationRef = useRef({ job: '', operation: '' });
  const detailReadRef = useRef(0);
  const generationRef = useRef(0);
  return {
    resetOperationRef,
    confirmOperationRef,
    detailReadRef,
    generationRef,
  };
}
export function useEquipmentRefs(profile: string, activation: string) {
  return {
    ...useEquipmentAuthorityRefs(profile, activation),
    ...useEquipmentOperationRefs(),
  };
}
export function useEquipmentSurfaceEffects(
  refs: ReturnType<typeof useEquipmentRefs>,
  s: ReturnType<typeof useEquipmentStore>,
  props: UseEquipmentStateProps,
) {
  const { profile, activation, addVisible, onDetailChange } = props;
  const { defaultContextRef, detailSurfaceRef, surfaceRef, mountedRef } = refs;
  const shownForm = s.form;
  useLayoutEffect(() => {
    acceptEquipmentAuthority(profile, activation);
    defaultContextRef.current = [profile, activation].join('|');
  }, [profile, activation, defaultContextRef]);
  useLayoutEffect(() => {
    if (s.detailOpen && s.selected?.id) {
      detailSurfaceRef.current?.focus({ preventScroll: true });
      detailSurfaceRef.current?.scrollIntoView?.({ block: 'start' });
    }
  }, [s.detailOpen, s.selected?.id, detailSurfaceRef]);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, [mountedRef]);
  useLayoutEffect(() => {
    surfaceRef.current += 1;
  }, [
    s.selected?.id,
    shownForm,
    s.credentialForm,
    profile,
    activation,
    addVisible,
    surfaceRef,
  ]);
  useEffect(() => {
    onDetailChange?.(s.detailOpen);
  }, [s.detailOpen, onDetailChange]);
}
