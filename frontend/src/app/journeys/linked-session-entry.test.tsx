import { render, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { PPPoE } from '../pages/pppoe-with-link';
import { SessionRoute } from '../../features/equipment/session-navigation';
import { pppoeAPI } from '../../platform/wails/presentation';
import type { Device } from '../../features/equipment/equipment-api';

vi.mock('../../platform/wails/presentation', async (importOriginal) =>
  (await import('../../shared/testing/presentation-mock')).presentationMock(
    {},
    Object.fromEntries(
      Object.entries(
        await importOriginal<Record<string, Record<string, unknown>>>(),
      ).filter(([name]) => !['pppoeAPI', 'deviceMonitoringAPI'].includes(name)),
    ),
  ),
);
const response = (data: object = {}) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: { total: 0, ...data },
});
it('opens the exact linked session target as its own navigation step', async () => {
  const open = vi.fn(),
    consumed = vi.fn();
  const device = {
    id: 'router-fixture',
    name: 'Sintético',
    revision: 1,
    active: true,
  } as Device;
  const row = {
    row_id: 'row-fixture',
    login: 'usuario sintético',
    remote_id: '*1',
    address: '192.0.2.1',
    uptime: { known: true, min: 1, max: 1 },
  };
  vi.mocked(pppoeAPI.current).mockResolvedValue(
    response({
      current: {
        observation_id: 'obs-fixture',
        count: 1,
        quality: { status: 'complete' },
        provenance: { ended_at: '2026-11-06T00:00:00Z' },
      },
    }) as never,
  );
  vi.mocked(pppoeAPI.references).mockResolvedValue(
    response({ references: [] }) as never,
  );
  vi.mocked(pppoeAPI.list).mockResolvedValue(
    response({ rows: [row], total: 1 }) as never,
  );
  vi.mocked(pppoeAPI.groups).mockResolvedValue(
    response({ groups: [] }) as never,
  );
  vi.mocked(pppoeAPI.detail).mockResolvedValue(
    response({ rows: [row], total: 1 }) as never,
  );
  render(
    <SessionRoute.Provider
      value={{ active: false, open, back: () => {}, leave: () => {} }}
    >
      <PPPoE
        profile="company"
        activation="active"
        device={device}
        back={() => {}}
        sessionTarget={{
          deviceID: 'router-fixture',
          observation: 'obs-fixture',
          row: 'row-fixture',
        }}
        onSessionConsumed={consumed}
      />
    </SessionRoute.Provider>,
  );
  await waitFor(() =>
    expect(open).toHaveBeenCalledWith({
      device: 'router-fixture',
      observation: 'obs-fixture',
      row: 'row-fixture',
      login: 'usuario sintético',
    }),
  );
  expect(pppoeAPI.detail).toHaveBeenCalledWith(
    'company',
    'active',
    'router-fixture',
    'obs-fixture',
    'row-fixture',
  );
  expect(consumed).toHaveBeenCalledOnce();
});
