import type { Props } from './equipment-detail';
import { pppoeAPI } from './presentation';
import { refreshAttemptInProgress } from './refresh-attempt-order';
import { useRefreshOnOpen } from './use-refresh-on-open';


export function equipmentReadBlocked(p: Props) {
  return (
    p.locked ||
    !p.vault ||
    !p.selected.active ||
    !p.selected.credential_id ||
    p.job?.state === 'RUNNING'
  );
}


export function equipmentQueryRunning(p: Props) {
  return (
    p.busy || (p.job?.state === 'RUNNING' && p.job.device_id === p.selected.id)
  );
}

export async function refreshUnlessCollecting(p: Props) {
  if (p.selected.role === 'concentrator') {
    const r = await pppoeAPI.current(p.profile, p.activation, p.selected.id);
    if (r.code !== 'OK' || refreshAttemptInProgress(r.data?.attempt)) return;
  }
  await p.startOperation('query', p.selected);
}


export function useSummaryRefresh(p: Props, tab: string, isRadio: boolean) {
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
    refresh: () => void refreshUnlessCollecting(p),
  });
}