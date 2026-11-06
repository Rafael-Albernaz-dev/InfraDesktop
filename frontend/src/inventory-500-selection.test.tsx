import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { Equipment } from './equipment';
import { equipmentAPI } from './equipment-api';
import type { Device } from './equipment-api';

vi.mock('./presentation', async (original) => ({
  ...(await original<typeof import('./presentation')>()),
  access: {
    status: vi.fn(async () => ({ code: 'OK', columns: [] })),
    setColumns: vi.fn(),
  },
  equipmentAPI: Object.fromEntries(
    'list,credentials,bulk,get,save,active,putCredential,setDefault,inspect,confirm,start,status,cancel,enrollmentStatus'
      .split(',')
      .map((key) => [key, vi.fn()]),
  ),
}));
const devices = Array.from({ length: 500 }, (_, i) => ({
  id: `fixture-${i}`,
  name: `Sintetico ${String(i).padStart(3, '0')}`,
  role: 'other',
  address: `192.0.2.${(i % 250) + 1}`,
  revision: 1,
  active: true,
})) as Device[];
const result = (page: Device[], next = 0) => ({
  code: 'OK',
  profile_id: 'company',
  activation_id: 'active',
  data: {
    devices: page,
    credentials: [],
    next_cursor: next,
    vault_available: true,
    total_devices: 500,
  },
});
it('selects each page of 500 synthetic devices and submits their exact revisions in one bulk operation', async () => {
  vi.mocked(equipmentAPI.list).mockImplementation(
    async (_p, _a, _search, cursor) =>
      result(
        devices.slice(cursor, cursor + 16),
        cursor + 16 < 500 ? cursor + 16 : 0,
      ) as never,
  );
  vi.mocked(equipmentAPI.credentials).mockResolvedValue(result([]) as never);
  vi.mocked(equipmentAPI.bulk).mockResolvedValue(result([]) as never);
  render(
    <Equipment
      profile="company"
      activation="active"
      connected
      createRequest={0}
    />,
  );
  for (let cursor = 0; cursor < 500; cursor += 16) {
    await screen.findByText(`Sintetico ${String(cursor).padStart(3, '0')}`);
    fireEvent.click(screen.getByRole('checkbox', { name: 'Selecionar todos' }));
    if (cursor + 16 < 500)
      fireEvent.click(screen.getByRole('button', { name: 'Próxima' }));
  }
  expect(screen.getByText('500 selecionados')).toBeVisible();
  fireEvent.click(screen.getByRole('button', { name: 'Definir bloco' }));
  const dialog = screen.getByRole('dialog', { name: 'Definir bloco' });
  fireEvent.change(within(dialog).getByLabelText('Bloco'), {
    target: { value: 'Torre sintética' },
  });
  fireEvent.click(within(dialog).getByRole('button', { name: 'Aplicar' }));
  await waitFor(() => expect(equipmentAPI.bulk).toHaveBeenCalledOnce());
  const input = vi.mocked(equipmentAPI.bulk).mock.calls[0][3];
  expect(input.items).toEqual(
    devices.map(({ id, revision }) => ({ id, revision })),
  );
  expect(input.block).toBe('Torre sintética');
  expect(equipmentAPI.start).not.toHaveBeenCalled();
});
