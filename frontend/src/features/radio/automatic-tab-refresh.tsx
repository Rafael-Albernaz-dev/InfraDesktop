import type { RefreshOpeningRef } from '../equipment/use-refresh-on-open';
import type { RadioEquipmentState } from './radio-equipment-state';
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
