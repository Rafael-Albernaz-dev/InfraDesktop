import {
  latestRefreshAttempt,
  refreshAttemptInProgress,
} from '../../shared/hooks/refresh-attempt-order';
import type { RefreshOpeningRef } from '../equipment/use-refresh-on-open';
import type { RadioEquipmentState } from '../radio/radio-equipment-state';
import type { usePPPoEState } from '../pppoe/pppoe-state';
import { useRefreshOnOpen } from '../equipment/use-refresh-on-open';
import { useRefreshAttempt } from '../equipment/use-refresh-attempt';

export function useRadioTabRefresh(
  state: RadioEquipmentState,
  tab: string,
  available: boolean,
  refresh: () => void,
  decision?: RefreshOpeningRef,
) {
  const scope = `${state.profile}|${state.activation}`;
  const history = useRefreshAttempt(
    scope,
    state.currentDevice,
    state.radio.view?.job,
  );
  useRefreshOnOpen({
    decision,
    opening: tab === 'Resumo' ? `${state.device.id}|Resumo` : '',
    scope,
    ready: !state.loading && history.ready,
    input: {
      observedAt: state.reading?.observed_at,
      device: { ...state.currentDevice, last_attempt: history.attempt },
      vaultAvailable: available,
      serviceAvailable: available,
      reading: state.radio.busy,
    },
    refresh,
  });
}

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
