import { useRef, useMemo } from 'react';
import type { Result } from './presentation';
function useSessionAuthorityRefs(initial: Result) {
  const hadAuthRef = useRef(false),
    contextKeyRef = useRef(''),
    previousViewRef = useRef(initial);
  return useMemo(
    () => ({ hadAuthRef, contextKeyRef, previousViewRef }),
    [hadAuthRef, contextKeyRef, previousViewRef],
  );
}
export function useSessionRefs(initial: Result) {
  const securityLockedRef = useRef(false),
    epochRef = useRef(0),
    workingRef = useRef(false),
    surfaceRef = useRef(0);
  const authority = useSessionAuthorityRefs(initial);
  return useMemo(
    () => ({
      securityLockedRef,
      epochRef,
      workingRef,
      surfaceRef,
      ...authority,
    }),
    [securityLockedRef, epochRef, workingRef, surfaceRef, authority],
  );
}
