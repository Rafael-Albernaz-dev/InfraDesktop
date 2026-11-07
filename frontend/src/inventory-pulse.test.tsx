import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from './equipment';
import { equipmentAPI } from './equipment-api';
import type { Device, EquipmentResult } from './equipment-api';

vi.mock('./presentation', async (original) => ({
  ...(await original<typeof import('./presentation')>()),
  access: {
    status: vi.fn(async () => ({ code: 'OK', columns: [] })),
    setColumns: vi.fn(),
  },
  equipmentAPI: {
    list: vi.fn(),
    credentials: vi.fn(),
    get: vi.fn(),
  },
}));
const device = {
  id: 'd',
  name: 'Synthetic pulse',
  address: '192.0.2.1',
  active: true,
  revision: 1,
} as Device;
function result(state = 'ONLINE', next = 0, network = 'ok'): EquipmentResult {
  const at = new Date().toISOString();
  return {
    code: 'OK',
    profile_id: 'p',
    activation_id: 'a',
    data: {
      devices: [
        {
          ...device,
          pulse: { state, since: at, checked_at: at, last_reply: at },
        },
      ],
      credentials: [],
      next_cursor: next,
      vault_available: false,
      pulse_network: network,
    },
  };
}
async function flush() {
  await act(async () => {
    await Promise.resolve();
  });
}
function open() {
  const view = render(<div />);
  vi.spyOn(
    view.container.ownerDocument,
    'visibilityState',
    'get',
  ).mockReturnValue('visible');
  view.rerender(<Equipment profile="p" activation="a" connected />);
}
beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date('2026-11-06T12:00:00Z'));
  vi.clearAllMocks();
  vi.mocked(equipmentAPI.list).mockResolvedValue(result());
  vi.mocked(equipmentAPI.credentials).mockResolvedValue(result());
});
afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});
it('renews every five seconds preserving filters, cursor, selection, row and scroll without flashing', async () => {
  vi.mocked(equipmentAPI.list).mockResolvedValue(result('ONLINE', 16));
  open();
  await flush();
  fireEvent.change(screen.getByLabelText('Estado'), {
    target: { value: 'ONLINE' },
  });
  await flush();
  fireEvent.click(screen.getByRole('button', { name: 'Próxima' }));
  await flush();
  fireEvent.click(
    screen.getByRole('checkbox', { name: 'Selecionar Synthetic pulse' }),
  );
  const row = screen.getByText(device.name).closest('tr');
  const scroll = screen.getByRole('table').parentElement!;
  scroll.scrollLeft = 120;
  const before = vi.mocked(equipmentAPI.list).mock.calls.length;
  await act(async () => {
    await vi.advanceTimersByTimeAsync(4999);
  });
  expect(equipmentAPI.list).toHaveBeenCalledTimes(before);
  await act(async () => {
    await vi.advanceTimersByTimeAsync(1);
  });
  expect(equipmentAPI.list).toHaveBeenCalledTimes(before + 1);
  expect(equipmentAPI.list).toHaveBeenLastCalledWith(
    'p',
    'a',
    '',
    16,
    false,
    expect.objectContaining({ state: 'ONLINE' }),
  );
  expect(screen.getByLabelText('Estado')).toHaveValue('ONLINE');
  expect(
    screen.getByRole('checkbox', { name: 'Selecionar Synthetic pulse' }),
  ).toBeChecked();
  expect(screen.getByText(device.name).closest('tr')).toBe(row);
  expect(scroll.scrollLeft).toBe(120);
  expect(
    screen.queryByText('Carregando equipamentos…'),
  ).not.toBeInTheDocument();
});
it('discards an old response after changing the state filter', async () => {
  open();
  await flush();
  let resolveOld: (r: EquipmentResult) => void = () => {
    throw new Error('not pending');
  };
  vi.mocked(equipmentAPI.list).mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveOld = resolve;
      }),
  );
  await act(async () => {
    await vi.advanceTimersByTimeAsync(5000);
  });
  vi.mocked(equipmentAPI.list).mockResolvedValue(result('OFFLINE'));
  fireEvent.change(screen.getByLabelText('Estado'), {
    target: { value: 'OFFLINE' },
  });
  await flush();
  await act(async () => {
    resolveOld(result('ONLINE'));
  });
  const row = screen.getByText(device.name).closest('tr')!;
  expect(within(row).getByText('OFFLINE')).toHaveClass('st-off');
  expect(within(row).queryByText('ONLINE')).not.toBeInTheDocument();
});
it('pauses while hidden, ignores the pending response and resumes automatically', async () => {
  open();
  await flush();
  const doc = screen.getByLabelText('Estado').ownerDocument;
  let visibility: DocumentVisibilityState = 'visible';
  vi.spyOn(doc, 'visibilityState', 'get').mockImplementation(() => visibility);
  let resolveOld: (r: EquipmentResult) => void = () => {
    throw new Error('not pending');
  };
  vi.mocked(equipmentAPI.list).mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveOld = resolve;
      }),
  );
  await act(async () => {
    await vi.advanceTimersByTimeAsync(5000);
  });
  visibility = 'hidden';
  fireEvent(doc, new Event('visibilitychange'));
  const calls = vi.mocked(equipmentAPI.list).mock.calls.length;
  await act(async () => {
    resolveOld(result('OFFLINE'));
    await vi.advanceTimersByTimeAsync(15000);
  });
  expect(equipmentAPI.list).toHaveBeenCalledTimes(calls);
  expect(
    within(screen.getByText(device.name).closest('tr')!).getByText('ONLINE'),
  ).toBeVisible();
  visibility = 'visible';
  fireEvent(doc, new Event('visibilitychange'));
  await flush();
  expect(equipmentAPI.list).toHaveBeenCalledTimes(calls + 1);
});
it.each([
  [
    'no_response',
    'Nenhum equipamento respondeu. Verifique a rede deste computador.',
  ],
  [
    'probe_blocked',
    'Este computador não tem permissão para enviar ping. Estado indisponível.',
  ],
])(
  'shows the contractual notice for %s and an unknown state',
  async (network, message) => {
    vi.mocked(equipmentAPI.list).mockResolvedValue(result('', 0, network));
    open();
    await flush();
    expect(screen.getByText(message)).toBeVisible();
    const row = screen.getByText(device.name).closest('tr')!;
    expect(within(row).queryByText('ONLINE')).not.toBeInTheDocument();
    expect(within(row).queryByText('OFFLINE')).not.toBeInTheDocument();
    expect(within(row).getByTitle('Ping há 0 s')).toHaveTextContent('—');
  },
);
it('distinguishes an empty state filter from an empty inventory and recovers through clear filters', async () => {
  const empty = result('');
  empty.data!.devices = [];
  vi.mocked(equipmentAPI.list).mockImplementation(
    async (...args) => (args[5]?.state ? empty : result()),
  );
  open();
  await flush();
  fireEvent.change(screen.getByLabelText('Estado'), {
    target: { value: 'OFFLINE' },
  });
  await flush();
  expect(screen.getByText('Nenhum equipamento encontrado')).toBeVisible();
  expect(
    screen.queryByText('Nenhum equipamento cadastrado'),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Limpar filtros' }));
  await flush();
  expect(screen.getByLabelText('Estado')).toHaveValue('');
  expect(screen.getByText(device.name)).toBeVisible();
});
it('never lets a list response erase direct navigation to an inactive device outside the filtered list', async () => {
  const empty = result('');
  empty.data!.devices = [];
  const detail = result('');
  detail.data!.device = { ...device, active: false, role: 'other' };
  vi.mocked(equipmentAPI.list).mockResolvedValue(empty);
  vi.mocked(equipmentAPI.get).mockResolvedValue(detail);
  const view = render(<div />);
  vi.spyOn(
    view.container.ownerDocument,
    'visibilityState',
    'get',
  ).mockReturnValue('visible');
  view.rerender(
    <Equipment profile="p" activation="a" connected targetID={device.id} />,
  );
  await flush();
  expect(screen.getByRole('heading', { name: device.name })).toBeVisible();
  await act(async () => {
    await vi.advanceTimersByTimeAsync(5000);
  });
  expect(screen.getByRole('heading', { name: device.name })).toBeVisible();
});

it('uses the current clock for ping age on the five-second renewal', async () => {
  const response = result();
  const checkedAt = new Date(Date.now() - 4000).toISOString();
  response.data!.devices![0].pulse!.checked_at = checkedAt;
  vi.mocked(equipmentAPI.list).mockResolvedValue(response);
  open();
  await flush();
  expect(screen.getByTitle('Ping há 4 s')).toHaveTextContent('ONLINE');
  await act(async () => {
    await vi.advanceTimersByTimeAsync(5000);
  });
  expect(screen.getByTitle('Ping há 9 s')).toHaveTextContent('ONLINE');
});
