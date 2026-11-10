import {
  latestRefreshAttempt,
  refreshAttemptInProgress,
} from '../../shared/hooks/refresh-attempt-order';
import type { usePPPoEState } from './pppoe-state';
import { useRefreshOnOpen } from '../equipment/use-refresh-on-open';

export function useSessionsTabRefresh(
  s: ReturnType<typeof usePPPoEState>,
  available: boolean,
) {
  useRefreshOnOpen({
    opening: s.tab === 'sessions' ? `${s.device.id}|Sessões` : '',
    scope: `${s.profile}|${s.activation}`,
    ready: !s.loading && !s.initialFailed,
    input: {
      observedAt: s.current?.provenance.ended_at,
      device: {
        ...s.device,
        last_attempt: latestRefreshAttempt(
          s.device.last_attempt,
          s.lastAttempt
            ? { ...s.lastAttempt, at: s.lastAttempt.at || '' }
            : undefined,
        ),
      },
      vaultAvailable: available,
      serviceAvailable: available,
      reading:
        s.busy ||
        Boolean(s.job) ||
        refreshAttemptInProgress(s.lastAttempt) ||
        refreshAttemptInProgress(s.device.last_attempt),
    },
    refresh: s.update,
  });
}
