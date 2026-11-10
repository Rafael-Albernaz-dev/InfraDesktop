import type { Props } from './equipment-detail';
import { pppoeAPI } from '../../platform/wails/presentation';
import { refreshAttemptInProgress } from '../../shared/hooks/refresh-attempt-order';
import { useOpeningAuthority } from '../../shared/hooks/use-opening-authority';
import { useRefreshOnOpen } from '../radio/use-refresh-on-open';

export async function refreshUnlessCollecting(
  p: Props,
  current: () => boolean = () => true,
) {
  if (p.selected.role === 'concentrator') {
    const r = await pppoeAPI.current(p.profile, p.activation, p.selected.id);
    if (r.code !== 'OK' || refreshAttemptInProgress(r.data?.attempt)) return;
  }
  if (current()) await p.startOperation('query', p.selected);
}

export function useSummaryRefresh(p: Props, tab: string, isRadio: boolean) {
  const captureAuthority = useOpeningAuthority(
    [p.profile, p.activation, p.selected.id, tab].join('|'),
  );
  useRefreshOnOpen({
    opening: !isRadio && tab === 'Resumo' ? `${p.selected.id}|Resumo` : '',
    scope: `${p.profile}|${p.activation}`,
    ready: p.ready,
    input: {
      observedAt: p.selected.result?.observed_at,
      device: p.selected,
      vaultAvailable: p.vault,
      serviceAvailable: p.connected,
      reading: p.busy || p.job?.state === 'RUNNING',
    },
    refresh: () => void refreshUnlessCollecting(p, captureAuthority()),
  });
}
