import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { RadioEquipment } from '../../features/radio/radio';
import { RadioRoles } from '../../features/equipment/radio-contract';
import { radioAPI as api } from '../../features/radio/radio-api';
import {
  device,
  reading,
  response,
} from '../../features/radio/radio-test-fixtures';
import { DeviceMonitoring } from '../../features/monitoring/device-monitoring';
vi.mock('../../features/radio/radio-api', () => ({
  radioAPI: { current: vi.fn(), cancelOperation: vi.fn() },
}));
it('PTP apresenta só o próprio lado A e declara lado B como L07-DRV', async () => {
  vi.mocked(api.current).mockResolvedValue(
    response({
      reading: {
        ...reading,
        metrics: [
          {
            name: 'signal',
            value: -52,
            unit: 'dBm',
            direction: 'local',
            perspective: 'CPE',
            device_id: device.id,
            interface: 'wlan1',
            observed_at: reading.observed_at,
            quality: 'observed',
          },
          {
            name: 'snr',
            value: 45,
            unit: 'dB',
            direction: 'local',
            perspective: 'CPE',
            device_id: device.id,
            interface: 'wlan1',
            observed_at: reading.observed_at,
            quality: 'observed',
          },
        ],
      },
    }),
  );
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={{ ...device, role: RadioRoles.ptp }}
      blocked={false}
      tab="Enlace"
    />,
  );
  const a = (await screen.findByText('Lado A')).closest('[data-state]')!;
  const b = screen.getByText('Lado B').closest('[data-state]')!;
  expect(a).toHaveAttribute('data-state', 'ready');
  expect(within(a as HTMLElement).getByText('-52 dBm')).toBeVisible();
  expect(within(a as HTMLElement).getByText('45 dB')).toBeVisible();
  expect(b).toHaveAttribute('data-state', 'unbuilt');
  expect(b).toHaveTextContent('Ainda não existe (L07-DRV)');
  expect(screen.queryByText('AP de origem')).not.toBeInTheDocument();
});
afterEach(() => {
  cleanup();
  vi.stubGlobal('go', undefined);
});
it('higiene sem observação é Sem leitura e uma leitura sem linhas é Vazio', async () => {
  let observed = false;
  const current = vi.fn(async (...args: [string, string, string, string]) => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      total: 0,
      read: {
        scope: args[3],
        ended_at: '2026-11-04T12:00:00Z',
        duration_ns: 0,
        rows:
          args[3] === 'hygiene' && !observed
            ? [
                {
                  kind: 'availability',
                  name: 'higiene',
                  value: 'unavailable',
                  uptime: { known: false, min: 0, max: 0 },
                },
              ]
            : [],
      },
    },
  }));
  vi.stubGlobal('go', {
    presentation: { App: { GetDeviceMonitoring: current } },
  });
  render(
    <DeviceMonitoring profile="company" activation="active" device={device} />,
  );
  const card = screen.getByText('Higiene de secrets').closest('[data-state]')!;
  await waitFor(() => expect(card).toHaveAttribute('data-state', 'unread'));
  const update = within(card as HTMLElement).getByRole('button', {
    name: 'Atualizar',
  });
  expect(update).toBeEnabled();
  observed = true;
  fireEvent.click(update);
  await waitFor(() => expect(card).toHaveAttribute('data-state', 'empty'));
  expect(
    within(card as HTMLElement).getByText('Nenhum secret encontrado'),
  ).toBeVisible();
  expect(current.mock.calls.some((args) => args[3] === 'hygiene')).toBe(true);
});
