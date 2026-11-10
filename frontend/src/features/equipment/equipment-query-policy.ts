import type { Props } from './equipment-detail';
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
