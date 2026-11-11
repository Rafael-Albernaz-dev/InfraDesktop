import { render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { SessionCPE } from '../../features/pppoe/session-cpe';
import { equipmentAPI, radioAPI } from '../../platform/wails/presentation';
import type { Device } from '../../features/equipment/equipment-api';
vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({}),
);
it('does not show a different device when the local lookup returns the wrong id', async () => {
  vi.mocked(radioAPI.link).mockResolvedValue({
    code: 'OK',
    profile_id: 'fixture',
    activation_id: 'one',
    data: {
      link: { cpe_id: 'requested' } as never,
      total: 0,
      more: false,
      next_offset: 0,
    },
  });
  vi.mocked(equipmentAPI.get).mockResolvedValue({
    code: 'OK',
    profile_id: 'fixture',
    activation_id: 'one',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      vault_available: true,
      device: { id: 'different', name: 'Outro equipamento' } as Device,
    },
  });
  render(
    <SessionCPE
      profile="fixture"
      activation="one"
      deviceId="ccr"
      observation="obs"
      row="row"
    />,
  );
  expect(await screen.findByText('Não informado nesta leitura')).toBeVisible();
  expect(screen.queryByText('Outro equipamento')).not.toBeInTheDocument();
});
