import type { Device } from './equipment-api';
import { pendingStarts } from './equipment-receipts';
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
