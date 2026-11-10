import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { EquipmentSummaryStatistics } from './equipment-summary-statistics';
import {
  deviceMonitoringAPI,
  pppoeAPI,
} from '../../platform/wails/presentation';
import type { Device } from './equipment-api';
vi.mock('../../platform/wails/presentation', () => ({
  deviceMonitoringAPI: { current: vi.fn() },
  pppoeAPI: { current: vi.fn() },
}));
const device = { id: 'synthetic', revision: 1 } as Device;
it('reabre projeção local após erro e não converte ausência de reinícios em zero', async () => {
  vi.mocked(pppoeAPI.current)
    .mockRejectedValueOnce(new Error('offline'))
    .mockResolvedValue({
      code: 'OK',
      profile_id: 'p',
      activation_id: 'a',
      data: {
        total: 1,
        current: {
          observation_id: 'o',
          count: 3,
          online: 2,
          provenance: { ended_at: new Date().toISOString() },
          quality: { status: 'complete' },
        },
      },
    });
  vi.mocked(deviceMonitoringAPI.current).mockResolvedValue({
    code: 'OK',
    profile_id: 'p',
    activation_id: 'a',
    data: { total: 0 },
  });
  const view = render(
    <EquipmentSummaryStatistics profile="p" activation="a" device={device} />,
  );
  await screen.findByRole('alert');
  expect(screen.queryByText('Nenhum reinício encontrado')).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: 'Tentar novamente' }));
  await screen.findByText('2');
  expect(screen.queryByRole('alert')).toBeNull();
  expect(screen.getByText('Ainda sem leitura')).toBeVisible();
  view.rerender(
    <EquipmentSummaryStatistics
      profile="other"
      activation="b"
      device={device}
    />,
  );
  await waitFor(() => expect(screen.queryByText('2')).toBeNull());
  expect(await screen.findAllByRole('alert')).toHaveLength(2);
});
