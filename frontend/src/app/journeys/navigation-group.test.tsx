import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import App from '../App';
import {
  access,
  equipmentAPI,
  type Result,
} from '../../platform/wails/presentation';
import { RadioTransports } from '../../features/radio/radio-contract';
import type { Device } from '../../features/equipment/equipment-api';

vi.mock('../../platform/wails/presentation', async (importOriginal) => ({
  ...(await importOriginal<
    typeof import('../../platform/wails/presentation')
  >()),
  access: Object.fromEntries(
    'status,selectCompany,getBrand'.split(',').map((key) => [key, vi.fn()]),
  ),
  equipmentAPI: Object.fromEntries(
    'list,credentials,get'.split(',').map((key) => [key, vi.fn()]),
  ),
}));
const account: Result = {
  code: 'OK',
  request_id: 'synthetic',
  instance_id: 'one',
  activation_id: 'a',
  profile_id: 'company',
  heartbeat: 0,
  setup: false,
  authenticated: true,
  name: 'user',
  theme: 'light',
  recovery: '',
  companies: [{ id: 'company', name: 'Empresa exemplo' }],
};
const device = {
  id: 'device',
  name: 'Roteador exemplo',
  role: 'other',
  address: '192.0.2.1',
  port: 22,
  transport: RadioTransports.ssh,
  revision: 1,
  active: true,
} as Device;
const reply = {
  code: 'OK',
  profile_id: 'company',
  activation_id: 'a',
  data: {
    devices: [device],
    device,
    credentials: [],
    next_cursor: 0,
    total_devices: 1,
    vault_available: true,
  },
};
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(access.status).mockResolvedValue(account);
  vi.mocked(access.selectCompany).mockResolvedValue(account);
  vi.mocked(access.getBrand).mockResolvedValue(account);
  vi.mocked(equipmentAPI.list).mockResolvedValue(reply);
  vi.mocked(equipmentAPI.credentials).mockResolvedValue(reply);
  vi.mocked(equipmentAPI.get).mockResolvedValue(reply);
});
async function overview() {
  render(<App />);
  fireEvent.click(await screen.findByText('Empresa exemplo'));
  await screen.findByRole('heading', { name: 'Visão geral' });
}
function group() {
  return within(screen.getByRole('navigation', { name: 'Menu' })).getByRole(
    'link',
    { name: 'Equipamentos' },
  );
}
function toggleTwice() {
  fireEvent.click(group());
  fireEvent.click(group());
}
async function list() {
  await overview();
  fireEvent.click(group());
  await screen.findByRole('cell', { name: 'Roteador exemplo' });
}
it('outside the group clicking Equipamentos opens the list', async () => {
  await overview();
  fireEvent.click(group());
  expect(
    await screen.findByRole('cell', { name: 'Roteador exemplo' }),
  ).toBeVisible();
});
it('reopening the group preserves the credentials route and adds no history step', async () => {
  await list();
  fireEvent.click(screen.getByRole('link', { name: 'Credenciais' }));
  expect(
    await screen.findByRole('heading', { name: 'Credenciais' }),
  ).toBeVisible();
  toggleTwice();
  expect(screen.getByRole('heading', { name: 'Credenciais' })).toBeVisible();
  expect(group()).toHaveAttribute('aria-expanded', 'true');
  fireEvent.keyDown(screen.getByRole('main').ownerDocument, {
    key: 'Backspace',
  });
  expect(
    await screen.findByRole('heading', { name: 'Equipamentos' }),
  ).toBeVisible();
});
it('reopening the group preserves the same open equipment sheet and adds no history step', async () => {
  await list();
  fireEvent.click(screen.getByRole('cell', { name: 'Roteador exemplo' }));
  expect(
    await screen.findByRole('heading', { name: 'Roteador exemplo' }),
  ).toBeVisible();
  toggleTwice();
  expect(
    screen.getByRole('region', { name: 'Ficha do equipamento' }),
  ).toBeVisible();
  expect(
    screen.getByRole('heading', { name: 'Roteador exemplo' }),
  ).toBeVisible();
  fireEvent.keyDown(screen.getByRole('main').ownerDocument, {
    key: 'Backspace',
  });
  expect(
    screen.queryByRole('region', { name: 'Ficha do equipamento' }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole('cell', { name: 'Roteador exemplo' })).toBeVisible();
});
