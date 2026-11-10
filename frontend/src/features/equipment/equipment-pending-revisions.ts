import type { Device, Job } from './equipment-api';
import { pendingStarts } from '../access/equipment-receipts';
export function retireOlderPendingRevisions(
  profile: string,
  activation: string,
  device: Device,
) {
  for (const operationKind of ['query', 'inspect']) {
    const prefix =
      [profile, activation, operationKind, device.id].join('|') + '|';
    for (const old of pendingStarts.keys()) {
      const oldRevision = Number(old.slice(prefix.length));
      if (
        old.startsWith(prefix) &&
        Number.isSafeInteger(oldRevision) &&
        Number.isSafeInteger(device.revision) &&
        oldRevision < device.revision
      )
        pendingStarts.delete(old);
    }
  }
}

export function clearPendingStartsForJob(
  profile: string,
  activation: string,
  current: Job,
) {
  const prefix =
    [
      profile,
      activation,
      current.kind === 'trust.inspect' ? 'inspect' : 'query',
      current.device_id,
    ].join('|') + '|';
  for (const [key, operation] of pendingStarts) {
    if (key.startsWith(prefix) && operation) pendingStarts.delete(key);
  }
}
