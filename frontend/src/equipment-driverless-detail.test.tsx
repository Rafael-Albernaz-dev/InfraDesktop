import { render, screen, within } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { EquipmentDetailPane, type Props } from './equipment-detail';
import type { Device } from './equipment-api';

function detail(block: string) {
  const device = {
    id: 'switch',
    name: 'SW Norte',
    role: 'switch',
    block,
    active: true,
    address: '192.0.2.1',
    port: 22,
    revision: 1,
    fingerprint: 'saved',
    credential_id: 'synthetic',
  } as Device;
  return {
    profile: 'synthetic',
    activation: 'active',
    selected: device,
    devices: [device],
    connected: true,
    vault: true,
    ready: true,
    busy: false,
    locked: false,
    displayNow: Date.now(),
    surfaceRef: { current: 0 },
    detailReadRef: { current: 0 },
    detailSurfaceRef: { current: null },
    setDetailOpen: vi.fn(),
    setJob: vi.fn(),
    startOperation: vi.fn(),
    openById: vi.fn(),
  } as unknown as Props;
}

it('shows the block and a driverless sheet without any read action', async () => {
  const p = detail('Torre Norte');
  const view = render(<EquipmentDetailPane {...p} />);
  expect(screen.getByText(/Switch · Torre Norte/)).toBeVisible();
  expect(screen.getByText(/Não feito.*L-REDE/)).toBeVisible();
  expect(within(screen.getByRole('tablist')).getAllByRole('tab')).toHaveLength(
    2,
  );
  expect(
    screen.queryByRole('button', { name: 'Atualizar' }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole('button', { name: 'Conectar' }),
  ).not.toBeInTheDocument();
  expect(p.startOperation).not.toHaveBeenCalled();
  view.rerender(<EquipmentDetailPane {...detail('')} />);
  expect(screen.queryByText(/Switch · Torre Norte/)).not.toBeInTheDocument();
  expect(screen.getByText(/Switch · Não informado/)).toBeVisible();
});
