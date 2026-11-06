import { operationalAccount } from './operational-account-fixture';
import { findDevicePrimary, openDeviceSheet } from './test-navigation';
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { it, expect, vi, beforeEach } from 'vitest';
import App from './App';
import { Equipment } from './test-equipment-shell';
import {
  access,
  equipmentAPI as api,
  pppoeAPI as sessions,
} from './presentation';
import type { Device, EquipmentResult } from './equipment-api';
vi.mock('./presentation', () => ({
  access: { status: vi.fn(), getBrand: vi.fn(), selectCompany: vi.fn() },
  pppoeAPI: {
    current: vi.fn(),
    references: vi.fn(),
    list: vi.fn(),
    start: vi.fn(),
  },
  equipmentAPI: {
    list: vi.fn(),
    credentials: vi.fn(),
    get: vi.fn(),
    inspect: vi.fn(),
    start: vi.fn(),
    status: vi.fn(),
    save: vi.fn(),
    confirm: vi.fn(),
    cancel: vi.fn(),
    resetTrust: vi.fn(),
  },
}));
const device = {
  id: 'router',
  role: 'concentrator',
  name: 'Saved router',
  address: '127.0.0.1',
  port: 22,
  transport: 'ssh',
  credential_id: 'reader',
  revision: 1,
  active: true,
  fingerprint: 'SHA256:test',
  trust_origin: '',
  result: {
    identity: 'Observed router',
    version: '7.26',
    family: 'v7',
    board: 'Board',
    uptime: '1h',
    cpu_percent: 12,
    free_memory_bytes: 1024,
    total_memory_bytes: 2048,
    interfaces: [
      { name: 'ether1', kind: 'ether', running: true, disabled: false },
    ],
    observed_at: '2026-10-24T19:00:00Z',
    source: 'SSH loopback',
    quality: 'complete',
  },
} as Device;
const result = (d = device): EquipmentResult => ({
  code: 'OK',
  profile_id: 'X',
  activation_id: 'a',
  data: {
    devices: [
      { ...d, result: d.result ? { ...d.result, interfaces: [] } : undefined },
    ],
    device: d,
    credentials: [],
    next_cursor: 0,
    total_devices: 1,
    vault_available: true,
  },
});
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(access.selectCompany).mockImplementation(async () =>
    access.status(),
  );
  vi.mocked(sessions.current).mockResolvedValue({
    code: 'OK',
    profile_id: 'X',
    activation_id: 'a',
    data: { total: 0 },
  });
  vi.mocked(sessions.references).mockResolvedValue({
    code: 'OK',
    profile_id: 'X',
    activation_id: 'a',
    data: { total: 0, references: [] },
  });
  vi.mocked(api.list).mockResolvedValue(result());
  vi.mocked(api.get).mockResolvedValue(result());
  vi.mocked(api.credentials).mockResolvedValue(result());
});

it('L06C-UX: equipment summary and interfaces are separate, with a visible PPPoE path', async () => {
  const concentrator = {
    ...device,
    role: 'concentrator',
    result: {
      ...device.result!,
      interfaces: [
        ...device.result!.interfaces,
        { name: 'VLAN100', kind: 'vlan', running: true, disabled: false },
        { name: 'WG-VPN', kind: 'wireguard', running: true, disabled: false },
      ],
    },
  };
  vi.mocked(api.get).mockResolvedValue(result(concentrator));
  render(
    <Equipment
      profile="X"
      activation="a"
      connected={true}

      targetID="router"
    />,
  );
  await screen.findByRole('heading', { name: 'Saved router' });
  expect(screen.queryByText('ether1')).not.toBeInTheDocument();
  expect(
    screen
      .getAllByRole('button')
      .filter((button) => button.classList.contains('primary')),
  ).toHaveLength(1);
  fireEvent.click(screen.getByRole('tab', { name: 'Interfaces e saúde' }));
  expect(await screen.findByText('ether1')).toBeVisible();
  expect(screen.queryByText('Observed router')).not.toBeInTheDocument();
  expect(screen.queryByText('VLAN100')).not.toBeInTheDocument();
  expect(screen.queryByText('WG-VPN')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'VLANs' }));
  expect(screen.getByText('VLAN100')).toBeVisible();
  expect(screen.queryByText('ether1')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'VPNs e túneis' }));
  expect(screen.getByText('WG-VPN')).toBeVisible();
  expect(screen.queryByText('VLAN100')).not.toBeInTheDocument();
  expect(screen.getByRole('tab', { name: 'Sessões' })).toBeVisible();
  fireEvent.click(screen.getByRole('tab', { name: 'Sessões' }));
  await screen.findByText('Sessões PPPoE');
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  expect(
    await screen.findByRole('row', { name: /Saved router/ }),
  ).toHaveFocus();
  expect(sessions.start).not.toHaveBeenCalled();
});
it('list reads the normal company inventory and reopens its full detail', async () => {
  vi.mocked(api.list).mockResolvedValue(
    result({
      ...device,
      role: 'concentrator',
      pppoe: { online: 53, offline: 2, at: '2026-10-26T12:00:00Z' },
    }),
  );
  vi.mocked(access.status).mockResolvedValue(operationalAccount());
  vi.mocked(access.getBrand).mockResolvedValue({
    code: 'OK',
    brand: { name: '', accent: 'teal', logo: '' },
  } as Awaited<ReturnType<typeof access.getBrand>>);
  render(<App />);
  fireEvent.click(await screen.findByText('Company X'));
  fireEvent.click(await screen.findByRole('link', { name: 'Equipamentos' }));
  await openDeviceSheet('Saved router');
  await openDeviceSheet('Saved router');
  fireEvent.click(
    await screen.findByRole('tab', { name: 'Interfaces e saúde' }),
  );
  expect(await screen.findByText('ether1')).toBeVisible();
  expect(api.get).toHaveBeenCalledWith('X', 'a', 'router');
});
it('successful query automatically opens full result without a second action', async () => {
  vi.mocked(api.start).mockResolvedValue({
    ...result(),
    data: {
      ...result().data!,
      job: {
        id: 'query',
        device_id: 'router',
        kind: 'device.query-start',
        state: 'SUCCEEDED',
        code: 'OK',
        result: device.result,
      },
    },
  });
  render(<Equipment profile="X" activation="a" connected />);
  await openDeviceSheet('Saved router');
  fireEvent.click(
    screen.getByRole('button', { name: /^(Atualizar|Consultar)$/ }),
  );
  await waitFor(() => expect(api.get).toHaveBeenCalled());
  fireEvent.click(
    await screen.findByRole('tab', { name: 'Interfaces e saúde' }),
  );
  expect(await screen.findByText('ether1')).toBeVisible();
  expect(
    screen.queryByRole('button', { name: 'Ver resultado' }),
  ).not.toBeInTheDocument();
});

it('save guides explicit acceptance and query, then cancel/failure preserve the saved device', async () => {
  let current: Device = { ...device, fingerprint: '', result: undefined };
  let saved = false;
  let sentSecret = '';
  vi.mocked(api.list).mockImplementation(async () => ({
    ...result(current),
    data: {
      ...result(current).data!,
      devices: saved ? [current] : [],
      total_devices: saved ? 1 : 0,
    },
  }));
  vi.mocked(api.get).mockImplementation(async () => result(current));
  vi.mocked(api.save).mockImplementation(async (_p, _a, input) => {
    sentSecret = input.secret;
    saved = true;
    return result(current);
  });
  vi.mocked(api.inspect).mockResolvedValue({
    ...result(current),
    data: {
      ...result(current).data!,
      job: {
        id: 'inspect',
        device_id: 'router',
        kind: 'trust.inspect',
        state: 'SUCCEEDED',
        code: 'OK',
        fingerprint: 'SHA256:observed',
      },
    },
  });
  vi.mocked(api.confirm).mockImplementation(async () => {
    current = { ...current, fingerprint: 'SHA256:observed', revision: 2 };
    return result(current);
  });
  vi.mocked(api.start).mockImplementation(async () => {
    current = { ...current, result: device.result };
    return {
      ...result(current),
      data: {
        ...result(current).data!,
        job: {
          id: 'read',
          device_id: 'router',
          kind: 'device.query-start',
          state: 'SUCCEEDED',
          code: 'OK',
        },
      },
    };
  });
  const view = render(
    <Equipment profile="enroll" activation="a" connected addPage />,
  );
  await screen.findByRole('heading', { name: 'Adicionar equipamento' });
  fireEvent.change(screen.getByLabelText('Papel do equipamento'), {
    target: { value: 'other' },
  });
  fireEvent.change(screen.getByLabelText('Credencial de consulta'), {
    target: { value: '__inline__' },
  });
  fireEvent.change(screen.getByLabelText('Nome'), {
    target: { value: 'Saved router' },
  });
  fireEvent.change(screen.getByLabelText('IP de gerenciamento'), {
    target: { value: '127.0.0.1' },
  });
  fireEvent.change(screen.getByLabelText('Usuário'), {
    target: { value: 'reader' },
  });
  fireEvent.change(screen.getByLabelText('Senha', { exact: true }), {
    target: { value: '  exact synthetic secret  ' },
  });
  fireEvent.click(screen.getByRole('button', { name: /^Salvar$/ }));
  await waitFor(() => expect(api.save).toHaveBeenCalledOnce());
  expect(api.inspect).not.toHaveBeenCalled();
  expect(api.start).not.toHaveBeenCalled();
  await openDeviceSheet('Saved router');
  fireEvent.click(
    await screen.findByRole('button', { name: 'Verificar identidade' }),
  );
  const trust = await screen.findByRole('dialog', { name: 'Primeira conexão' });
  expect(api.save).toHaveBeenCalledTimes(1);
  expect(api.start).not.toHaveBeenCalled();
  expect(
    screen.queryByLabelText('Origem da conferência'),
  ).not.toBeInTheDocument();
  expect(trust.querySelectorAll('button.primary')).toHaveLength(1);
  fireEvent.click(
    within(trust).getByRole('button', {
      name: 'Confiar neste equipamento e conectar',
    }),
  );
  fireEvent.click(
    await screen.findByRole('tab', { name: 'Interfaces e saúde' }),
  );
  expect(await screen.findByText('ether1')).toBeVisible();
  expect(api.start).toHaveBeenCalledTimes(1);
  expect(api.save).toHaveBeenCalledTimes(1);
  expect(sentSecret).toBe('  exact synthetic secret  ');
  expect(
    screen.queryByDisplayValue('  exact synthetic secret  '),
  ).not.toBeInTheDocument();
  vi.mocked(api.start).mockResolvedValue({
    ...result(current),
    code: 'CONNECT_FAILED',
  });
  fireEvent.click(await findDevicePrimary(view.container, device.name));
  expect(await screen.findByRole('alert')).toHaveTextContent('porta SSH');
  expect(screen.getByText('ether1')).toBeVisible();
  expect(api.save).toHaveBeenCalledTimes(1);
});
it('cancel first acceptance never authenticates and leaves a resumable saved device', async () => {
  const fresh = { ...device, fingerprint: '', result: undefined };
  vi.mocked(api.list).mockResolvedValue(result(fresh));
  vi.mocked(api.get).mockResolvedValue(result(fresh));
  vi.mocked(api.inspect).mockResolvedValue({
    ...result(fresh),
    data: {
      ...result(fresh).data!,
      job: {
        id: 'candidate-cancel',
        device_id: 'router',
        kind: 'trust.inspect',
        state: 'SUCCEEDED',
        code: 'OK',
        fingerprint: 'SHA256:candidate',
      },
    },
  });
  vi.mocked(api.cancel).mockResolvedValue({
    ...result(fresh),
    data: {
      ...result(fresh).data!,
      job: {
        id: 'candidate-cancel',
        device_id: 'router',
        kind: 'trust.inspect',
        state: 'CANCELLED',
        code: 'CANCELLED',
      },
    },
  });
  render(<Equipment profile="cancel-first" activation="a" connected />);
  await openDeviceSheet('Saved router');
  fireEvent.click(
    screen.getByRole('button', { name: /^(Conectar|Atualizar)$/ }),
  );
  fireEvent.click(
    within(
      await screen.findByRole('dialog', { name: 'Primeira conexão' }),
    ).getByRole('button', { name: 'Cancelar' }),
  );
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
  );
  expect(api.cancel).toHaveBeenCalledWith(
    'cancel-first',
    'a',
    'candidate-cancel',
  );
  expect(api.start).not.toHaveBeenCalled();
  expect(api.confirm).not.toHaveBeenCalled();
  fireEvent.click(
    screen.getByRole('button', { name: /^(Conectar|Atualizar)$/ }),
  );
  await screen.findByRole('dialog', { name: 'Primeira conexão' });
  expect(api.save).not.toHaveBeenCalled();
});
it('overview and list navigation retain saved data while Back exposes the list', async () => {
  vi.mocked(access.status).mockResolvedValue(operationalAccount());
  vi.mocked(access.getBrand).mockResolvedValue({
    code: 'OK',
    brand: { name: '', accent: 'teal', logo: '' },
  } as Awaited<ReturnType<typeof access.getBrand>>);
  render(<App />);
  fireEvent.click(await screen.findByText('Company X'));
  fireEvent.click(await screen.findByRole('link', { name: 'Equipamentos' }));
  await openDeviceSheet('Saved router');
  await openDeviceSheet('Saved router');
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  expect(await screen.findByLabelText('Buscar')).toBeVisible();
  expect(
    screen.queryByRole('button', { name: 'Voltar' }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('link', { name: 'Visão geral' }));
  fireEvent.click(screen.getByRole('link', { name: 'Equipamentos' }));
  await openDeviceSheet('Saved router');
  fireEvent.click(
    await screen.findByRole('tab', { name: 'Interfaces e saúde' }),
  );
  expect(await screen.findByText('ether1')).toBeVisible();
});

it('inventory read error offers retry without claiming an empty inventory', async () => {
  vi.mocked(access.status).mockResolvedValue(operationalAccount());
  vi.mocked(access.getBrand).mockResolvedValue({
    code: 'OK',
    brand: { name: '', accent: 'teal', logo: '' },
  } as Awaited<ReturnType<typeof access.getBrand>>);
  let fail = true;
  vi.mocked(api.list).mockImplementation(async () => {
    if (fail) throw new Error('transport');
    return result();
  });
  render(<App />);
  fireEvent.click(await screen.findByText('Company X'));
  fireEvent.click(await screen.findByRole('link', { name: 'Equipamentos' }));
  await screen.findByRole('alert');
  expect(
    screen.queryByRole('heading', { name: 'Nenhum equipamento cadastrado' }),
  ).not.toBeInTheDocument();
  fail = false;
  fireEvent.click(screen.getByRole('button', { name: 'Tentar novamente' }));
  expect(
    await screen.findByRole('cell', { name: 'Saved router' }),
  ).toBeVisible();
});
