import {
  act,
  fireEvent,
  render,
  screen,
  within,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Equipment } from './equipment';
import { equipmentAPI as api } from './equipment-api';
import type { EquipmentResult } from './equipment-api';
vi.mock('./presentation', () => ({
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
it('protects credential drafts against backdrop and dirty close', async () => {
  vi.mocked(api.list).mockResolvedValue({
    ...result(),
    data: { ...result().data!, total_devices: 1 },
  });
  render(
    <Equipment
      credentialsPage
      profile="credential-dirty"
      activation="a"
      connected

    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Adicionar credencial' }),
  );
  fireEvent.change(screen.getByLabelText('Senha remota'), {
    target: { value: 'synthetic credential' },
  });
  const dialog = screen.getByRole('dialog');
  fireEvent.click(dialog.parentElement!);
  expect(dialog).toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
  fireEvent.click(
    within(
      screen.getByRole('dialog', { name: 'Descartar alterações?' }),
    ).getByRole('button', { name: 'Continuar editando' }),
  );
  expect(screen.getByLabelText('Senha remota')).toHaveValue(
    'synthetic credential',
  );
});
it('blocks all credential closing routes during a send and clears the request object', async () => {
  vi.mocked(api.list).mockResolvedValue({
    ...result(),
    data: { ...result().data!, total_devices: 1 },
  });
  let resolve!: (r: EquipmentResult) => void;
  let request: { secret: string } | undefined;
  vi.mocked(api.putCredential).mockImplementation((_p, _a, d) => {
    request = d;
    return new Promise((r) => {
      resolve = r;
    });
  });
  render(
    <Equipment
      credentialsPage
      profile="credential-send"
      activation="a"
      connected

    />,
  );
  fireEvent.click(
    await screen.findByRole('button', { name: 'Adicionar credencial' }),
  );
  for (const [label, value] of [
    ['Alias', 'Consulta'],
    ['Usuário remoto', 'reader'],
    ['Senha remota', 'synthetic'],
  ])
    fireEvent.change(screen.getByLabelText(label), { target: { value } });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar credencial' }));
  const dialog = screen.getByRole('dialog');
  fireEvent.click(screen.getByRole('button', { name: 'Fechar painel' }));
  fireEvent.keyDown(dialog, { key: 'Escape' });
  fireEvent.click(dialog.parentElement!);
  expect(dialog).toBeInTheDocument();
  expect(
    screen.queryByRole('dialog', { name: 'Descartar alterações?' }),
  ).not.toBeInTheDocument();
  await act(async () => resolve(result('INVALID_REQUEST')));
  expect(request?.secret).toBe('');
  expect(screen.getByLabelText('Senha remota')).toHaveValue('');
  expect(screen.getByRole('alert')).toHaveTextContent(/O serviço recusou/);
});
