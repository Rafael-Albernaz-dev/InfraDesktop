import { act, fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import App from '../../app/App';
import {
  access,
  equipmentAPI,
  type Result,
} from '../../platform/wails/presentation';

vi.mock('../../platform/wails/presentation', () => ({
  access: Object.fromEntries(
    'status,login,selectCompany,getBrand'.split(',').map((k) => [k, vi.fn()]),
  ),
  equipmentAPI: { list: vi.fn() },
}));
const remembered: Result = {
  code: 'OK',
  request_id: 'test',
  instance_id: 'first',
  activation_id: 'activation',
  profile_id: 'company',
  heartbeat: 0,
  setup: false,
  authenticated: true,
  name: 'user',
  theme: 'light',
  recovery: '',
  companies: [{ id: 'company', name: 'Empresa exemplo' }],
};

it('opens the company list for remembered session and keeps the page after reconnecting', async () => {
  vi.mocked(access.status).mockResolvedValue(remembered);
  vi.mocked(access.selectCompany).mockResolvedValue(remembered);
  vi.mocked(access.getBrand).mockResolvedValue(remembered);
  vi.mocked(equipmentAPI.list).mockResolvedValue({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'activation',
    data: {
      devices: [],
      credentials: [],
      next_cursor: 0,
      total_devices: 0,
      vault_available: true,
    },
  });
  render(<App />);
  expect(
    await screen.findByRole('heading', { name: 'Empresas' }),
  ).toBeVisible();
  expect(screen.getByText('· Atual')).toBeVisible();
  fireEvent.click(screen.getByText('Empresa exemplo'));
  expect(
    await screen.findByRole('heading', { name: 'Visão geral' }),
  ).toBeVisible();
  fireEvent.click(screen.getByRole('button', { name: 'Configurações' }));
  expect(
    await screen.findByRole('heading', { name: 'Configurações' }),
  ).toBeVisible();
  vi.mocked(access.status).mockResolvedValue({
    ...remembered,
    instance_id: 'second',
  });
  const callsBeforeReconnect = vi.mocked(access.status).mock.calls.length;
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 1300));
  });
  expect(vi.mocked(access.status).mock.calls.length).toBeGreaterThan(
    callsBeforeReconnect,
  );
  expect(screen.getByRole('heading', { name: 'Configurações' })).toBeVisible();
});

it('opens companies after explicit login with a remembered company', async () => {
  vi.mocked(access.status).mockResolvedValue({
    ...remembered,
    authenticated: false,
    profile_id: '',
    name: '',
  });
  vi.mocked(access.login).mockResolvedValue(remembered);
  vi.mocked(access.getBrand).mockResolvedValue(remembered);
  render(<App />);
  await screen.findByText('Serviço ativo');
  fireEvent.change(screen.getByLabelText('Usuário'), {
    target: { value: 'user' },
  });
  fireEvent.change(screen.getByLabelText('Senha', { exact: true }), {
    target: { value: 'password' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Entrar' }));
  expect(
    await screen.findByRole('heading', { name: 'Empresas' }),
  ).toBeVisible();
  expect(screen.getByText('· Atual')).toBeVisible();
});
