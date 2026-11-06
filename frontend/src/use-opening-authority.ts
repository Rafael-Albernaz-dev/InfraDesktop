import { useCallback, useLayoutEffect, useRef } from 'react';
// StrictMode remounts retain an opening; a different opening revokes its pending continuation.
export function useOpeningAuthority(key: string) {
  const owner = useRef({ key, active: true });
  useLayoutEffect(() => {
    if (owner.current.key !== key) owner.current = { key, active: true };
    else owner.current.active = true;
    const opened = owner.current;
    return () => {
      opened.active = false;
    };
  }, [key]);
  return useCallback(() => {
    const opened = owner.current;
    return () => opened.active;
  }, []);
}
