import { expect, it } from 'vitest';
import type { Device } from '../../features/equipment/equipment-api';
import { latestSavedReading } from '../../features/equipment/equipment-reading-format';
import { RadioRoles } from '../../features/equipment/radio-contract';

it.each([RadioRoles.ap, RadioRoles.cpe])(
  'shows the saved radio reading for %s',
  (role) => {
    const device = {
      id: role,
      role,
      radio_summary: {
        observed_at: '2026-11-05T10:00:00Z',
        version: '6.49',
        model: 'synthetic',
        cpu: 7,
        uptime: '1d',
      },
    } as unknown as Device;
    expect(latestSavedReading(device)).toMatchObject({
      at: '2026-11-05T10:00:00Z',
      version: '6.49',
      cpu: 7,
    });
  },
);
