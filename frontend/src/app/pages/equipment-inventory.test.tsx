import { device, data } from '../../shared/testing/equipment-test-fixture';

import { beginAdd, openDeviceSheet } from '../test-navigation';
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from '../test-equipment-shell';

import { equipmentAPI as api } from '../../features/equipment/equipment-api';

import type {
  EquipmentResult,
  Device,
} from '../../features/equipment/equipment-api';
vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({
    deviceMonitoringAPI: {
      current: vi.fn().mockResolvedValue({
        code: 'OK',
        profile_id: 'company',
        activation_id: 'active',
        data: { total: 0 },
      }),
    },
  }),
);
beforeEach(resetEquipmentMocks);
import { resetEquipmentMocks } from '../../shared/testing/equipment-setup';
for (const failure of ['SERVICE_UNAVAILABLE', 'reject']) {
  it(`retries an initial inventory ${failure} locally without claiming empty`, async () => {
    if (failure === 'reject')
      vi.mocked(api.list).mockRejectedValueOnce(new Error('transport'));
    else
      vi.mocked(api.list).mockResolvedValueOnce({ ...data(), code: failure });
    render(<Equipment profile="X" activation="a" connected />);
    await screen.findByRole('alert');
    expect(
      screen.queryByRole('heading', { name: 'Nenhum equipamento cadastrado' }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Tentar novamente' }));
    expect(await openDeviceSheet('Router X')).toBeVisible();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(api.list).toHaveBeenCalledTimes(2);
    expect(api.start).not.toHaveBeenCalled();
    expect(api.inspect).not.toHaveBeenCalled();
  });
}
it('inactive filtering stays explicit; no remote work happens while listing or editing', async () => {
  render(<Equipment profile="X" activation="a" connected />);
  await openDeviceSheet('Router X');
  expect(api.start).not.toHaveBeenCalled();
  expect(api.inspect).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  fireEvent.click(screen.getByRole('checkbox', { name: 'Mostrar inativos' }));
  await waitFor(() =>
    expect(api.list).toHaveBeenLastCalledWith('X', 'a', '', 0, true, {
      role: '',
      block: '',
      block_empty: false,
      model: '',
    }),
  );
  await openDeviceSheet('Router X');
  fireEvent.click(screen.getByRole('button', { name: 'Editar equipamento' }));
  expect(api.start).not.toHaveBeenCalled();
});
it('Q03 late X inventory and credential responses cannot populate the remounted Y surface', async () => {
  let resolve!: (r: EquipmentResult) => void;
  let credentialResolve!: (r: EquipmentResult) => void;
  vi.mocked(api.credentials)
    .mockImplementationOnce(
      () =>
        new Promise((r) => {
          credentialResolve = r;
        }),
    )
    .mockResolvedValue({
      ...data(),
      data: {
        ...data().data!,
        credentials: [
          {
            id: 'fresh-y',
            alias: 'Fresh Y',
            user: 'reader-y',
            revision: 1,
            links: 0,
          },
        ],
      },
    });
  vi.mocked(api.list)
    .mockImplementationOnce(() => new Promise((r) => (resolve = r)))
    .mockResolvedValue(data([]));
  const view = render(
    <Equipment key="X" profile="X" activation="a" connected />,
  );
  view.rerender(<Equipment key="Y" profile="Y" activation="b" connected />);
  await screen.findByText('Nenhum equipamento cadastrado');
  await act(async () => {
    resolve(data());
    credentialResolve({
      ...data(),
      data: {
        ...data().data!,
        credentials: [
          {
            id: 'old-x',
            alias: 'Late X',
            user: 'reader-x',
            revision: 1,
            links: 0,
          },
        ],
      },
    });
  });
  expect(screen.queryByText('Router X')).not.toBeInTheDocument();
  expect(api.credentials).toHaveBeenCalledTimes(2);
  expect(api.credentials).toHaveBeenCalledWith('X', 'a', 0);
  expect(api.credentials).toHaveBeenCalledWith('Y', 'b', 0);
  await beginAdd();
  expect(screen.getByRole('option', { name: 'Fresh Y' })).toBeInTheDocument();
  expect(screen.queryByText(/Late X/)).not.toBeInTheDocument();
});
it('full detail interfaces survive a subsequent list projection and a new context discards late detail', async () => {
  const observation = {
    identity: 'Fixture router',
    version: '7.26',
    family: 'v7',
    board: 'Board',
    uptime: '1h',
    cpu_percent: 12,
    free_memory_bytes: 100,
    total_memory_bytes: 200,
    interfaces: [
      { name: 'ether1', kind: 'ether', running: true, disabled: false },
    ],
    observed_at: '2026-10-24T14:00:00Z',
    source: 'SSH fixture',
    quality: 'complete',
  };
  vi.mocked(api.list).mockResolvedValue(
    data([{ ...device, result: { ...observation, interfaces: [] } }]),
  );
  vi.mocked(api.get).mockResolvedValue({
    ...data(),
    data: { ...data().data!, device: { ...device, result: observation } },
  });
  const view = render(
    <Equipment key="X" profile="X" activation="a" connected />,
  );
  await openDeviceSheet('Router X');
  fireEvent.click(screen.getByRole('tab', { name: 'Resumo' }));
  fireEvent.click(
    await screen.findByRole('tab', { name: 'Interfaces e saúde' }),
  );
  await screen.findByText('ether1');
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  fireEvent.change(screen.getByLabelText('Buscar'), {
    target: { value: 'Router' },
  });
  await waitFor(() =>
    expect(api.list).toHaveBeenLastCalledWith('X', 'a', 'Router', 0, false),
  );
  await openDeviceSheet('Router X');
  fireEvent.click(screen.getByRole('tab', { name: 'Interfaces e saúde' }));
  expect(await screen.findByText('ether1')).toBeInTheDocument();
  let resolve!: (r: EquipmentResult) => void;
  vi.mocked(api.get).mockImplementationOnce(
    () => new Promise((r) => (resolve = r)),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  fireEvent.click(screen.getByRole('cell', { name: 'Router X' }));
  vi.mocked(api.list).mockResolvedValue(data([]));
  view.rerender(<Equipment key="Y" profile="Y" activation="b" connected />);
  await screen.findByText('Nenhum equipamento cadastrado');
  await act(async () =>
    resolve({
      ...data(),
      data: { ...data().data!, device: { ...device, result: observation } },
    }),
  );
  expect(screen.queryByText('ether1')).not.toBeInTheDocument();
});
it('late detail cannot restore an old selected device', async () => {
  const other = { ...device, id: 'other', name: 'Router B' };
  vi.mocked(api.list).mockResolvedValue(
    data([
      { ...device, result: { observed_at: '2026-10-24' } as Device['result'] },
      other,
    ]),
  );
  let resolve!: (r: EquipmentResult) => void;
  vi.mocked(api.get).mockImplementationOnce(
    () => new Promise((r) => (resolve = r)),
  );
  render(<Equipment profile="detail" activation="surface" connected />);
  await openDeviceSheet('Router X');
  fireEvent.click(screen.getByRole('tab', { name: 'Resumo' }));
  if (screen.queryByRole('button', { name: 'Voltar' }))
    fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  fireEvent.click(screen.getByRole('row', { name: /Router B/ }));
  await act(async () =>
    resolve({ ...data(), data: { ...data().data!, device } }),
  );
  expect(screen.getByRole('heading', { name: 'Router B' })).toBeInTheDocument();
});
it('inactive filtering and an empty search recover through the frozen list and explicit sheet', async () => {
  vi.mocked(api.list).mockImplementation(
    async (_p, _a, search, _cursor, inactive) => ({
      ...data(search || !inactive ? [] : [{ ...device, active: false }]),
      data: {
        ...data(search || !inactive ? [] : [{ ...device, active: false }])
          .data!,
        total_devices: 1,
      },
    }),
  );
  render(<Equipment profile="inactive-only" activation="a" connected />);
  await screen.findByText('Nenhum equipamento cadastrado');
  expect(screen.getByLabelText('Estado')).toBeEnabled();
  const options = within(screen.getByLabelText('Estado')).getAllByRole(
    'option',
  );
  expect(options.map((o) => o.textContent)).toEqual([
    'Todos',
    'ONLINE',
    'OFFLINE',
    'Sem estado',
  ]);
  expect(screen.getByRole('table')).toBeVisible();
  expect(screen.getAllByRole('button', { name: /Adicionar/ })).toHaveLength(1);
  fireEvent.click(screen.getByRole('checkbox', { name: 'Mostrar inativos' }));
  await openDeviceSheet('Router X');
  expect(screen.getByRole('button', { name: 'Reativar' })).toBeEnabled();
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  fireEvent.change(screen.getByLabelText('Buscar'), {
    target: { value: 'absent' },
  });
  await screen.findByText('Nenhum equipamento encontrado');
  expect(screen.getByRole('table')).toBeVisible();
  expect(screen.queryByText('Detalhes do equipamento')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Limpar filtros' }));
  await screen.findByText('Nenhum equipamento cadastrado');
  expect(screen.getByLabelText('Buscar')).toHaveValue('');
  expect(
    screen.getByRole('checkbox', { name: 'Mostrar inativos' }),
  ).not.toBeChecked();
  fireEvent.click(screen.getByRole('checkbox', { name: 'Mostrar inativos' }));
  await openDeviceSheet('Router X');
  expect(screen.getByRole('button', { name: 'Reativar' })).toBeEnabled();
});
it('Q03 inventory filters and refresh do not reload an independent credential catalog', async () => {
  render(<Equipment profile="Q03" activation="q03" connected />);
  await openDeviceSheet('Router X');
  await waitFor(() => expect(api.credentials).toHaveBeenCalledTimes(1));
  fireEvent.click(screen.getByRole('button', { name: 'Voltar' }));
  const search = screen.getByPlaceholderText(/Buscar/);
  fireEvent.change(search, { target: { value: 'Router' } });
  await waitFor(() => expect(api.list).toHaveBeenCalledTimes(2));
  expect(api.credentials).toHaveBeenCalledTimes(1);
});
