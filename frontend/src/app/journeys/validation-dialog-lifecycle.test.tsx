import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from '../pages/equipment';
import { equipmentAPI as api } from '../../features/equipment/equipment-api';
import type { EquipmentResult } from '../../features/equipment/equipment-api';
vi.mock('../../platform/wails/presentation', () => ({
  equipmentAPI: Object.fromEntries(
    [
      'list',
      'credentials',
      'save',
      'enrollmentStatus',
      'putCredential',
      'get',
      'active',
      'inspect',
      'confirm',
      'start',
      'status',
      'cancel',
    ].map((k) => [k, vi.fn()]),
  ),
}));
const result = (code = 'OK'): EquipmentResult => ({
  code,
  profile_id: 'modal',
  activation_id: 'a',
  data: { devices: [], credentials: [], next_cursor: 0, vault_available: true },
});
beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.list).mockResolvedValue(result());
  vi.mocked(api.credentials).mockResolvedValue(result());
});

it('requires discard before switching a dirty credential and cancel preserves the draft', async () => {
  const credential = {
    id: 'shared',
    alias: 'Shared',
    user: 'reader',
    links: 2,
    revision: 1,
  };
  vi.mocked(api.list).mockResolvedValue({
    ...result(),
    data: { ...result().data!, total_devices: 1 },
  });
  vi.mocked(api.credentials).mockResolvedValue({
    ...result(),
    data: { ...result().data!, credentials: [credential] },
  });
  render(
    <Equipment
      credentialsPage
      profile="credential-switch"
      activation="a"
      connected
    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Adicionar credencial' }),
  );
  fireEvent.change(screen.getByLabelText('Senha remota'), {
    target: { value: 'synthetic' },
  });
  fireEvent.change(screen.getByLabelText('Credencial para editar'), {
    target: { value: 'shared' },
  });
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Continuar editando' }),
  );
  expect(screen.getByLabelText('Senha remota')).toHaveValue('synthetic');
  expect(screen.getByLabelText('Credencial para editar')).toHaveValue('');
  fireEvent.change(screen.getByLabelText('Credencial para editar'), {
    target: { value: 'shared' },
  });
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Descartar alterações' }),
  );
  expect(screen.getByLabelText('Senha remota')).toHaveValue('');
  expect(screen.getByLabelText('Alias')).toHaveValue('Shared');
  expect(
    screen.getByRole('button', { name: 'Salvar credencial' }),
  ).toBeEnabled();
  expect(api.putCredential).not.toHaveBeenCalled();
});
