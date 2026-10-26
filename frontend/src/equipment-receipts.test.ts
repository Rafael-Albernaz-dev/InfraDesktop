import { expect, it } from 'vitest';
import {
  acceptEquipmentAuthority,
  pendingStarts,
  pendingEnrollments,
  pendingPPPoE,
} from './equipment-receipts';

it('Q01 retains every uncertain ID in the same authority, invalidates only accepted switches/logout', () => {
  acceptEquipmentAuthority('receipts-company', 'activation-one');
  for (let i = 0; i < 128; i++)
    pendingStarts.set('device' + i, 'operation' + i);
  pendingEnrollments.set('enroll', {
    id: 'device',
    operation_id: 'enrollment',
  });
  pendingPPPoE.set('collect', { operation: 'pppoe-operation' });
  acceptEquipmentAuthority('receipts-company', 'activation-one');
  expect(pendingStarts.size).toBe(128);
  expect(pendingStarts.get('device127')).toBe('operation127');
  expect(pendingPPPoE.get('collect')?.operation).toBe('pppoe-operation');
  // Re-selecting the SAME company creates a new activation in access.Service.
  acceptEquipmentAuthority('receipts-company', 'activation-two');
  expect(pendingStarts.size + pendingEnrollments.size + pendingPPPoE.size).toBe(
    0,
  );
  pendingStarts.set('device', 'uncertain');
  acceptEquipmentAuthority('', '');
  expect(pendingStarts.size).toBe(0);
});
