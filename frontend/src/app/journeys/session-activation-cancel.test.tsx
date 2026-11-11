import type { PPRow } from '../../platform/wails/presentation';
import { createRef } from 'react';
import { render, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { PPPoESessionDetail } from '../../features/pppoe/pppoe-session-screen';
import { pppoeReadAPI, radioAPI } from '../../platform/wails/presentation';
import type { PPObservation } from '../../platform/wails/pppoe-types';
import type { Device } from '../../features/equipment/equipment-api';
vi.mock('../../platform/wails/presentation', async (importOriginal) =>
  (await import('../../shared/testing/presentation-mock')).presentationMock(
    {},
    Object.fromEntries(
      Object.entries(
        await importOriginal<Record<string, Record<string, unknown>>>(),
      ).filter(
        ([name]) =>
          ![
            'pppoeReadAPI',
            'pppoeDiagnosticAPI',
            'deviceMonitoringAPI',
            'pppoeAPI',
            'radioAPI',
          ].includes(name),
      ),
    ),
  ),
);
const device = {
  id: 'router-fixture',
  name: 'Sintético',
  revision: 1,
  active: true,
} as Device;
it('cancels the previous traffic job on activation change within the same company', async () => {
  vi.mocked(pppoeReadAPI.start).mockImplementation(
    async (profile, activation) =>
      ({
        code: 'OK',
        profile_id: profile,
        activation_id: activation,
        data: {
          total: 0,
          job: { id: `job-${activation}`, state: 'RUNNING', code: 'OK' },
        },
      }) as never,
  );
  vi.mocked(pppoeReadAPI.status).mockImplementation(
    async (profile, activation, id) =>
      ({
        code: 'OK',
        profile_id: profile,
        activation_id: activation,
        data: { total: 0, job: { id, state: 'RUNNING', code: 'OK' } },
      }) as never,
  );
  vi.mocked(pppoeReadAPI.cancel).mockImplementation(
    async (profile, activation) =>
      ({
        code: 'OK',
        profile_id: profile,
        activation_id: activation,
        data: { total: 0 },
      }) as never,
  );
  vi.mocked(radioAPI.link).mockImplementation(
    async (profile, activation) =>
      ({
        code: 'OK',
        profile_id: profile,
        activation_id: activation,
        data: { total: 0 },
      }) as never,
  );
  const selected = {
    row_id: 'row',
    login: 'sintético',
    state: 'ONLINE',
    address: '192.0.2.1',
    uptime: { known: true, min: 1, max: 1 },
  } as PPRow;
  const current = {
    observation_id: 'obs',
    provenance: { ended_at: '2026-11-06T00:00:00Z' },
  } as PPObservation;
  const screenFor = (activation: string) => (
    <PPPoESessionDetail
      selected={selected}
      current={current}
      detailHeadingRef={createRef()}
      detailTab="traffic"
      setDetailTab={() => {}}
      profile="company"
      activation={activation}
      device={device}
      disabled={false}
      detailIntentRef={{ current: 0 }}
      detailTriggerRef={{ current: null }}
      select={() => {}}
      back={() => {}}
    />
  );
  const view = render(screenFor('active-one'));
  await waitFor(() => expect(pppoeReadAPI.status).toHaveBeenCalled());
  view.rerender(screenFor('active-two'));
  await waitFor(() =>
    expect(pppoeReadAPI.cancel).toHaveBeenCalledWith(
      'company',
      'active-one',
      'job-active-one',
    ),
  );
  await waitFor(() => expect(pppoeReadAPI.start).toHaveBeenCalledTimes(2));
  expect(vi.mocked(pppoeReadAPI.start).mock.calls[1].slice(0, 2)).toEqual([
    'company',
    'active-two',
  ]);
  view.unmount();
});
