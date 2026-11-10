import { render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from '../../app/pages/equipment';
import { equipmentAPI as api } from '../equipment/equipment-api';
import type { Device, EquipmentResult } from '../equipment/equipment-api';

vi.mock('../../platform/wails/presentation', () => ({
  equipmentAPI: { list: vi.fn(), credentials: vi.fn() },
}));
const device = (id: string, name: string, credential: string, active = true) =>
  ({
    id,
    name,
    credential_id: credential,
    active,
    role: 'other',
    revision: 1,
  }) as Device;
const result = (
  devices: Device[],
  total: number,
  next = 0,
): EquipmentResult => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'a',
  data: {
    devices,
    credentials: [],
    next_cursor: next,
    total_devices: total,
    vault_available: true,
  },
});
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.credentials).mockResolvedValue({
    ...result([], 0),
    data: {
      ...result([], 0).data!,
      credentials: [
        {
          id: 'cred',
          alias: 'Credencial A',
          user: 'leitor',
          revision: 1,
          links: 3,
        },
        {
          id: 'empty',
          alias: 'Credencial vazia',
          user: 'leitor',
          revision: 1,
          links: 0,
        },
      ],
    },
  });
});
const open = () =>
  render(
    <Equipment
      profile="company"
      activation="a"
      connected

      credentialsPage
    />,
  );
const cell = (name: string) =>
  within(screen.getByRole('row', { name: new RegExp(name) })).getAllByRole(
    'cell',
  )[2];
it('lists sorted names across active and inactive pages, and an empty credential', async () => {
  const z = device('z', 'Zeta', 'cred');
  const a = device('a', 'Alfa', 'cred');
  const inactive = device('i', 'Beta', 'cred', false);
  const unbound = device('u', 'Sem credencial', '');
  vi.mocked(api.list).mockImplementation(
    async (_p, _a, _search, cursor, inactiveOnly) => {
      if (inactiveOnly) return result([inactive], 4);
      return cursor === 0 ? result([z, unbound], 4, 2) : result([a], 4);
    },
  );
  open();
  await waitFor(() =>
    expect(cell('Credencial A')).toHaveTextContent('Alfa, Beta, Zeta'),
  );
  expect(cell('Credencial A').querySelector('[title]')).toHaveAttribute(
    'title',
    'Alfa, Beta, Zeta',
  );
  expect(cell('Credencial vazia')).toHaveTextContent('—');
  expect(api.list).toHaveBeenCalledWith('company', 'a', '', 2, false);
  expect(api.list).toHaveBeenCalledWith('company', 'a', '', 0, true);
});
it('shows the single linked name', async () => {
  vi.mocked(api.list).mockImplementation(async (_p, _a, _s, _c, inactive) =>
    result(inactive ? [] : [device('one', 'Único', 'cred')], 1),
  );
  open();
  await waitFor(() => expect(cell('Credencial A')).toHaveTextContent('Único'));
});
it('reports a failed page without presenting partial names', async () => {
  vi.mocked(api.list).mockImplementation(async (_p, _a, _s, _c, inactive) =>
    inactive
      ? { ...result([], 1), code: 'SERVICE_UNAVAILABLE' }
      : result([device('one', 'Único', 'cred')], 1),
  );
  open();
  await waitFor(() =>
    expect(api.list).toHaveBeenCalledWith('company', 'a', '', 0, true),
  );
  expect(cell('Credencial A')).toHaveTextContent('Não informado nesta leitura');
});

it('does not present incomplete inventory as a complete usage list', async () => {
  vi.mocked(api.list).mockImplementation(async (_p, _a, _s, _c, inactive) =>
    result(inactive ? [] : [device('one', 'Único', 'cred')], 2),
  );
  open();
  await waitFor(() =>
    expect(api.list).toHaveBeenCalledWith('company', 'a', '', 0, true),
  );
  expect(cell('Credencial A')).toHaveTextContent('Não informado nesta leitura');
});
