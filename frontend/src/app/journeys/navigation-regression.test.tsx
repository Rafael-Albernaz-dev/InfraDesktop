import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import App from '../App';
import {
  access,
  equipmentAPI,
  type Result,
} from '../../platform/wails/presentation';
import type { Device } from '../../features/equipment/equipment-api';
import { createNavigationHistory } from '../navigation-history';
import { useRef } from 'react';
import { Equipment } from '../pages/equipment';
import { Button } from '../../shared/ui/Button';
import {
  reading,
  link,
  device as radioDevice,
} from '../../shared/testing/radio-test-fixtures';
import {
  RadioRoles,
  RadioTransports,
} from '../../features/equipment/radio-contract';

vi.mock('../../platform/wails/presentation', async (importOriginal) =>
  (await import('../../shared/testing/presentation-mock')).presentationMock(
    {},
    Object.fromEntries(
      Object.entries(
        await importOriginal<Record<string, Record<string, unknown>>>(),
      ).filter(([name]) => !['access', 'equipmentAPI'].includes(name)),
    ),
  ),
);
const account: Result = {
  code: 'OK',
  request_id: 'test',
  instance_id: 'one',
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
const device = {
  id: 'device',
  role: 'other',
  name: 'Roteador exemplo',
  address: '192.0.2.1',
  port: 22,
  transport: RadioTransports.ssh,
  revision: 1,
  active: true,
} as Device;
const data = {
  code: 'OK',
  profile_id: 'company',
  activation_id: 'activation',
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
  vi.mocked(equipmentAPI.list).mockResolvedValue(data);
  vi.mocked(equipmentAPI.get).mockResolvedValue(data);
  vi.mocked(equipmentAPI.credentials).mockResolvedValue(data);
});
async function inventory() {
  render(<App />);
  fireEvent.click(await screen.findByText('Empresa exemplo'));
  fireEvent.click(await screen.findByRole('link', { name: 'Equipamentos' }));
  return screen.findByRole('cell', { name: 'Roteador exemplo' });
}
it('reopens the equipment group after collapsing it repeatedly', async () => {
  await inventory();
  const group = screen.getByRole('link', { name: 'Equipamentos' });
  expect(group).toHaveAttribute('aria-expanded', 'true');
  fireEvent.click(group);
  expect(group).toHaveAttribute('aria-expanded', 'false');
  fireEvent.click(group);
  expect(group).toHaveAttribute('aria-expanded', 'true');
  fireEvent.click(group);
  fireEvent.click(group);
  expect(group).toHaveAttribute('aria-expanded', 'true');
});
it('returns from a detail to the list when Lista is clicked repeatedly', async () => {
  fireEvent.click(await inventory());
  expect(
    await screen.findByRole('region', { name: 'Ficha do equipamento' }),
  ).toBeVisible();
  fireEvent.click(
    within(screen.getByRole('navigation', { name: 'Menu' })).getByRole('link', {
      name: 'Lista',
    }),
  );
  expect(
    screen.queryByRole('region', { name: 'Ficha do equipamento' }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole('cell', { name: 'Roteador exemplo' })).toBeVisible();
  fireEvent.click(screen.getByRole('link', { name: 'Lista' }));
  expect(screen.getByRole('cell', { name: 'Roteador exemplo' })).toBeVisible();
});

function EmptySelectionHarness() {
  const back = useRef<(() => void) | undefined>(undefined);
  return (
    <>
      <Button onClick={() => back.current?.()}>Lista</Button>
      <Equipment
        profile="company"
        activation="activation"
        connected

        onBackReady={(next) => {
          back.current = next;
        }}
      />
    </>
  );
}
it('returns to the list even without an external selection', async () => {
  render(<EmptySelectionHarness />);
  fireEvent.click(
    await screen.findByRole('cell', { name: 'Roteador exemplo' }),
  );
  expect(
    await screen.findByRole('region', { name: 'Ficha do equipamento' }),
  ).toBeVisible();
  fireEvent.click(screen.getByRole('button', { name: 'Lista' }));
  expect(
    screen.queryByRole('region', { name: 'Ficha do equipamento' }),
  ).not.toBeInTheDocument();
});

it.each(['mouse', 'alt-left', 'backspace'])(
  'returns from a sheet with %s',
  async (trigger) => {
    fireEvent.click(await inventory());
    expect(
      await screen.findByRole('region', { name: 'Ficha do equipamento' }),
    ).toBeVisible();
    if (trigger === 'mouse')
      fireEvent.mouseDown(screen.getByRole('main').ownerDocument, {
        button: 3,
      });
    if (trigger === 'alt-left')
      fireEvent.keyDown(screen.getByRole('main').ownerDocument, {
        key: 'ArrowLeft',
        altKey: true,
      });
    if (trigger === 'backspace')
      fireEvent.keyDown(screen.getByRole('main').ownerDocument, {
        key: 'Backspace',
      });
    expect(
      screen.queryByRole('region', { name: 'Ficha do equipamento' }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole('cell', { name: 'Roteador exemplo' }),
    ).toBeVisible();
  },
);

it('guards each back trigger when the add screen has changes', async () => {
  await inventory();
  fireEvent.keyDown(screen.getByRole('textbox', { name: 'Buscar' }), {
    key: 'Backspace',
  });
  expect(screen.getByRole('heading', { name: 'Equipamentos' })).toBeVisible();
  fireEvent.click(
    screen.getByRole('button', { name: 'Adicionar equipamento' }),
  );
  fireEvent.change(screen.getByLabelText('Nome'), {
    target: { value: 'AP novo' },
  });
  const doc = screen.getByRole('main').ownerDocument;
  for (const trigger of [
    () => fireEvent.keyDown(doc, { key: 'ArrowLeft', altKey: true }),
    () => fireEvent.keyDown(doc, { key: 'Backspace' }),
    () => fireEvent.mouseDown(doc, { button: 3 }),
  ]) {
    trigger();
    const dialog = screen.getByRole('dialog', {
      name: 'Descartar alterações?',
    });
    expect(dialog).toBeVisible();
    fireEvent.keyDown(doc, { key: 'ArrowLeft', altKey: true });
    fireEvent.keyDown(doc, { key: 'Backspace' });
    fireEvent.mouseDown(doc, { button: 3 });
    expect(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).toBeVisible();
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Continuar editando' }),
    );
  }
  expect(
    screen.getByRole('heading', { name: 'Adicionar equipamento' }),
  ).toBeVisible();
});

it('blocks back navigation while an equipment operation is pending', async () => {
  const bound = { ...device, credential_id: 'cred' };
  const available = {
    ...data,
    data: { ...data.data, device: bound, devices: [bound] },
  };
  vi.mocked(equipmentAPI.list).mockResolvedValue(available);
  vi.mocked(equipmentAPI.get).mockResolvedValue(available);
  let finish!: (result: typeof available) => void;
  vi.mocked(equipmentAPI.inspect).mockReturnValue(
    new Promise((resolve) => {
      finish = resolve;
    }),
  );
  fireEvent.click(await inventory());
  fireEvent.click(await screen.findByRole('button', { name: 'Conectar' }));
  await waitFor(() => expect(equipmentAPI.inspect).toHaveBeenCalledOnce());
  fireEvent.keyDown(screen.getByRole('main').ownerDocument, {
    key: 'ArrowLeft',
    altKey: true,
  });
  expect(
    screen.getByRole('region', { name: 'Ficha do equipamento' }),
  ).toBeVisible();
  await act(async () => finish({ ...available, code: 'SERVICE_UNAVAILABLE' }));
});

it('asks before discarding a dirty form for a navigation click', async () => {
  await inventory();
  fireEvent.click(
    screen.getByRole('button', { name: 'Adicionar equipamento' }),
  );
  fireEvent.change(screen.getByRole('textbox', { name: 'Nome' }), {
    target: { value: 'rascunho' },
  });
  fireEvent.click(screen.getByRole('link', { name: 'Visão geral' }));
  expect(
    screen.getByRole('dialog', { name: 'Descartar alterações?' }),
  ).toBeVisible();
  fireEvent.click(screen.getByRole('button', { name: 'Continuar editando' }));
  expect(screen.getByRole('textbox', { name: 'Nome' })).toHaveValue('rascunho');
  fireEvent.click(screen.getByRole('link', { name: 'Visão geral' }));
  fireEvent.click(screen.getByRole('button', { name: 'Descartar alterações' }));
  expect(
    await screen.findByRole('heading', { name: 'Visão geral' }),
  ).toBeVisible();
});

it('limits and resets the navigation stack', () => {
  const stack = createNavigationHistory(2);
  stack.record({ route: 'overview', device: '' });
  stack.record({ route: 'equipment', device: '' });
  stack.record({ route: 'equipment', device: 'device' });
  expect(stack.back()).toEqual({ route: 'equipment', device: '' });
  expect(stack.back()).toBeUndefined();
  stack.reset();
  expect(stack.back()).toBeUndefined();
});

it('clears navigation history when selecting a different company', async () => {
  const twoCompanies = {
    ...account,
    companies: [...account.companies, { id: 'other', name: 'Empresa dois' }],
  };
  const other = {
    ...twoCompanies,
    profile_id: 'other',
    activation_id: 'new-activation',
  };
  vi.mocked(access.status).mockResolvedValue(twoCompanies);
  vi.mocked(access.selectCompany).mockImplementation(async (id) =>
    id === 'other' ? other : twoCompanies,
  );
  await inventory();
  fireEvent.click(screen.getByRole('link', { name: /Empresa exemplo/ }));
  fireEvent.click(screen.getByText('Empresa dois'));
  expect(
    await screen.findByRole('heading', { name: 'Visão geral' }),
  ).toBeVisible();
  fireEvent.keyDown(screen.getByRole('main').ownerDocument, {
    key: 'Backspace',
  });
  expect(screen.getByRole('heading', { name: 'Visão geral' })).toBeVisible();
  expect(
    screen.queryByRole('heading', { name: 'Equipamentos' }),
  ).not.toBeInTheDocument();
});

it('clears navigation history across logout and login', async () => {
  const loggedOut = {
    ...account,
    authenticated: false,
    profile_id: '',
    name: '',
  };
  vi.mocked(access.logout).mockImplementation(async () => {
    vi.mocked(access.status).mockResolvedValue(loggedOut);
    return loggedOut;
  });
  vi.mocked(access.login).mockResolvedValue(account);
  await inventory();
  fireEvent.click(screen.getByRole('button', { name: /Conta:/ }));
  fireEvent.click(screen.getByRole('menuitem', { name: 'Sair da conta' }));
  fireEvent.click(screen.getByRole('button', { name: 'Sair' }));
  await screen.findByRole('heading', { name: 'Entrar' });
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
  fireEvent.keyDown(screen.getByRole('main').ownerDocument, {
    key: 'Backspace',
  });
  expect(screen.getByRole('heading', { name: 'Empresas' })).toBeVisible();
});

it('returns to Lista after a CPE opens its linked session on the concentrator', async () => {
  const cpe = {
    ...radioDevice,
    id: 'synthetic-cpe',
    name: 'CPE exemplo',
    role: RadioRoles.cpe,
    credential_id: 'cred',
  };
  const concentrator = {
    ...radioDevice,
    id: 'concentrator',
    name: 'Concentrador exemplo',
    role: 'concentrator',
    credential_id: 'cred',
  };
  const devices = [cpe, concentrator];
  const now = new Date().toISOString();
  const row = {
    row_id: 'linked-row',
    login: 'X',
    address: '192.0.2.100',
    state: 'ONLINE',
    uptime: { known: false, min: 0, max: 0 },
  };
  const reply = (data: Record<string, unknown> = {}) => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'activation',
    data: { total: 1, ...data },
  });
  vi.mocked(equipmentAPI.list).mockResolvedValue({
    ...data,
    data: { ...data.data, devices, total_devices: 2 },
  });
  vi.mocked(equipmentAPI.get).mockImplementation(async (_p, _a, id) => ({
    ...data,
    data: { ...data.data, device: devices.find((d) => d.id === id) },
  }));
  const bindings = {
    GetRadio: vi.fn(async () =>
      reply({ reading: { ...reading, observed_at: now }, link }),
    ),
    GetPPPoECurrent: vi.fn(async () =>
      reply({
        current: {
          observation_id: 'latest',
          count: 1,
          provenance: { ended_at: now },
          quality: { status: 'complete' },
        },
      }),
    ),
    ListPPPoESessions: vi.fn(async () => reply({ rows: [row] })),
    ListPPPoEReferences: vi.fn(async () => reply({ references: [] })),
    GetPPPoESession: vi.fn(async () => reply({ rows: [row] })),
    PPPoEContextCounts: vi.fn(async () => reply({ groups: [] })),
    GetDeviceMonitoring: vi.fn(async () => reply()),
    GetRadioLink: vi.fn(async () => reply()),
    StartPPPoERead: vi.fn(() => new Promise(() => {})),
    CancelPPPoEReadOperation: vi.fn(async () => reply()),
  };
  vi.stubGlobal('go', { presentation: { App: bindings } });
  try {
    render(<App />);
    fireEvent.click(await screen.findByText('Empresa exemplo'));
    fireEvent.click(await screen.findByRole('link', { name: 'Equipamentos' }));
    fireEvent.click(await screen.findByRole('cell', { name: 'CPE exemplo' }));
    fireEvent.click(await screen.findByRole('tab', { name: 'Cliente' }));
    fireEvent.click(await screen.findByRole('link', { name: 'X' }));
    expect(
      await screen.findByRole('region', { name: 'Detalhes da sessão' }),
    ).toHaveTextContent('X');
    expect(location.hash).toBe('#sessao/X');
    await waitFor(() => expect(bindings.StartPPPoERead).toHaveBeenCalledOnce());
    fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
    expect(
      await screen.findByRole('heading', { name: 'CPE exemplo' }),
    ).toBeVisible();
    expect(bindings.CancelPPPoEReadOperation).toHaveBeenCalledWith(
      'company',
      'activation',
      'concentrator',
      expect.any(String),
    );
    expect(screen.getByRole('tab', { name: 'Cliente' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    fireEvent.click(screen.getByRole('link', { name: 'Lista' }));
    expect(
      screen.queryByRole('region', { name: 'Ficha do equipamento' }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('cell', { name: 'CPE exemplo' })).toBeVisible();
  } finally {
    vi.unstubAllGlobals();
  }
});

it('expands the equipment group when the sheet breadcrumb opens the list', async () => {
  const cell = await inventory();
  fireEvent.click(cell);
  await screen.findByRole('region', { name: 'Ficha do equipamento' });
  const group = within(
    screen.getByRole('navigation', { name: 'Menu' }),
  ).getByRole('link', { name: 'Equipamentos' });
  fireEvent.click(group);
  expect(group).toHaveAttribute('aria-expanded', 'false');
  fireEvent.click(
    within(screen.getByRole('navigation', { name: 'Caminho' })).getByRole(
      'link',
      { name: 'Equipamentos' },
    ),
  );
  expect(group).toHaveAttribute('aria-expanded', 'true');
});
