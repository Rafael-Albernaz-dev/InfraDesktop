import type { usePPPoEState } from './pppoe-state';
import type { ReadingUpdateStatus } from './reading-update-state';
type State = ReturnType<typeof usePPPoEState>;
export function observationReadingStatus(
  s: State,
  disabled: boolean,
): ReadingUpdateStatus {
  const sessions = s.tab === 'sessions';
  return {
    refreshing: sessions && Boolean(s.job),
    error: sessions ? s.collectionFailure?.reason : undefined,
    code: s.collectionFailure?.code,
    retry: s.update,
    disabled,
  };
}
export function showSeparatePPPoEError(s: State) {
  return (
    s.error &&
    !(s.tab === 'sessions' && s.error === s.collectionFailure?.reason)
  );
}
