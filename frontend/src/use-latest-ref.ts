import { useLayoutEffect, useRef } from 'react';
// Effects keep their scalar owner while callbacks observe the latest presentation values.
export function useLatestRef<T>(value: T) {
  const ref = useRef(value);
  useLayoutEffect(() => {
    ref.current = value;
  }, [value]);
  return ref;
}
