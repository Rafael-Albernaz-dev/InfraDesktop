import { useRef } from 'react';
import type { PPPoERefs } from './pppoe-state';
function useDetailRefs() {
  const detailTriggerRef = useRef<HTMLElement | null>(null);
  const detailHeadingRef = useRef<HTMLHeadingElement>(null);
  const pairHeadingRef = useRef<HTMLHeadingElement>(null);
  const focusedControlRef = useRef<HTMLElement | null>(null);
  const searchControlRef = useRef<HTMLInputElement>(null);
  return {
    detailTriggerRef,
    detailHeadingRef,
    pairHeadingRef,
    focusedControlRef,
    searchControlRef,
  };
}
function useExportRefs() {
  const referencesHeadingRef = useRef<HTMLHeadingElement>(null);
  const exportControlRef = useRef<HTMLButtonElement>(null);
  const csvExportControlRef = useRef<HTMLButtonElement>(null);
  const removedFocusRef = useRef('');
  const exportFocusRef = useRef<'sanitized' | 'operational' | ''>('');
  return {
    referencesHeadingRef,
    exportControlRef,
    csvExportControlRef,
    removedFocusRef,
    exportFocusRef,
  };
}
function useReadRefs() {
  const lifeRef = useRef({ generation: 0 });
  const primaryRef = useRef<HTMLButtonElement>(null);
  const detailIntentRef = useRef(0);
  return { lifeRef, primaryRef, detailIntentRef };
}
export function usePPPoERefs(): PPPoERefs {
  return { ...useDetailRefs(), ...useExportRefs(), ...useReadRefs() };
}
