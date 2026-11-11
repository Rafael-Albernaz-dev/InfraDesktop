import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from '../pages/equipment';
import { equipmentAPI } from '../../features/equipment/equipment-api';
import type {
  Device,
  EquipmentResult,
} from '../../features/equipment/equipment-api';
import { RadioRoles } from '../../features/equipment/radio-contract';

const prefs = vi.hoisted(() => ({ saved: [] as string[] }));
vi.mock('../../platform/wails/presentation', async () =>
  (await import('../../shared/testing/presentation-mock')).presentationMock({
    access: {
      status: vi.fn(async () => ({ code: 'OK', columns: prefs.saved })),
      setColumns: vi.fn(async (columns: string[]) => {
        prefs.saved = columns;
        return { code: 'OK', columns };
      }),
    },
  }),
);

const rows = [
  {
    id: 'x1',
    name: 'Synthetic AP',
    role: RadioRoles.ap,
    block: 'Norte',
    address: '192.0.2.1',
    revision: 1,
    active: true,
  },
  {
    id: 'x2',
    name: 'Synthetic Switch',
    role: 'switch',
    block: 'Sul',
    address: '192.0.2.2',
    revision: 1,
    active: true,
  },
] as unknown as Device[];
const result = (profile: string, devices: Device[]) =>
  ({
    code: 'OK',
    profile_id: profile,
    activation_id: 'a',
    data: {
      devices,
      credentials: [],
      blocks: ['Norte', 'Sul'],
      models: [],
      next_cursor: 0,
      vault_available: true,
      total_devices: devices.length,
    },
  }) as EquipmentResult;
beforeEach(() => {
  prefs.saved = [];
  vi.clearAllMocks();
  vi.mocked(equipmentAPI.list).mockImplementation(
    async (profile, _activation, search, _cursor, _inactive, filters) => {
      const found =
        profile === 'X'
          ? rows
          : [{ ...rows[1], id: 'y1', name: 'Other company' }];
      return result(
        profile,
        found.filter(
          (d) =>
            d.name.includes(search) &&
            (!filters?.role || d.role === filters.role),
        ),
      );
    },
  );
  vi.mocked(equipmentAPI.credentials).mockResolvedValue(result('X', []));
});

it('does not open the sheet from a checkbox and clears selection on filter and company changes', async () => {
  const view = render(<Equipment profile="X" activation="a" connected />);
  fireEvent.click(
    await screen.findByRole('checkbox', { name: 'Selecionar Synthetic AP' }),
  );
  expect(screen.getByText('1 selecionado')).toBeVisible();
  expect(
    screen.queryByRole('region', { name: 'Ficha do equipamento' }),
  ).not.toBeInTheDocument();
  fireEvent.change(screen.getByRole('combobox', { name: 'Papel' }), {
    target: { value: 'switch' },
  });
  await screen.findByRole('checkbox', { name: 'Selecionar Synthetic Switch' });
  expect(screen.queryByText('1 selecionado')).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('checkbox', { name: 'Selecionar Synthetic Switch' }),
  );
  expect(screen.getByText('1 selecionado')).toBeVisible();
  view.rerender(<Equipment profile="Y" activation="b" connected />);
  await screen.findByRole('checkbox', { name: 'Selecionar Other company' });
  expect(screen.queryByText('1 selecionado')).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole('checkbox', { name: 'Selecionar Other company' }),
  );
  view.rerender(<Equipment profile="X" activation="a" connected />);
  await screen.findByRole('checkbox', { name: 'Selecionar Synthetic Switch' });
  expect(screen.queryByText('1 selecionado')).not.toBeInTheDocument();
});

it('persists a hidden column after remount and closes the menu only on escape or outside click', async () => {
  const first = render(<Equipment profile="X" activation="a" connected />);
  await screen.findByRole('checkbox', { name: 'Selecionar Synthetic AP' });
  fireEvent.click(screen.getByRole('button', { name: 'Colunas' }));
  const menu = screen.getByRole('menu', { name: 'Colunas' });
  fireEvent.click(within(menu).getByRole('checkbox', { name: 'CPU' }));
  await waitFor(() =>
    expect(
      screen.queryByRole('columnheader', { name: 'CPU' }),
    ).not.toBeInTheDocument(),
  );
  expect(screen.getByRole('menu', { name: 'Colunas' })).toBeVisible();
  fireEvent.keyDown(screen.getByRole('menu', { name: 'Colunas' }), {
    key: 'Escape',
  });
  expect(
    screen.queryByRole('menu', { name: 'Colunas' }),
  ).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Colunas' }));
  fireEvent.mouseDown(first.baseElement);
  expect(
    screen.queryByRole('menu', { name: 'Colunas' }),
  ).not.toBeInTheDocument();
  first.unmount();
  render(<Equipment profile="X" activation="a" connected />);
  await waitFor(() =>
    expect(
      screen.queryByRole('columnheader', { name: 'CPU' }),
    ).not.toBeInTheDocument(),
  );
  expect(prefs.saved).not.toContain('cpu');
});

it('submits selected device revisions to the bulk service without opening the sheet', async () => {
  vi.mocked(equipmentAPI.bulk).mockResolvedValue(result('X', []));
  render(<Equipment profile="X" activation="a" connected />);
  fireEvent.click(
    await screen.findByRole('checkbox', { name: 'Selecionar Synthetic AP' }),
  );
  fireEvent.click(
    screen.getByRole('checkbox', { name: 'Selecionar Synthetic Switch' }),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Definir bloco' }));
  const dialog = screen.getByRole('dialog', { name: 'Definir bloco' });
  fireEvent.change(within(dialog).getByLabelText('Bloco'), {
    target: { value: 'Novo bloco' },
  });
  fireEvent.click(within(dialog).getByRole('button', { name: 'Aplicar' }));
  await waitFor(() =>
    expect(equipmentAPI.bulk).toHaveBeenCalledWith(
      'X',
      'a',
      expect.any(String),
      {
        items: [
          { id: 'x1', revision: 1 },
          { id: 'x2', revision: 1 },
        ],
        block: 'Novo bloco',
      },
    ),
  );
  expect(equipmentAPI.start).not.toHaveBeenCalled();
  expect(equipmentAPI.inspect).not.toHaveBeenCalled();
});

it('reorders visible columns and restores the default order', async () => {
  render(<Equipment profile="X" activation="a" connected />);
  await screen.findByRole('checkbox', { name: 'Selecionar Synthetic AP' });
  fireEvent.click(screen.getByRole('button', { name: 'Colunas' }));
  fireEvent.click(screen.getByRole('button', { name: 'Subir Papel' }));
  await waitFor(() =>
    expect(prefs.saved.slice(0, 3)).toEqual(['equipment', 'role', 'status']),
  );
  const headers = screen
    .getAllByRole('columnheader')
    .map((node) => node.textContent);
  expect(headers.indexOf('Papel')).toBeLessThan(headers.indexOf('Estado'));
  fireEvent.click(screen.getByRole('button', { name: 'Restaurar padrão' }));
  await waitFor(() =>
    expect(prefs.saved.slice(0, 3)).toEqual(['equipment', 'status', 'role']),
  );
});
