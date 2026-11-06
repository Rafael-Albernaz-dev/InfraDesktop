import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { Equipment } from './equipment';
import { RadioEquipment } from './radio';
import { RadioRoles } from './radio-contract';
import { device, reading, link } from './radio-test-fixtures';
const now = new Date(Date.now() - 12_000).toISOString();
function fixtures() {
  const cpe = {
    ...device,
    id: 'synthetic-cpe',
    name: 'CPE sintética',
    role: RadioRoles.cpe,
    credential_id: 'bound',
  };
  const concentrator = {
    ...device,
    id: 'concentrator',
    name: 'Concentrador sintético',
    role: 'concentrator',
    credential_id: 'bound',
  };
  const devices = [cpe, concentrator];
  const row = {
    row_id: 'linked-row',
    login: 'X',
    address: '192.0.2.100',
    state: 'ONLINE',
    uptime: { known: false, min: 0, max: 0 },
  };
  const result = (data = {}) => ({
    code: 'OK',
    profile_id: 'company',
    activation_id: 'active',
    data: {
      devices,
      credentials: [],
      next_cursor: 0,
      vault_available: true,
      total: 1,
      ...data,
    },
  });
  const bindings = {
    ListEquipment: vi.fn(async () => result()),
    ListCredentials: vi.fn(async () => result()),
    GetEquipment: vi.fn(async (...args: [string, string, string]) =>
      result({ device: devices.find((d) => d.id === args[2]) }),
    ),
    GetRadio: vi.fn(async () =>
      result({ reading: { ...reading, observed_at: now }, link }),
    ),
    GetPPPoECurrent: vi.fn(async () =>
      result({
        current: {
          observation_id: 'latest',
          count: 1,
          provenance: { ended_at: now },
          quality: { status: 'complete' },
        },
      }),
    ),
    ListPPPoESessions: vi.fn(async () => result({ rows: [row] })),
    ListPPPoEReferences: vi.fn(async () => result({ references: [] })),
    GetPPPoESession: vi.fn(async () => result({ rows: [row] })),
    PPPoEContextCounts: vi.fn(async () => result({ groups: [] })),
    GetDeviceMonitoring: vi.fn(async () => result()),
    GetRadioLink: vi.fn(async () => result()),
    StartPPPoERead: vi.fn(async () => ({ ...result(), code: 'BUSY' })),
    CancelRadioOperation: vi.fn(async () => result()),
  };
  vi.stubGlobal('go', { presentation: { App: bindings } });
  return { cpe, bindings, result };
}
afterEach(() => {
  cleanup();
  vi.stubGlobal('go', undefined);
});
it('Cliente da CPE lê o login na última observação e abre a tela da sessão vinculada', async () => {
  const { cpe, bindings } = fixtures();
  render(
    <Equipment
      profile="company"
      activation="active"
      connected

      targetID={cpe.id}
    />,
  );
  fireEvent.click(await screen.findByRole('tab', { name: 'Cliente' }));
  const user = await screen.findByRole('link', { name: 'X' });
  const block = screen
    .getByText('Sessão PPPoE vinculada')
    .closest('[data-state]')!;
  expect(block).toHaveAttribute('data-state', 'ready');
  expect(block).toHaveTextContent('192.0.2.100');
  expect(bindings.ListPPPoESessions).toHaveBeenCalledWith(
    'company',
    'active',
    'concentrator',
    'latest',
    expect.objectContaining({ search: 'X' }),
    '',
  );
  fireEvent.click(user);
  const panel = await screen.findByRole('region', {
    name: 'Detalhes da sessão',
  });
  expect(panel).toHaveTextContent('X');
  fireEvent.click(within(panel).getByRole('button', { name: 'Voltar' }));
  expect(await screen.findByRole('heading', { name: 'Concentrador sintético' })).toBeVisible();
  expect(screen.getByRole('tab', { name: 'Sessões' })).toHaveAttribute('aria-selected', 'true');
  expect(bindings.GetPPPoESession).toHaveBeenCalledWith(
    'company',
    'active',
    'concentrator',
    'latest',
    'linked-row',
  );
});
it('Cliente sem vínculo é Vazio e não busca sessões por aproximação de nome', async () => {
  const { cpe, bindings, result } = fixtures();
  bindings.GetRadio.mockResolvedValue(
    result({ reading: { ...reading, observed_at: now } }),
  );
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={cpe}
      blocked={false}
      tab="Cliente"
    />,
  );
  expect(await screen.findByText('Sem vínculo')).toBeVisible();
  expect(
    screen.getByText('Sessão PPPoE vinculada').closest('[data-state]'),
  ).toHaveAttribute('data-state', 'empty');
  expect(bindings.ListPPPoESessions).not.toHaveBeenCalled();
});
it('cliente AP confirmado abre ficha pelo nome, linha e teclado; sem cpe_id não navega', async () => {
  const { result, bindings } = fixtures();
  const clients = [
    { ...reading.clients[0], cpe_id: 'verified-cpe' },
    {
      ...reading.clients[0],
      mac: '02:00:00:00:00:99',
      name: 'Nome observado sem vínculo',
      metrics: [],
    },
  ];
  bindings.GetRadio.mockResolvedValue(
    result({ reading: { ...reading, clients } }),
  );
  const open = vi.fn();
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={device}
      blocked={false}
      tab="Clientes"
      devices={[{ ...device, id: 'verified-cpe', name: 'CPE cadastrada' }]}
      openEquipment={open}
    />,
  );
  const linked = await screen.findByRole('link', { name: 'CPE cadastrada' });
  fireEvent.click(linked);
  expect(open).toHaveBeenCalledOnce();
  fireEvent.click(linked.closest('tr')!);
  fireEvent.keyDown(linked.closest('tr')!, { key: 'Enter' });
  expect(open).toHaveBeenCalledTimes(3);
  expect(open).toHaveBeenLastCalledWith('verified-cpe');
  const unbound = screen
    .getByRole('cell', { name: 'Nome observado sem vínculo' })
    .closest('tr')!;
  expect(within(unbound).queryByRole('link')).not.toBeInTheDocument();
  fireEvent.click(unbound);
  await waitFor(() => expect(open).toHaveBeenCalledTimes(3));
});

it('o login exato da CPE é localizado mesmo depois de outros resultados por prefixo', async () => {
  const { cpe, bindings, result } = fixtures();
  bindings.ListPPPoESessions.mockResolvedValueOnce(
    result({
      rows: [
        {
          row_id: 'prefix',
          login: 'X-prefix',
          state: 'ONLINE',
          address: '192.0.2.99',
          uptime: { known: false, min: 0, max: 0 },
        },
      ],
      next_cursor: 'later',
    }),
  );
  render(
    <RadioEquipment
      profile="company"
      activation="active"
      device={cpe}
      blocked={false}
      tab="Cliente"
    />,
  );
  expect(await screen.findByRole('link', { name: 'X' })).toBeVisible();
  expect(bindings.ListPPPoESessions).toHaveBeenLastCalledWith(
    'company',
    'active',
    'concentrator',
    'latest',
    expect.objectContaining({ search: 'X' }),
    'later',
  );
});
